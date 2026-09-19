//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// NotchFilterConfig specifies configuration for a second-order notch filter.
type NotchFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Q          float64
	Frequency  float64
}

// PeakFilterConfig specifies configuration for a second-order peaking EQ filter.
type PeakFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	Q          float64
	Frequency  float64
}

// LowShelfFilterConfig specifies configuration for a second-order low-shelf filter.
type LowShelfFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

// HighShelfFilterConfig specifies configuration for a second-order high-shelf filter.
type HighShelfFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

// Type aliases for convenience.
type (
	Notch2Config     = NotchFilterConfig
	Peak2Config      = PeakFilterConfig
	LoShelf2Config   = LowShelfFilterConfig
	LowShelf2Config  = LowShelfFilterConfig
	HiShelf2Config   = HighShelfFilterConfig
	HighShelf2Config = HighShelfFilterConfig
	Notch2           = NotchFilter
	Peak2            = PeakFilter
	LoShelf2         = LowShelfFilter
	LowShelf2        = LowShelfFilter
	HiShelf2         = HighShelfFilter
	HighShelf2       = HighShelfFilter
)

// NotchFilter wraps miniaudio's ma_notch2 filter.
type NotchFilter struct {
	handle *notch2Handle
	lib    *Library
	config NotchFilterConfig
}

