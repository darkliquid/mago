package mago

import (
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
