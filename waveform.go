//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// WaveformConfig describes how a waveform generator should be initialized.
type WaveformConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Type       WaveformType
	Amplitude  float64
	Frequency  float64
}

// Waveform generates periodic audio waveforms (sine, square, triangle, sawtooth).
type Waveform struct {
	lib        *Library
	handle     *waveformHandle
	channels   uint32
	format     Format
	sampleRate uint32
}

// NewWaveform creates and initializes a new Waveform generator.
func (lib *Library) NewWaveform(config WaveformConfig) (*Waveform, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	native := waveformConfigNative(config)

	handle := (*waveformHandle)(lib.bindings.magoAlloc(magoObjectWaveform))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate waveform: out of memory")
	}

	result := lib.bindings.maWaveformInit(&native, handle)
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_waveform_init", result)
	}

	return &Waveform{lib: lib, handle: handle, channels: config.Channels, format: config.Format, sampleRate: config.SampleRate}, nil
}

// Read writes up to len(out)/channels frames into out, returning how many frames were written.
// out must contain an exact multiple of the configured channel count.
func (w *Waveform) Read(out []float32) (uint64, error) {
	if w == nil || w.handle == nil {
		return 0, fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if w.channels == 0 || len(out)%int(w.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCount := uint64(len(out) / int(w.channels))
	var framesRead uint64
	result := w.lib.bindings.maWaveformReadPCMFrames(w.handle, unsafe.Pointer(&out[0]), frameCount, &framesRead)
	if result != Success {
		return framesRead, w.lib.resultError("ma_waveform_read_pcm_frames", result)
	}
	return framesRead, nil
}

// SeekToPCMFrame moves the waveform internal cursor to frameIndex.
func (w *Waveform) SeekToPCMFrame(frameIndex uint64) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_seek_to_pcm_frame", w.lib.bindings.maWaveformSeekToPCMFrame(w.handle, frameIndex))
}

// SetAmplitude updates the waveform amplitude.
func (w *Waveform) SetAmplitude(amplitude float64) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_amplitude", w.lib.bindings.maWaveformSetAmplitude(w.handle, amplitude))
}

// SetFrequency updates the waveform frequency in Hz.
func (w *Waveform) SetFrequency(frequency float64) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_frequency", w.lib.bindings.maWaveformSetFrequency(w.handle, frequency))
}

// SetType updates the waveform shape (sine, square, triangle, sawtooth).
func (w *Waveform) SetType(waveformType WaveformType) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_type", w.lib.bindings.maWaveformSetType(w.handle, waveformType))
}

// SetSampleRate updates the waveform sample rate in Hz.
func (w *Waveform) SetSampleRate(sampleRate uint32) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_sample_rate", w.lib.bindings.maWaveformSetSampleRate(w.handle, sampleRate))
}

// Close uninitializes the waveform and frees its native memory.
func (w *Waveform) Close() error {
	if w == nil || w.handle == nil {
		return nil
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	w.lib.bindings.maWaveformUninit(w.handle)
	w.lib.bindings.magoFree(unsafe.Pointer(w.handle))
	w.handle = nil
	return nil
}
