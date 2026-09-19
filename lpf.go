//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// LowPassFilter1Config specifies configuration for a first-order low-pass filter.
type LowPassFilter1Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
}

// LowPassFilter2Config specifies configuration for a second-order low-pass filter.
type LowPassFilter2Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}

// LowPassFilterConfig specifies configuration for an Nth-order low-pass filter.
type LowPassFilterConfig struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

// Type aliases for convenience.
type (
	LPF1Config = LowPassFilter1Config
	LPF2Config = LowPassFilter2Config
	LPFConfig  = LowPassFilterConfig
	LPF1       = LowPassFilter1
	LPF2       = LowPassFilter2
	LPF        = LowPassFilter
)

// LowPassFilter1 wraps miniaudio's ma_lpf1 filter.
type LowPassFilter1 struct {
	handle *lpf1Handle
	lib    *Library
	config LowPassFilter1Config
}

// NewLowPassFilter1 creates and initializes a first-order low-pass filter.
func (lib *Library) NewLowPassFilter1(config LowPassFilter1Config) (*LowPassFilter1, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectLPF1)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate lpf1 failed")
	}
	handle := (*lpf1Handle)(raw)

	nativeConfig := lpf1ConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
	}

	res := lib.bindings.maLPF1Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_lpf1_init", res)
	}

	return &LowPassFilter1{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewLPF1 is an alias for NewLowPassFilter1.
func (lib *Library) NewLPF1(config LowPassFilter1Config) (*LowPassFilter1, error) {
	return lib.NewLowPassFilter1(config)
}

// Reinit reconfigures the first-order low-pass filter without reallocating.
func (f *LowPassFilter1) Reinit(config LowPassFilter1Config) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf1")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := lpf1ConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
	}
	if err := f.lib.resultError("ma_lpf1_reinit", f.lib.bindings.maLPF1Reinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// ClearCache clears the internal state/history buffer of the filter.
func (f *LowPassFilter1) ClearCache() error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf1")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	// miniaudio 0.11.25's ma_lpf1_clear_cache has a bug where it sets pLPF->a to 0
	// instead of clearing the history buffer pR1. Call ma_lpf1_reinit to restore a.
	if res := f.lib.bindings.maLPF1ClearCache(f.handle); res != Success {
		return f.lib.resultError("ma_lpf1_clear_cache", res)
	}
	return f.Reinit(f.config)
}

// ProcessPCMFrames processes audio frames through the filter.
// pFramesOut and pFramesIn can point to the same buffer for in-place processing.
func (f *LowPassFilter1) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf1")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_lpf1_process_pcm_frames", f.lib.bindings.maLPF1ProcessPCMFrames(f.handle, pFramesOut, pFramesIn, frameCount))
}

// Latency returns the filter's latency in frames.
func (f *LowPassFilter1) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maLPF1GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *LowPassFilter1) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maLPF1Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// LowPassFilter2 wraps miniaudio's ma_lpf2 filter.
type LowPassFilter2 struct {
	handle *lpf2Handle
	lib    *Library
}

// NewLowPassFilter2 creates and initializes a second-order low-pass filter.
func (lib *Library) NewLowPassFilter2(config LowPassFilter2Config) (*LowPassFilter2, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectLPF2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate lpf2 failed")
	}
	handle := (*lpf2Handle)(raw)

	nativeConfig := lpf2ConfigNative(config)

	res := lib.bindings.maLPF2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_lpf2_init", res)
	}

	return &LowPassFilter2{
		handle: handle,
		lib:    lib,
	}, nil
}

// NewLPF2 is an alias for NewLowPassFilter2.
func (lib *Library) NewLPF2(config LowPassFilter2Config) (*LowPassFilter2, error) {
	return lib.NewLowPassFilter2(config)
}

// Reinit reconfigures the second-order low-pass filter without reallocating.
func (f *LowPassFilter2) Reinit(config LowPassFilter2Config) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := lpf2ConfigNative(config)
	return f.lib.resultError("ma_lpf2_reinit", f.lib.bindings.maLPF2Reinit(&nativeConfig, f.handle))
}

// ClearCache clears the internal state/history buffer of the filter.
func (f *LowPassFilter2) ClearCache() error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_lpf2_clear_cache", f.lib.bindings.maLPF2ClearCache(f.handle))
}

// ProcessPCMFrames processes audio frames through the filter.
func (f *LowPassFilter2) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_lpf2_process_pcm_frames", f.lib.bindings.maLPF2ProcessPCMFrames(f.handle, pFramesOut, pFramesIn, frameCount))
}

// Latency returns the filter's latency in frames.
func (f *LowPassFilter2) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maLPF2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *LowPassFilter2) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maLPF2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// LowPassFilter wraps miniaudio's ma_lpf filter.
type LowPassFilter struct {
	handle *lpfHandle
	lib    *Library
	config LowPassFilterConfig
}

// NewLowPassFilter creates and initializes an Nth-order low-pass filter.
func (lib *Library) NewLowPassFilter(config LowPassFilterConfig) (*LowPassFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectLPF)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate lpf failed")
	}
	handle := (*lpfHandle)(raw)

	nativeConfig := lpfConfigNative(config)

	res := lib.bindings.maLPFInit(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_lpf_init", res)
	}

	return &LowPassFilter{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewLPF is an alias for NewLowPassFilter.
func (lib *Library) NewLPF(config LowPassFilterConfig) (*LowPassFilter, error) {
	return lib.NewLowPassFilter(config)
}

// Reinit reconfigures the Nth-order low-pass filter without reallocating.
func (f *LowPassFilter) Reinit(config LowPassFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := lpfConfigNative(config)
	if err := f.lib.resultError("ma_lpf_reinit", f.lib.bindings.maLPFReinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// ClearCache clears the internal state/history buffer of the filter.
func (f *LowPassFilter) ClearCache() error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	if res := f.lib.bindings.maLPFClearCache(f.handle); res != Success {
		return f.lib.resultError("ma_lpf_clear_cache", res)
	}
	if f.config.Order%2 != 0 {
		return f.Reinit(f.config)
	}
	return nil
}

// ProcessPCMFrames processes audio frames through the filter.
func (f *LowPassFilter) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil lpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_lpf_process_pcm_frames", f.lib.bindings.maLPFProcessPCMFrames(f.handle, pFramesOut, pFramesIn, frameCount))
}

// Latency returns the filter's latency in frames.
func (f *LowPassFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maLPFGetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *LowPassFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maLPFUninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}
