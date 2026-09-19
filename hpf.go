//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// HighPassFilter1Config specifies configuration for a first-order high-pass filter.
type HighPassFilter1Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
}

// HighPassFilter2Config specifies configuration for a second-order high-pass filter.
type HighPassFilter2Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}

// HighPassFilterConfig specifies configuration for an Nth-order high-pass filter.
type HighPassFilterConfig struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

// Type aliases for convenience.
type (
	HPF1Config = HighPassFilter1Config
	HPF2Config = HighPassFilter2Config
	HPFConfig  = HighPassFilterConfig
	HPF1       = HighPassFilter1
	HPF2       = HighPassFilter2
	HPF        = HighPassFilter
)

// HighPassFilter1 wraps miniaudio's ma_hpf1 filter.
type HighPassFilter1 struct {
	handle *hpf1Handle
	lib    *Library
}

// NewHighPassFilter1 creates and initializes a first-order high-pass filter.
func (lib *Library) NewHighPassFilter1(config HighPassFilter1Config) (*HighPassFilter1, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectHPF1)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate hpf1 failed")
	}
	handle := (*hpf1Handle)(raw)

	nativeConfig := hpf1ConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
	}

	res := lib.bindings.maHPF1Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_hpf1_init", res)
	}

	return &HighPassFilter1{
		handle: handle,
		lib:    lib,
	}, nil
}

// NewHPF1 is an alias for NewHighPassFilter1.
func (lib *Library) NewHPF1(config HighPassFilter1Config) (*HighPassFilter1, error) {
	return lib.NewHighPassFilter1(config)
}

// Reinit reconfigures the first-order high-pass filter without reallocating.
func (f *HighPassFilter1) Reinit(config HighPassFilter1Config) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hpf1")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := hpf1ConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
	}
	return f.lib.resultError("ma_hpf1_reinit", f.lib.bindings.maHPF1Reinit(&nativeConfig, f.handle))
}

// ProcessPCMFrames processes audio frames through the filter.
// pFramesOut and pFramesIn can point to the same buffer for in-place processing.
func (f *HighPassFilter1) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hpf1")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_hpf1_process_pcm_frames", f.lib.bindings.maHPF1ProcessPCMFrames(f.handle, pFramesOut, pFramesIn, frameCount))
}

// Latency returns the filter's latency in frames.
func (f *HighPassFilter1) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maHPF1GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *HighPassFilter1) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maHPF1Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// HighPassFilter2 wraps miniaudio's ma_hpf2 filter.
type HighPassFilter2 struct {
	handle *hpf2Handle
	lib    *Library
}

// NewHighPassFilter2 creates and initializes a second-order high-pass filter.
func (lib *Library) NewHighPassFilter2(config HighPassFilter2Config) (*HighPassFilter2, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectHPF2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate hpf2 failed")
	}
	handle := (*hpf2Handle)(raw)

	nativeConfig := hpf2ConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
		Q:               config.Q,
	}

	res := lib.bindings.maHPF2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_hpf2_init", res)
	}

	return &HighPassFilter2{
		handle: handle,
		lib:    lib,
	}, nil
}

// NewHPF2 is an alias for NewHighPassFilter2.
func (lib *Library) NewHPF2(config HighPassFilter2Config) (*HighPassFilter2, error) {
	return lib.NewHighPassFilter2(config)
}

// Reinit reconfigures the second-order high-pass filter without reallocating.
func (f *HighPassFilter2) Reinit(config HighPassFilter2Config) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := hpf2ConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
		Q:               config.Q,
	}
	return f.lib.resultError("ma_hpf2_reinit", f.lib.bindings.maHPF2Reinit(&nativeConfig, f.handle))
}

// ProcessPCMFrames processes audio frames through the filter.
func (f *HighPassFilter2) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_hpf2_process_pcm_frames", f.lib.bindings.maHPF2ProcessPCMFrames(f.handle, pFramesOut, pFramesIn, frameCount))
}

// Latency returns the filter's latency in frames.
func (f *HighPassFilter2) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maHPF2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *HighPassFilter2) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maHPF2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// HighPassFilter wraps miniaudio's ma_hpf filter.
type HighPassFilter struct {
	handle *hpfHandle
	lib    *Library
}

// NewHighPassFilter creates and initializes an Nth-order high-pass filter.
func (lib *Library) NewHighPassFilter(config HighPassFilterConfig) (*HighPassFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectHPF)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate hpf failed")
	}
	handle := (*hpfHandle)(raw)

	nativeConfig := hpfConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
		Order:           config.Order,
	}

	res := lib.bindings.maHPFInit(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_hpf_init", res)
	}

	return &HighPassFilter{
		handle: handle,
		lib:    lib,
	}, nil
}

// NewHPF is an alias for NewHighPassFilter.
func (lib *Library) NewHPF(config HighPassFilterConfig) (*HighPassFilter, error) {
	return lib.NewHighPassFilter(config)
}

// Reinit reconfigures the Nth-order high-pass filter without reallocating.
func (f *HighPassFilter) Reinit(config HighPassFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := hpfConfigNative{
		Format:          config.Format,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
		Order:           config.Order,
	}
	return f.lib.resultError("ma_hpf_reinit", f.lib.bindings.maHPFReinit(&nativeConfig, f.handle))
}

// ProcessPCMFrames processes audio frames through the filter.
func (f *HighPassFilter) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	return f.lib.resultError("ma_hpf_process_pcm_frames", f.lib.bindings.maHPFProcessPCMFrames(f.handle, pFramesOut, pFramesIn, frameCount))
}

// Latency returns the filter's latency in frames.
func (f *HighPassFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maHPFGetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *HighPassFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maHPFUninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}
