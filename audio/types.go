package audio

import (
	"io"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/darkliquid/mago"
)

type Config struct {
	LibraryPath        string
	Backends           []mago.Backend
	DeviceIndex        int
	DeviceName         string
	SampleRate         uint32
	Channels           uint32
	PeriodSizeInFrames uint32
	PerformanceProfile mago.PerformanceProfile
	ShareMode          mago.ShareMode
}

// PlayOptions controls the initial playback state and modifiers applied when a
// clip is started.
type PlayOptions struct {
	StartPaused bool
	Loop        bool
	Volume      float64
	Speed       float64
	Reverse     bool
}

// Clip is an in-memory decoded audio asset.
//
// Clips are immutable from the caller's point of view except for Release, which
// frees the stored sample buffer when the clip is no longer needed.
type Clip struct {
	mu         syncRWMutex
	samples    []float32
	channels   int
	sampleRate int
	frameCount int
}

// Stream represents one active or paused playback instance of a Clip.
//
// Multiple streams can be created from the same clip and mixed together by a
// single Engine.
type Stream struct {
	engine *Engine
	clip   *Clip

	mu       syncMutex
	position float64
	volume   float64
	speed    float64
	looping  bool
	reverse  bool
	state    streamState
	fade     fadeState
}

// Engine owns the playback device and software mixer used by the audio package.
//
// An Engine can play multiple streams concurrently and exposes a higher-level API
// than the low-level mago package.
type Engine struct {
	lib        *mago.Library
	ctx        *mago.Context
	device     *mago.Device
	sampleRate uint32
	channels   uint32

	mu      syncMutex
	streams map[*Stream]struct{}
	closed  bool
}

// DeviceInfo is re-exported from the low-level package for convenience.
type DeviceInfo = mago.DeviceInfo

type streamState uint8

const (
	streamStopped streamState = iota
	streamPlaying
	streamPaused
	streamClosed
)

type fadeState struct {
	active       bool
	start        float64
	end          float64
	remaining    int
	total        int
	stopWhenDone bool
}

type syncMutex struct{ mu sync.Mutex }

type syncRWMutex struct{ mu sync.RWMutex }

func (m *syncMutex) Lock()      { m.mu.Lock() }
func (m *syncMutex) Unlock()    { m.mu.Unlock() }
func (m *syncRWMutex) Lock()    { m.mu.Lock() }
func (m *syncRWMutex) Unlock()  { m.mu.Unlock() }
func (m *syncRWMutex) RLock()   { m.mu.RLock() }
func (m *syncRWMutex) RUnlock() { m.mu.RUnlock() }

func DefaultConfig() Config {
	return Config{
		DeviceIndex:        -1,
		SampleRate:         48000,
		Channels:           2,
		PeriodSizeInFrames: 256,
		PerformanceProfile: mago.PerformanceProfileLowLatency,
		ShareMode:          mago.ShareModeShared,
		Backends:           defaultBackends(),
	}
}

// DefaultPlayOptions returns sane defaults for straightforward playback:
// full volume, normal speed, no looping, and forward playback.
func DefaultPlayOptions() PlayOptions {
	return PlayOptions{
		Volume: 1,
		Speed:  1,
	}
}

func defaultBackends() []mago.Backend {
	switch runtime.GOOS {
	case "windows":
		return []mago.Backend{mago.BackendWASAPI, mago.BackendDSound, mago.BackendWinMM}
	case "darwin":
		return []mago.Backend{mago.BackendCoreAudio}
	case "freebsd":
		return []mago.Backend{mago.BackendOSS, mago.BackendJACK}
	case "netbsd":
		return []mago.Backend{mago.BackendAudio4}
	default:
		return []mago.Backend{mago.BackendPulseAudio, mago.BackendALSA, mago.BackendJACK}
	}
}

func findDeviceIndex(devices []mago.DeviceInfo, name string) int {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		for i, device := range devices {
			if device.IsDefault {
				return i
			}
		}
		if len(devices) == 0 {
			return -1
		}
		return 0
	}
	for i, device := range devices {
		if strings.Contains(strings.ToLower(device.Name), needle) {
			return i
		}
	}
	return -1
}

func durationFromFrames(frames int, sampleRate int) time.Duration {
	if sampleRate <= 0 || frames <= 0 {
		return 0
	}
	seconds := float64(frames) / float64(sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var (
	_ io.Reader
	_ io.ReadSeeker
)