// NewNotchFilter creates and initializes a second-order notch filter.
func (lib *Library) NewNotchFilter(config NotchFilterConfig) (*NotchFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectNotch2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate notch2 failed")
	}
	handle := (*notch2Handle)(raw)

	nativeConfig := notch2ConfigNative(config)

	res := lib.bindings.maNotch2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_notch2_init", res)
	}

	return &NotchFilter{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewNotch2 is an alias for NewNotchFilter.
func (lib *Library) NewNotch2(config NotchFilterConfig) (*NotchFilter, error) {
	return lib.NewNotchFilter(config)
}

// Reinit reconfigures the notch filter without reallocating.
func (f *NotchFilter) Reinit(config NotchFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil notch2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := notch2ConfigNative(config)
	if err := f.lib.resultError("ma_notch2_reinit", f.lib.bindings.maNotch2Reinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *NotchFilter) Process(out, in []float32) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil notch2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_notch2_process_pcm_frames",
		f.lib.bindings.maNotch2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *NotchFilter) ProcessS16(out, in []int16) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil notch2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_notch2_process_pcm_frames",
		f.lib.bindings.maNotch2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (f *NotchFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maNotch2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *NotchFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maNotch2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// PeakFilter wraps miniaudio's ma_peak2 filter.
type PeakFilter struct {
	handle *peak2Handle
	lib    *Library
	config PeakFilterConfig
}

// NewPeakFilter creates and initializes a second-order peaking EQ filter.
func (lib *Library) NewPeakFilter(config PeakFilterConfig) (*PeakFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectPeak2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate peak2 failed")
	}
	handle := (*peak2Handle)(raw)

	nativeConfig := peak2ConfigNative(config)

	res := lib.bindings.maPeak2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_peak2_init", res)
	}

	return &PeakFilter{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewPeak2 is an alias for NewPeakFilter.
func (lib *Library) NewPeak2(config PeakFilterConfig) (*PeakFilter, error) {
	return lib.NewPeakFilter(config)
}

// Reinit reconfigures the peaking filter without reallocating.
func (f *PeakFilter) Reinit(config PeakFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil peak2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := peak2ConfigNative(config)
	if err := f.lib.resultError("ma_peak2_reinit", f.lib.bindings.maPeak2Reinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *PeakFilter) Process(out, in []float32) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil peak2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_peak2_process_pcm_frames",
		f.lib.bindings.maPeak2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *PeakFilter) ProcessS16(out, in []int16) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil peak2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_peak2_process_pcm_frames",
		f.lib.bindings.maPeak2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (f *PeakFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maPeak2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *PeakFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maPeak2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// LowShelfFilter wraps miniaudio's ma_loshelf2 filter.
type LowShelfFilter struct {
	handle *loshelf2Handle
	lib    *Library
	config LowShelfFilterConfig
}

// NewLowShelfFilter creates and initializes a second-order low-shelf filter.
func (lib *Library) NewLowShelfFilter(config LowShelfFilterConfig) (*LowShelfFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectLoShelf2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate loshelf2 failed")
	}
	handle := (*loshelf2Handle)(raw)

	nativeConfig := loshelf2ConfigNative(config)

	res := lib.bindings.maLoShelf2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_loshelf2_init", res)
	}

	return &LowShelfFilter{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewLoShelf2 is an alias for NewLowShelfFilter.
func (lib *Library) NewLoShelf2(config LowShelfFilterConfig) (*LowShelfFilter, error) {
	return lib.NewLowShelfFilter(config)
}

// NewLowShelf2 is an alias for NewLowShelfFilter.
func (lib *Library) NewLowShelf2(config LowShelfFilterConfig) (*LowShelfFilter, error) {
	return lib.NewLowShelfFilter(config)
}

// Reinit reconfigures the low-shelf filter without reallocating.
func (f *LowShelfFilter) Reinit(config LowShelfFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil loshelf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := loshelf2ConfigNative(config)
	if err := f.lib.resultError("ma_loshelf2_reinit", f.lib.bindings.maLoShelf2Reinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *LowShelfFilter) Process(out, in []float32) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil loshelf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_loshelf2_process_pcm_frames",
		f.lib.bindings.maLoShelf2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *LowShelfFilter) ProcessS16(out, in []int16) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil loshelf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_loshelf2_process_pcm_frames",
		f.lib.bindings.maLoShelf2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (f *LowShelfFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maLoShelf2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *LowShelfFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maLoShelf2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}

// HighShelfFilter wraps miniaudio's ma_hishelf2 filter.
type HighShelfFilter struct {
	handle *hishelf2Handle
	lib    *Library
	config HighShelfFilterConfig
}

// NewHighShelfFilter creates and initializes a second-order high-shelf filter.
func (lib *Library) NewHighShelfFilter(config HighShelfFilterConfig) (*HighShelfFilter, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectHiShelf2)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate hishelf2 failed")
	}
	handle := (*hishelf2Handle)(raw)

	nativeConfig := hishelf2ConfigNative(config)

	res := lib.bindings.maHiShelf2Init(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_hishelf2_init", res)
	}

	return &HighShelfFilter{
		handle: handle,
		lib:    lib,
		config: config,
	}, nil
}

// NewHiShelf2 is an alias for NewHighShelfFilter.
func (lib *Library) NewHiShelf2(config HighShelfFilterConfig) (*HighShelfFilter, error) {
	return lib.NewHighShelfFilter(config)
}

// NewHighShelf2 is an alias for NewHighShelfFilter.
func (lib *Library) NewHighShelf2(config HighShelfFilterConfig) (*HighShelfFilter, error) {
	return lib.NewHighShelfFilter(config)
}

// Reinit reconfigures the high-shelf filter without reallocating.
func (f *HighShelfFilter) Reinit(config HighShelfFilterConfig) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hishelf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := hishelf2ConfigNative(config)
	if err := f.lib.resultError("ma_hishelf2_reinit", f.lib.bindings.maHiShelf2Reinit(&nativeConfig, f.handle)); err != nil {
		return err
	}
	f.config = config
	return nil
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *HighShelfFilter) Process(out, in []float32) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hishelf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_hishelf2_process_pcm_frames",
		f.lib.bindings.maHiShelf2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (f *HighShelfFilter) ProcessS16(out, in []int16) error {
	if f == nil || f.handle == nil {
		return fmt.Errorf("mago: nil hishelf2")
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(f.config.Channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return f.lib.resultError("ma_hishelf2_process_pcm_frames",
		f.lib.bindings.maHiShelf2ProcessPCMFrames(f.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (f *HighShelfFilter) Latency() uint32 {
	if f == nil || f.handle == nil {
		return 0
	}
	if err := f.lib.ensureOpen(); err != nil {
		return 0
	}
	return f.lib.bindings.maHiShelf2GetLatency(f.handle)
}

// Close uninitializes the filter and frees its native memory.
func (f *HighShelfFilter) Close() error {
	if f == nil || f.handle == nil {
		return nil
	}
	if err := f.lib.ensureOpen(); err != nil {
		return err
	}
	f.lib.bindings.maHiShelf2Uninit(f.handle, nil)
	f.lib.bindings.magoFree(unsafe.Pointer(f.handle))
	f.handle = nil
	return nil
}
