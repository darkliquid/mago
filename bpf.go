//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// BandPassFilter2Config specifies configuration for a second-order band-pass filter.
type BandPassFilter2Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}

// BandPassFilterConfig specifies configuration for an Nth-order band-pass filter.
// Order must be an even number.
type BandPassFilterConfig struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

// Type aliases for convenience.
type (
	BPF2Config = BandPassFilter2Config
	BPFConfig  = BandPassFilterConfig
	BPF2       = BandPassFilter2
	BPF        = BandPassFilter
)

// BandPassFilter2 wraps miniaudio's ma_bpf2 filter.
type BandPassFilter2 struct {
	handle *bpf2Handle
	lib    *Library
	config BandPassFilter2Config
}

// NewBandPassFilter2 creates and initializes a second-order band-pass filter.
func (lib *Library) NewBandPassFilter2(config BandPassFilter2Config) (*BandPassFilter2, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectBPF2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate bpf2 failed")
	}
	handle := (*bpf2Handle)(raw)

	nativeConfig := bpf2ConfigNative(config)

	res := lib.bindings.maBPF2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_bpf2_init", res)
	}

	return &BandPassFilter2{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewBPF2 is an alias for NewBandPassFilter2.
func (lib *Library) NewBPF2(config BandPassFilter2Config) (*BandPassFilter2, error) {
	return lib.NewBandPassFilter2(config)
}

// Reinit reconfigures the second-order band-pass filter without reallocating.
func (f *BandPassFilter2) Reinit(config BandPassFilter2Config) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil bpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := bpf2ConfigNative(config)
	if err := f.lib.resultError("ma_bpf2_reinit", f.lib.bindings.maBPF2Reinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *BandPassFilter2) Process(out, in []float32) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil bpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_bpf2_process_pcm_frames",
		f.lib.bindings.maBPF2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *BandPassFilter2) ProcessS16(out, in []int16) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil bpf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_bpf2_process_pcm_frames",
		f.lib.bindings.maBPF2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (f *BandPassFilter2) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maBPF2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *BandPassFilter2) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maBPF2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// BandPassFilter wraps miniaudio's ma_bpf filter.
type BandPassFilter struct {
	handle *bpfHandle
	lib    *Library
	config BandPassFilterConfig
}

// NewBandPassFilter creates and initializes an Nth-order band-pass filter.
// The order must be an even number.
func (lib *Library) NewBandPassFilter(config BandPassFilterConfig) (*BandPassFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectBPF)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate bpf failed")
	}
	handle := (*bpfHandle)(raw)

	nativeConfig := bpfConfigNative(config)

	res := lib.bindings.maBPFInit(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_bpf_init", res)
	}

	return &BandPassFilter{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewBPF is an alias for NewBandPassFilter.
func (lib *Library) NewBPF(config BandPassFilterConfig) (*BandPassFilter, error) {
	return lib.NewBandPassFilter(config)
}

// Reinit reconfigures the Nth-order band-pass filter without reallocating.
func (f *BandPassFilter) Reinit(config BandPassFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil bpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := bpfConfigNative(config)
	if err := f.lib.resultError("ma_bpf_reinit", f.lib.bindings.maBPFReinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *BandPassFilter) Process(out, in []float32) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil bpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_bpf_process_pcm_frames",
		f.lib.bindings.maBPFProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *BandPassFilter) ProcessS16(out, in []int16) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil bpf")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_bpf_process_pcm_frames",
		f.lib.bindings.maBPFProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (f *BandPassFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maBPFGetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *BandPassFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maBPFUninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}
