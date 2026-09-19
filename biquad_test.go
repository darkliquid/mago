package mago

import (
	"math"
	"testing"
	"unsafe"
)

func TestBiquadPassthrough(t *testing.T) {
	lib := newNullLibrary(t)

	// Biquad configured for passthrough: y[n] = 1*x[n] + 0*x[n-1] + 0*x[n-2] - 0*y[n-1] - 0*y[n-2]
	// a0 = 1, b0 = 1, others = 0
	bq, err := lib.NewBiquad(BiquadConfig{
		Format:   FormatF32,
		Channels: 1,
		B0:       1.0,
		B1:       0.0,
		B2:       0.0,
		A0:       1.0,
		A1:       0.0,
		A2:       0.0,
	})
	if err != nil {
		t.Fatalf("NewBiquad: %v", err)
	}
	defer func() { _ = bq.Close() }()

	input := []float32{0.1, -0.5, 0.8, -0.2, 0.0, 0.99, -0.99, 0.42}
	output := make([]float32, len(input))

	if err := bq.ProcessPCMFrames(unsafe.Pointer(&output[0]), unsafe.Pointer(&input[0]), uint64(len(input))); err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
	}

	for i := range input {
		if math.Abs(float64(output[i]-input[i])) > 1e-6 {
			t.Errorf("frame %d: got %f, want %f", i, output[i], input[i])
		}
	}

	// In-place processing
	inPlace := make([]float32, len(input))
	copy(inPlace, input)
	if err := bq.ProcessPCMFrames(unsafe.Pointer(&inPlace[0]), unsafe.Pointer(&inPlace[0]), uint64(len(inPlace))); err != nil {
		t.Fatalf("ProcessPCMFrames in-place: %v", err)
	}
	for i := range input {
		if math.Abs(float64(inPlace[i]-input[i])) > 1e-6 {
			t.Errorf("in-place frame %d: got %f, want %f", i, inPlace[i], input[i])
		}
	}
}

func TestBiquadControlsAndLifecycle(t *testing.T) {
	lib := newNullLibrary(t)

	bq, err := lib.NewBiquad(BiquadConfig{
		Format:   FormatF32,
		Channels: 2,
		B0:       1.0,
		A0:       1.0,
	})
	if err != nil {
		t.Fatalf("NewBiquad: %v", err)
	}

	_ = bq.Latency()

	if err := bq.ClearCache(); err != nil {
		t.Errorf("ClearCache: %v", err)
	}

	if err := bq.Reinit(BiquadConfig{
		Format:   FormatF32,
		Channels: 2,
		B0:       0.5,
		A0:       1.0,
	}); err != nil {
		t.Errorf("Reinit: %v", err)
	}

	if err := bq.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := bq.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var dummy [2]float32
	if err := bq.ProcessPCMFrames(unsafe.Pointer(&dummy[0]), unsafe.Pointer(&dummy[0]), 1); err == nil {
		t.Fatal("process on closed biquad should fail")
	}
	if err := bq.Reinit(BiquadConfig{Format: FormatF32, Channels: 2, B0: 1, A0: 1}); err == nil {
		t.Fatal("reinit on closed biquad should fail")
	}
	if err := bq.ClearCache(); err == nil {
		t.Fatal("clear cache on closed biquad should fail")
	}

	// Nil receiver checks
	var nilBQ *Biquad
	if err := nilBQ.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
	if err := nilBQ.ProcessPCMFrames(nil, nil, 0); err == nil {
		t.Error("nil ProcessPCMFrames should return error")
	}
	if err := nilBQ.Reinit(BiquadConfig{}); err == nil {
		t.Error("nil Reinit should return error")
	}
	if err := nilBQ.ClearCache(); err == nil {
		t.Error("nil ClearCache should return error")
	}
	if lat := nilBQ.Latency(); lat != 0 {
		t.Errorf("nil Latency: got %d, want 0", lat)
	}
}
