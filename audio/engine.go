package audio

import (
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
)

// Open creates a playback engine and starts the underlying output device.
//
// If Config.LibraryPath is empty, the low-level runtime loader uses the normal
// platform-specific shared library discovery rules from the mago package.
func Open(config Config) (*Engine, error) {
	cfg := config
	defaults := DefaultConfig()
	if cfg.SampleRate == 0 {
		cfg.SampleRate = defaults.SampleRate
	}
	if cfg.Channels == 0 {
		cfg.Channels = defaults.Channels
	}
	if cfg.PeriodSizeInFrames == 0 {
		cfg.PeriodSizeInFrames = defaults.PeriodSizeInFrames
	}
	if cfg.PerformanceProfile == 0 {
		cfg.PerformanceProfile = defaults.PerformanceProfile
	}
	if cfg.ShareMode == 0 {
		cfg.ShareMode = defaults.ShareMode
	}
	if cfg.Backends == nil {
		cfg.Backends = defaults.Backends
	}
	if cfg.DeviceIndex == 0 && config.DeviceIndex == 0 && config.DeviceName == "" {
		cfg.DeviceIndex = defaults.DeviceIndex
	}

	openOptions := []mago.LibraryOption{}
	if cfg.LibraryPath != "" {
		openOptions = append(openOptions, mago.WithLibraryPath(cfg.LibraryPath))
	}

	lib, err := mago.Open(openOptions...)
	if err != nil {
		return nil, err
	}

	ctx, err := lib.NewContext(cfg.Backends...)
	if err != nil {
		_ = lib.Close()
		return nil, err
	}

	if cfg.DeviceName != "" || cfg.DeviceIndex < 0 {
		playback, _, err := ctx.Devices()
		if err != nil {
			_ = ctx.Close()
			_ = lib.Close()
			return nil, err
		}
		if cfg.DeviceName != "" {
			cfg.DeviceIndex = findDeviceIndex(playback, cfg.DeviceName)
			if cfg.DeviceIndex < 0 {
				_ = ctx.Close()
				_ = lib.Close()
				return nil, fmt.Errorf("audio: no playback device matched %q", cfg.DeviceName)
			}
		} else if cfg.DeviceIndex < 0 {
			cfg.DeviceIndex = findDeviceIndex(playback, "")
		}
	}

	engine := &Engine{
		lib:        lib,
		ctx:        ctx,
		sampleRate: cfg.SampleRate,
		channels:   cfg.Channels,
		streams:    make(map[*Stream]struct{}),
	}

	deviceConfig := mago.DefaultPlaybackDeviceConfig()
	deviceConfig.DeviceIndex = cfg.DeviceIndex
	deviceConfig.Format = mago.FormatF32
	deviceConfig.Channels = cfg.Channels
	deviceConfig.SampleRate = cfg.SampleRate
	deviceConfig.PeriodSizeInFrames = cfg.PeriodSizeInFrames
	deviceConfig.PerformanceProfile = cfg.PerformanceProfile
	deviceConfig.ShareMode = cfg.ShareMode
	deviceConfig.DataCallback = engine.onDeviceData

	device, err := ctx.NewPlaybackDevice(deviceConfig)
	if err != nil {
		_ = ctx.Close()
		_ = lib.Close()
		return nil, err
	}
	engine.device = device
	if err := engine.device.Start(); err != nil {
		_ = engine.device.Close()
		_ = ctx.Close()
		_ = lib.Close()
		return nil, err
	}
	return engine, nil
}

// Start starts the underlying device explicitly.
//
// Open already starts the device automatically, so most callers do not need to
// invoke this unless they previously stopped the engine.
func (e *Engine) Start() error {
	if e == nil {
		return fmt.Errorf("audio: nil engine")
	}
	return e.device.Start()
}

// Stop stops the underlying playback device.
func (e *Engine) Stop() error {
	if e == nil {
		return fmt.Errorf("audio: nil engine")
	}
	return e.device.Stop()
}

