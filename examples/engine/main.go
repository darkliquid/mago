// Command engine shows miniaudio's high-level engine without an audio device: it
// builds an engine, plays an in-memory tone through a group, and moves the
// listener to demonstrate distance attenuation.
package main

import (
	"fmt"
	"math"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

const (
	channels   = 2
	sampleRate = 48000
	blockSize  = 1024
)

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	// NoDevice means the engine is mixed by hand with Read, so this runs with no
	// audio hardware at all.
	config := mago.DefaultEngineConfig(channels, sampleRate)
	config.NoDevice = true

	engine, err := lib.NewEngine(config)
	example.Must(err)

	fmt.Printf("engine: %d channel(s) at %d Hz, %d listener(s)\n",
		engine.Channels(), engine.SampleRate(), engine.ListenerCount())

	// A mono tone spatialized into the engine's stereo output.
	waveform, err := lib.NewWaveform(mago.WaveformConfig{
		Format:     mago.FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Type:       mago.WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440,
	})
	example.Must(err)

	// A group cascades its volume and pan down to every sound attached to it.
	music, err := engine.NewSoundGroup(mago.SoundGroupConfig{})
	example.Must(err)
	example.Must(music.SetVolume(0.5))

	sound, err := engine.NewSoundFromDataSource(waveform, mago.SoundConfig{Group: music})
	example.Must(err)
	example.Must(sound.SetPosition(mago.Vec3{}))

	fmt.Println("routing: waveform -> sound -> group(0.5) -> endpoint")

	// A new sound is stopped, so it has to be started.
	example.Must(sound.Start())
	fmt.Printf("playing: %v, volume %.2f\n", sound.IsPlaying(), sound.Volume())

	peakAt := func() float64 {
		samples := make([]float32, blockSize*int(engine.Channels()))
		example.Must(func() error {
			_, err := engine.Read(samples)
			return err
		}())

		peak := 0.0
		for _, sample := range samples {
			if magnitude := math.Abs(float64(sample)); magnitude > peak {
				peak = magnitude
			}
		}
		return peak
	}

	// The engine smooths spatialized gain over a few milliseconds, so read a
	// couple of blocks before measuring.
	peakAt()
	fmt.Printf("at the listener:    peak %.4f\n", peakAt())

	listener := engine.Listener(0)
	for _, distance := range []float32{2, 4, 8} {
		example.Must(listener.SetPosition(mago.Vec3{X: distance}))
		peakAt()
		fmt.Printf("listener at x=%d:  peak %.4f\n", int(distance), peakAt())
	}

	// Everything the listener does is readable back.
	position := listener.Position()
	fmt.Printf("listener position: x=%.1f y=%.1f z=%.1f\n", position.X, position.Y, position.Z)

	// Muting the group silences the sound underneath it.
	example.Must(music.SetVolume(0))
	fmt.Printf("group muted:       peak %.4f\n", peakAt())

	example.Must(sound.Close())
	example.Must(music.Close())
	example.Must(engine.Close())
	example.Must(waveform.Close())
}
