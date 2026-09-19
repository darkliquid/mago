//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// BiquadConfig specifies configuration for a second-order IIR biquad filter.
type BiquadConfig struct {
	Format   Format
	Channels uint32
	B0       float64
	B1       float64
	B2       float64
	A0       float64
	A1       float64
	A2       float64
}

// Biquad wraps miniaudio's ma_biquad filter.
type Biquad struct {
	handle   *biquadHandle
	lib      *Library
	channels uint32
}

// NewBiquad creates and initializes a biquad filter.
func (lib *Library) NewBiquad(config BiquadConfig) (*Biquad, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectBiquad)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate biquad failed")
	}
	handle := (*biquadHandle)(raw)

	nativeConfig := biquadConfigNative(config)

	res := lib.bindings.maBiquadInit(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_biquad_init", res)
	}

	return &Biquad{
		handle:   handle,
		lib:      lib,
		channels: config.Channels,
	}, nil
}

// Reinit reconfigures the biquad filter without reallocating.
func (b *Biquad) Reinit(config BiquadConfig) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil biquad")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}

	nativeConfig := biquadConfigNative(config)
	if err := b.lib.resultError("ma_biquad_reinit", b.lib.bindings.maBiquadReinit(&nativeConfig, b.handle)); err != nil {
		return err
	}
	b.channels = config.Channels
	return nil
}

// ClearCache clears the internal state/history buffer of the filter.
func (b *Biquad) ClearCache() error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil biquad")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	return b.lib.resultError("ma_biquad_clear_cache", b.lib.bindings.maBiquadClearCache(b.handle))
}

// Process processes float32 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (b *Biquad) Process(out, in []float32) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil biquad")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(b.channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return b.lib.resultError("ma_biquad_process_pcm_frames",
		b.lib.bindings.maBiquadProcessPCMFrames(b.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// ProcessS16 processes int16 audio frames through the filter.
// out and in can be the same slice for in-place processing.
func (b *Biquad) ProcessS16(out, in []int16) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil biquad")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	frameCount, err := validateFilterSlices(b.channels, out, in)
	if err != nil || frameCount == 0 {
		return err
	}
	return b.lib.resultError("ma_biquad_process_pcm_frames",
		b.lib.bindings.maBiquadProcessPCMFrames(b.handle, unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), frameCount))
}

// Latency returns the filter's latency in frames.
func (b *Biquad) Latency() uint32 {
	if b == nil || b.handle == nil {
		return 0
	}
	if err := b.lib.ensureOpen(); err != nil {
		return 0
	}
	return b.lib.bindings.maBiquadGetLatency(b.handle)
}

// Close uninitializes the biquad filter and frees its native memory.
func (b *Biquad) Close() error {
	if b == nil || b.handle == nil {
		return nil
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	b.lib.bindings.maBiquadUninit(b.handle, nil)
	b.lib.bindings.magoFree(unsafe.Pointer(b.handle))
	b.handle = nil
	return nil
}