// Close stops playback, closes active streams, and releases the underlying
// low-level library resources.
func (e *Engine) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	streams := make([]*Stream, 0, len(e.streams))
	for stream := range e.streams {
		streams = append(streams, stream)
	}
	e.mu.Unlock()

	for _, stream := range streams {
		stream.Close()
	}

	var firstErr error
	if e.device != nil {
		if err := e.device.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if e.ctx != nil {
		if err := e.ctx.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if e.lib != nil {
		if err := e.lib.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Devices returns the playback and capture devices visible to the engine's
// selected backend set.
func (e *Engine) Devices() ([]DeviceInfo, []DeviceInfo, error) {
	if e == nil {
		return nil, nil, fmt.Errorf("audio: nil engine")
	}
	return e.ctx.Devices()
}

// Load decodes a WAV stream from r into an in-memory Clip.
func (e *Engine) Load(r io.Reader) (*Clip, error) {
	return decodeWAV(r)
}

// LoadReadSeeker rewinds rs to the beginning and decodes it into an in-memory Clip.
func (e *Engine) LoadReadSeeker(rs io.ReadSeeker) (*Clip, error) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return decodeWAV(rs)
}

// Play creates a new stream for clip and registers it with the engine mixer.
//
// The returned Stream can be independently paused, resumed, stopped, faded, or
// configured for reverse playback.
func (e *Engine) Play(clip *Clip, options PlayOptions) (*Stream, error) {
	if e == nil {
		return nil, fmt.Errorf("audio: nil engine")
	}
	if clip == nil {
		return nil, fmt.Errorf("audio: nil clip")
	}
	if options.Speed == 0 {
		options.Speed = 1
	}
	if options.Speed < 0 {
		return nil, fmt.Errorf("audio: speed must be positive")
	}

	clip.mu.RLock()
	frameCount := clip.frameCount
	clip.mu.RUnlock()
	if frameCount == 0 {
		return nil, fmt.Errorf("audio: clip has no samples")
	}

	stream := &Stream{
		engine:  e,
		clip:    clip,
		volume:  options.Volume,
		speed:   options.Speed,
		looping: options.Loop,
		reverse: options.Reverse,
		state:   streamPlaying,
	}
	if options.StartPaused {
		stream.state = streamPaused
	}
	stream.resetPositionLocked(frameCount)

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, fmt.Errorf("audio: engine is closed")
	}
	e.streams[stream] = struct{}{}
	e.mu.Unlock()

	return stream, nil
}

// Crossfade starts a new stream from to and fades out from over the given duration.
//
// The incoming stream is started automatically with volume ramped from zero to the
// requested target volume.
func (e *Engine) Crossfade(from *Stream, to *Clip, duration time.Duration, options PlayOptions) (*Stream, error) {
	if duration < 0 {
		return nil, fmt.Errorf("audio: duration must be non-negative")
	}
	if options.Speed == 0 {
		options.Speed = 1
	}
	targetVolume := options.Volume
	if targetVolume == 0 {
		targetVolume = 1
	}
	options.Volume = 0

	toStream, err := e.Play(to, options)
	if err != nil {
		return nil, err
	}
	toStream.FadeTo(targetVolume, duration)
	toStream.Start()
	if from != nil {
		from.FadeOutAndStop(duration)
	}
	return toStream, nil
}

func (e *Engine) onDeviceData(device *mago.Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
	_ = device
	_ = input
	if output == nil {
		return
	}

	out := unsafe.Slice((*float32)(output), int(frameCount*e.channels))
	for i := range out {
		out[i] = 0
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for stream := range e.streams {
		if !stream.mixInto(out, int(frameCount), int(e.channels), int(e.sampleRate)) {
			delete(e.streams, stream)
		}
	}
}

type noCopy struct{}

var _ sync.Locker = (*sync.Mutex)(nil)
var _ = noCopy{}
