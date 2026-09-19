//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// NoiseConfig describes how a noise generator should be initialized.
type NoiseConfig struct {
	Format            Format
	Channels          uint32
	Type              NoiseType
	Seed              int32
	Amplitude         float64
	DuplicateChannels bool
}

// Noise generates white, pink, or brownian noise.
type Noise struct {
	lib    *Library
	handle *noiseHandle
}

// NewNoise creates and initializes a new Noise generator.
func (lib *Library) NewNoise(config NoiseConfig) (*Noise, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	native := noiseConfigNative{
		Format:            config.Format,
		Channels:          config.Channels,
		Type:              config.Type,
		Seed:              config.Seed,
		Amplitude:         config.Amplitude,
		DuplicateChannels: boolToBool32(config.DuplicateChannels),
	}

	handle := (*noiseHandle)(lib.bindings.magoAlloc(magoObjectNoise))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate noise: out of memory")
	}

	result := lib.bindings.maNoiseInit(&native, nil, handle)
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_noise_init", result)
	}

	return &Noise{lib: lib, handle: handle}, nil
}

// ReadPCMFrames writes up to frameCount frames into out, returning how many were written.
func (n *Noise) ReadPCMFrames(out unsafe.Pointer, frameCount uint64) (uint64, error) {
	if n == nil || n.handle == nil {
		return 0, fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var framesRead uint64
	result := n.lib.bindings.maNoiseReadPCMFrames(n.handle, out, frameCount, &framesRead)
	if result != Success {
		return framesRead, n.lib.resultError("ma_noise_read_pcm_frames", result)
	}
	return framesRead, nil
}

// SetAmplitude updates the noise amplitude.
func (n *Noise) SetAmplitude(amplitude float64) error {
	if n == nil || n.handle == nil {
		return fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	return n.lib.resultError("ma_noise_set_amplitude", n.lib.bindings.maNoiseSetAmplitude(n.handle, amplitude))
}

// SetSeed updates the random number generator seed.
func (n *Noise) SetSeed(seed int32) error {
	if n == nil || n.handle == nil {
		return fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	return n.lib.resultError("ma_noise_set_seed", n.lib.bindings.maNoiseSetSeed(n.handle, seed))
}

// SetType attempts to change the noise type dynamically.
// Note: miniaudio 0.11.25 does not support dynamic noise type changes and will return ResultInvalidOperation.
func (n *Noise) SetType(noiseType NoiseType) error {
	if n == nil || n.handle == nil {
		return fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	return n.lib.resultError("ma_noise_set_type", n.lib.bindings.maNoiseSetType(n.handle, noiseType))
}

// Close uninitializes the noise generator and frees its native memory.
func (n *Noise) Close() error {
	if n == nil || n.handle == nil {
		return nil
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	n.lib.bindings.maNoiseUninit(n.handle, nil)
	n.lib.bindings.magoFree(unsafe.Pointer(n.handle))
	n.handle = nil
	return nil
}
