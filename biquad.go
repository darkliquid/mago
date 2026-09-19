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
	handle *biquadHandle
	lib    *Library
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

	nativeConfig := biquadConfigNative{
		Format:   config.Format,
		Channels: config.Channels,
		B0:       config.B0,
		B1:       config.B1,
		B2:       config.B2,
		A0:       config.A0,
		A1:       config.A1,
		A2:       config.A2,
	}

	res := lib.bindings.maBiquadInit(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_biquad_init", res)
	}

	return &Biquad{
		handle: handle,
		lib:    lib,
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

	nativeConfig := biquadConfigNative{
		Format:   config.Format,
		Channels: config.Channels,
		B0:       config.B0,
		B1:       config.B1,
		B2:       config.B2,
		A0:       config.A0,
		A1:       config.A1,
		A2:       config.A2,
	}
	return b.lib.resultError("ma_biquad_reinit", b.lib.bindings.maBiquadReinit(&nativeConfig, b.handle))
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

// ProcessPCMFrames processes audio frames through the filter.
// pFramesOut and pFramesIn can point to the same buffer for in-place processing.
func (b *Biquad) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint64) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil biquad")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	return b.lib.resultError("ma_biquad_process_pcm_frames", b.lib.bindings.maBiquadProcessPCMFrames(b.handle, pFramesOut, pFramesIn, frameCount))
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
