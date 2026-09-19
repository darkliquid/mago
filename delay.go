//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// DelayConfig specifies configuration for a delay line effect.
type DelayConfig struct {
	Channels      uint32
	SampleRate    uint32
	DelayInFrames uint32
	DelayStart    bool
	Wet           float32
	Dry           float32
	Decay         float32
}

// DefaultDelayConfig creates a standard delay configuration.
func DefaultDelayConfig(channels, sampleRate, delayInFrames uint32, decay float32) DelayConfig {
	return DelayConfig{
		Channels:      channels,
		SampleRate:    sampleRate,
		DelayInFrames: delayInFrames,
		DelayStart:    decay == 0,
		Wet:           1.0,
		Dry:           1.0,
		Decay:         decay,
	}
}

// Delay wraps miniaudio's ma_delay line effect.
type Delay struct {
	handle *delayHandle
	lib    *Library
}

// NewDelay creates and initializes a delay line effect.
func (lib *Library) NewDelay(config DelayConfig) (*Delay, error) {
	if lib == nil {
		return nil, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	raw := lib.bindings.magoAlloc(magoObjectDelay)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate delay failed")
	}
	handle := (*delayHandle)(raw)

	var delayStart uint32
	if config.DelayStart {
		delayStart = 1
	}

	nativeConfig := delayConfigNative{
		Channels:      config.Channels,
		SampleRate:    config.SampleRate,
		DelayInFrames: config.DelayInFrames,
		DelayStart:    delayStart,
		Wet:           config.Wet,
		Dry:           config.Dry,
		Decay:         config.Decay,
	}

	res := lib.bindings.maDelayInit(&nativeConfig, nil, handle)
	if res != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_delay_init", res)
	}

	return &Delay{
		handle: handle,
		lib:    lib,
	}, nil
}

// ProcessPCMFrames processes audio frames through the delay effect.
// pFramesOut and pFramesIn can point to the same buffer for in-place processing.
func (d *Delay) ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint32) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil delay")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}
	return d.lib.resultError("ma_delay_process_pcm_frames", d.lib.bindings.maDelayProcessPCMFrames(d.handle, pFramesOut, pFramesIn, frameCount))
}

// Wet returns the wet mix factor (0.0 to 1.0).
func (d *Delay) Wet() float32 {
	if d == nil || d.handle == nil {
		return 0
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0
	}
	return d.lib.bindings.maDelayGetWet(d.handle)
}

// SetWet sets the wet mix factor (0.0 to 1.0).
func (d *Delay) SetWet(wet float32) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil delay")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}
	d.lib.bindings.maDelaySetWet(d.handle, wet)
	return nil
}

// Dry returns the dry mix factor (0.0 to 1.0).
func (d *Delay) Dry() float32 {
	if d == nil || d.handle == nil {
		return 0
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0
	}
	return d.lib.bindings.maDelayGetDry(d.handle)
}

// SetDry sets the dry mix factor (0.0 to 1.0).
func (d *Delay) SetDry(dry float32) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil delay")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}
	d.lib.bindings.maDelaySetDry(d.handle, dry)
	return nil
}

// Decay returns the feedback decay factor (0.0 to 1.0).
func (d *Delay) Decay() float32 {
	if d == nil || d.handle == nil {
		return 0
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0
	}
	return d.lib.bindings.maDelayGetDecay(d.handle)
}

// SetDecay sets the feedback decay factor (0.0 to 1.0).
func (d *Delay) SetDecay(decay float32) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil delay")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}
	d.lib.bindings.maDelaySetDecay(d.handle, decay)
	return nil
}

// Close uninitializes the delay line and frees its native memory.
func (d *Delay) Close() error {
	if d == nil || d.handle == nil {
		return nil
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}
	d.lib.bindings.maDelayUninit(d.handle, nil)
	d.lib.bindings.magoFree(unsafe.Pointer(d.handle))
	d.handle = nil
	return nil
}
