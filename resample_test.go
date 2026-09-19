package mago

import (
	"math"
	"testing"
)

func TestResamplerProcessSlice(t *testing.T) {
	lib := newNullLibrary(t)
	r, err := lib.NewResampler(ResamplerConfig{
		Format:        FormatF32,
		Channels:      1,
		SampleRateIn:  44100,
		SampleRateOut: 48000,
		Algorithm:     ResampleAlgorithmLinear,
	})
	if err != nil {
		t.Fatalf("NewResampler: %v", err)
	}
	defer func() { _ = r.Close() }()

	in := make([]float32, 441)
	out := make([]float32, 480)
	inRead, outWritten, err := r.Process(in, out)
	if err != nil {
		t.Fatalf("r.Process: %v", err)
	}
	if inRead == 0 || outWritten == 0 {
		t.Errorf("expected frames processed, got in=%d, out=%d", inRead, outWritten)
	}
}

func TestResamplerUpsamplesF32(t *testing.T) {
	lib := newNullLibrary(t)

	resampler, err := lib.NewResampler(DefaultResamplerConfig(FormatF32, 1, 24_000, 48_000))
	if err != nil {
		t.Fatalf("NewResampler: %v", err)
	}
	defer func() { _ = resampler.Close() }()

	in := make([]float32, 240)
	for i := range in {
		in[i] = float32(i) / 240
	}
	out := make([]float32, 960)

	usedIn, usedOut, err := resampler.Process(in, out)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if usedIn == 0 || usedOut == 0 {
		t.Fatalf("expected frames in and out, got %d and %d", usedIn, usedOut)
	}
}

func TestLinearResamplerPreservesToneEnergy(t *testing.T) {
	lib := newNullLibrary(t)

	resampler, err := lib.NewLinearResampler(DefaultLinearResamplerConfig(FormatF32, 1, 48_000, 24_000))
	if err != nil {
		t.Fatalf("NewLinearResampler: %v", err)
	}
	defer func() { _ = resampler.Close() }()

	in := make([]float32, 480)
	for i := range in {
		in[i] = 0.5 * float32(math.Sin(2*math.Pi*440*float64(i)/48_000))
	}
	out := make([]float32, 480)

	_, usedOut, err := resampler.Process(in, out)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	var peak float32
	for i := 0; i < int(usedOut); i++ {
		if out[i] > peak {
			peak = out[i]
		}
	}
	if peak < 0.3 || peak > 0.6 {
		t.Fatalf("down-sampled peak %v, want roughly 0.5", peak)
	}
}

func TestDataConverterChangesFormatChannelsAndRate(t *testing.T) {
	lib := newNullLibrary(t)

	converter, err := lib.NewDataConverter(DefaultDataConverterConfig(
		FormatF32, FormatS16, 2, 1, 48_000, 24_000,
	))
	if err != nil {
		t.Fatalf("NewDataConverter: %v", err)
	}
	defer func() { _ = converter.Close() }()

	in := make([]float32, 480)
	for i := range in {
		in[i] = 0.5
	}
	out := make([]int16, 240)

	_, usedOut, err := converter.ProcessF32ToS16(in, out)
	if err != nil {
		t.Fatalf("ProcessF32ToS16: %v", err)
	}
	if usedOut == 0 {
		t.Fatal("expected output frames")
	}
	// The resampler has a little latency, so the very first output frame can be
	// zero; the rest of the steady-state signal should be the scaled-down 0.5.
	var peak int16
	for i := 0; i < int(usedOut); i++ {
		if out[i] > peak {
			peak = out[i]
		}
	}
	if peak < 1000 {
		t.Fatalf("peak output sample = %d, want a scaled-down 0.5", peak)
	}

	if _, err := converter.InputChannelMap(); err != nil {
		t.Fatalf("InputChannelMap: %v", err)
	}
	if _, err := converter.OutputChannelMap(); err != nil {
		t.Fatalf("OutputChannelMap: %v", err)
	}
}

func TestDataConverterRejectsCustomWeights(t *testing.T) {
	lib := newNullLibrary(t)

	config := DefaultDataConverterConfig(FormatF32, FormatF32, 2, 2, 48_000, 48_000)
	config.ChannelMixMode = ChannelMixModeCustomWeights
	if _, err := lib.NewDataConverter(config); err == nil {
		t.Fatal("expected custom weights to be rejected")
	}
}
