package speaker

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
	"github.com/gopxl/beep"
)

const channelCount = 2

var (
	mu             sync.Mutex
	mixer          beep.Mixer
	current        *speakerState
	bufferDuration time.Duration
)

type speakerState struct {
	lib    *mago.Library
	ctx    *mago.Context
	device *mago.Device

	scratch [][2]float64
}

// Init initializes audio playback through speaker. It must be called before
// audio can be heard.
//
// The bufferSize argument is expressed in samples, matching beep/speaker.
func Init(sampleRate beep.SampleRate, bufferSize int) error {
	if sampleRate <= 0 {
		return errors.New("speaker: sample rate must be positive")
	}
	if bufferSize <= 0 {
		return errors.New("speaker: buffer size must be positive")
	}

	mu.Lock()
	if current != nil {
		mu.Unlock()
		return errors.New("speaker cannot be initialized more than once")
	}
	mu.Unlock()

	lib, err := mago.Open()
	if err != nil {
		return err
	}

	ctx, err := lib.NewContext()
	if err != nil {
		_ = lib.Close()
		return err
	}

	state := &speakerState{
		lib: lib,
		ctx: ctx,
	}

	config := mago.DefaultPlaybackDeviceConfig()
	config.Format = mago.FormatF32
	config.Channels = channelCount
	if sampleRate > beep.SampleRate(math.MaxUint32) {
		_ = ctx.Close()
		_ = lib.Close()
		return fmt.Errorf("speaker: sample rate must be %d or less", uint32(math.MaxUint32))
	}
	if bufferSize > math.MaxUint32 {
		_ = ctx.Close()
		_ = lib.Close()
		return fmt.Errorf("speaker: buffer size must be %d or less", uint32(math.MaxUint32))
	}
	config.SampleRate = uint32(sampleRate)
	config.PeriodSizeInFrames = uint32(bufferSize)
	config.DataCallback = state.onDeviceData

	device, err := ctx.NewPlaybackDevice(config)
	if err != nil {
		_ = ctx.Close()
		_ = lib.Close()
		return err
	}
	state.device = device

	if err := device.Start(); err != nil {
		_ = device.Close()
		_ = ctx.Close()
		_ = lib.Close()
		return err
	}

	mu.Lock()
	if current != nil {
		mu.Unlock()
		_ = state.close()
		return errors.New("speaker cannot be initialized more than once")
	}
	mixer = beep.Mixer{}
	current = state
	bufferDuration = sampleRate.D(bufferSize)
	mu.Unlock()
	return nil
}

// Close closes audio playback and releases the underlying mago resources.
func Close() {
	mu.Lock()
	state := current
	current = nil
	bufferDuration = 0
	mixer.Clear()
	mu.Unlock()

	if state != nil {
		_ = state.close()
	}
}

// Lock locks the speaker. While locked, speaker won't pull new data from the
// playing Streamers.
func Lock() {
	mu.Lock()
}

// Unlock unlocks the speaker.
func Unlock() {
	mu.Unlock()
}

// Play starts playing all provided Streamers through the speaker.
func Play(s ...beep.Streamer) {
	mu.Lock()
	mixer.Add(s...)
	mu.Unlock()
}

// PlayAndWait plays all provided Streamers through the speaker and waits until
// they have all finished playing.
func PlayAndWait(s ...beep.Streamer) {
	mu.Lock()
	var wg sync.WaitGroup
	wg.Add(len(s))
	wrapped := make([]beep.Streamer, 0, len(s))
	for _, streamer := range s {
		wrapped = append(wrapped, &drainSignalStreamer{
			Streamer: streamer,
			done:     wg.Done,
		})
	}
	mixer.Add(wrapped...)
	waitForDriver := bufferDuration
	mu.Unlock()

	wg.Wait()
	if waitForDriver > 0 {
		time.Sleep(waitForDriver)
	}
}

// Suspend suspends the entire audio playback.
func Suspend() error {
	state, err := snapshotState()
	if err != nil {
		return err
	}
	return state.device.Stop()
}

// Resume resumes audio playback after Suspend.
func Resume() error {
	state, err := snapshotState()
	if err != nil {
		return err
	}
	return state.device.Start()
}

// Clear removes all currently playing Streamers from the speaker.
func Clear() {
	mu.Lock()
	mixer.Clear()
	mu.Unlock()
}

func snapshotState() (*speakerState, error) {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		return nil, errors.New("speaker: not initialized")
	}
	return current, nil
}

func (s *speakerState) close() error {
	var firstErr error
	if s.device != nil {
		if err := s.device.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.ctx != nil {
		if err := s.ctx.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.lib != nil {
		if err := s.lib.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *speakerState) onDeviceData(device *mago.Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
	_ = device
	_ = input
	if output == nil {
		return
	}

	out := unsafe.Slice((*float32)(output), int(frameCount)*channelCount)

	mu.Lock()
	defer mu.Unlock()
	streamToFloat32(&mixer, &s.scratch, out)
}

func streamToFloat32(streamer beep.Streamer, scratch *[][2]float64, out []float32) {
	for i := range out {
		out[i] = 0
	}
	if streamer == nil || len(out) == 0 {
		return
	}

	frames := len(out) / channelCount
	if frames == 0 {
		return
	}
	if cap(*scratch) < frames {
		*scratch = make([][2]float64, frames)
	}
	samples := (*scratch)[:frames]
	n, _ := streamer.Stream(samples)
	for i := n; i < frames; i++ {
		samples[i] = [2]float64{}
	}
	for i := 0; i < frames; i++ {
		out[i*channelCount] = clampSample(samples[i][0])
		out[i*channelCount+1] = clampSample(samples[i][1])
	}
}

func clampSample(v float64) float32 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return float32(v)
}

type drainSignalStreamer struct {
	beep.Streamer

	once sync.Once
	done func()
}

func (s *drainSignalStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = s.Streamer.Stream(samples)
	if !ok {
		s.once.Do(s.done)
	}
	return n, ok
}
