package mago

import (
	"math"
	"testing"
	"unsafe"
)

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

	usedIn, usedOut, err := resampler.ProcessPCMFrames(
		unsafe.Pointer(&in[0]), uint64(len(in)),
		unsafe.Pointer(&out[0]), uint64(len(out)),
	)
	if err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
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

	_, usedOut, err := resampler.ProcessPCMFrames(
		unsafe.Pointer(&in[0]), uint64(len(in)),
		unsafe.Pointer(&out[0]), uint64(len(out)),
	)
	if err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
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
