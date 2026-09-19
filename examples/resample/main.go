// Command resample shows sample-rate conversion: the general resampler, the
// linear resampler, and the one-call data converter. It needs no audio device.
package main

import (
	"fmt"
	"math"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	const (
		toneFreq   = 440.0
		rateIn     = 24_000
		toneFrames = 480
	)

	tone := make([]float32, toneFrames)
	for i := range tone {
		tone[i] = 0.5 * float32(math.Sin(2*math.Pi*toneFreq*float64(i)/rateIn))
	}

	// General resampler: 24kHz -> 48kHz.
	resampler, err := lib.NewResampler(mago.DefaultResamplerConfig(mago.FormatF32, 1, rateIn, rateIn*2))
	example.Must(err)
	defer func() { example.Must(resampler.Close()) }()

	upsampled := make([]float32, toneFrames*2)
	usedIn, usedOut, err := resampler.Process(tone, upsampled)
	example.Must(err)
	fmt.Printf("Resampler:       consumed %d frames, produced %d frames (24kHz -> 48kHz)\n", usedIn, usedOut)

	if needed, err := resampler.RequiredInputFrameCount(480); err == nil {
		fmt.Printf("                 to produce 480 output frames needs %d input frames\n", needed)
	}

	// Linear resampler: 48kHz -> 24kHz.
	linear, err := lib.NewLinearResampler(mago.DefaultLinearResamplerConfig(mago.FormatF32, 1, rateIn*2, rateIn))
	example.Must(err)
	defer func() { example.Must(linear.Close()) }()

	downsampled := make([]float32, len(upsampled))
	_, linearOut, err := linear.Process(upsampled, downsampled)
	example.Must(err)
	fmt.Printf("LinearResampler: produced %d frames (48kHz -> 24kHz), peak %.3f\n", linearOut, peak(downsampled[:linearOut]))

	// Data converter: stereo f32 48kHz -> mono s16 24kHz in one call.
	converter, err := lib.NewDataConverter(mago.DefaultDataConverterConfig(
		mago.FormatF32, mago.FormatS16, 2, 1, rateIn*2, rateIn,
	))
	example.Must(err)
	defer func() { example.Must(converter.Close()) }()

	stereo := make([]float32, len(upsampled)*2)
	for i, value := range upsampled {
		stereo[2*i] = value
		stereo[2*i+1] = value
	}
	mono := make([]int16, len(upsampled))

	convIn, convOut, err := converter.ProcessF32ToS16(stereo, mono)
	example.Must(err)
	fmt.Printf("DataConverter:   consumed %d stereo frames, produced %d mono s16 frames\n", convIn, convOut)

	if needed, err := converter.RequiredInputFrameCount(uint64(len(mono))); err == nil {
		fmt.Printf("                 to fill the output needs %d input frames\n", needed)
	}
}

func peak(samples []float32) float32 {
	var highest float32
	for _, sample := range samples {
		if magnitude := float32(math.Abs(float64(sample))); magnitude > highest {
			highest = magnitude
		}
	}
	return highest
}
