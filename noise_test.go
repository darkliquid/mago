package mago

import (
	"bytes"
	"errors"
	"math"
	"testing"
	"unsafe"
)

func TestNoiseSeededReproducibility(t *testing.T) {
	lib := newNullLibrary(t)

	newNoiseWithSeed := func(seed int32) *Noise {
		n, err := lib.NewNoise(NoiseConfig{
			Format:    FormatF32,
			Channels:  1,
			Type:      NoiseTypeWhite,
			Seed:      seed,
			Amplitude: 0.5,
		})
		if err != nil {
			t.Fatalf("NewNoise: %v", err)
		}
		return n
	}

	n1 := newNoiseWithSeed(12345)
	defer func() { _ = n1.Close() }()
	n2 := newNoiseWithSeed(12345)
	defer func() { _ = n2.Close() }()
	n3 := newNoiseWithSeed(54321)
	defer func() { _ = n3.Close() }()

	const frameCount = 128
	buf1 := make([]float32, frameCount)
	buf2 := make([]float32, frameCount)
	buf3 := make([]float32, frameCount)

	read1, err1 := n1.ReadPCMFrames(unsafe.Pointer(&buf1[0]), frameCount)
	read2, err2 := n2.ReadPCMFrames(unsafe.Pointer(&buf2[0]), frameCount)
	read3, err3 := n3.ReadPCMFrames(unsafe.Pointer(&buf3[0]), frameCount)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("ReadPCMFrames failed: %v, %v, %v", err1, err2, err3)
	}
	if read1 != frameCount || read2 != frameCount || read3 != frameCount {
		t.Fatalf("read counts mismatch: %d, %d, %d", read1, read2, read3)
	}

	b1 := unsafe.Slice((*byte)(unsafe.Pointer(&buf1[0])), frameCount*4)
	b2 := unsafe.Slice((*byte)(unsafe.Pointer(&buf2[0])), frameCount*4)
	b3 := unsafe.Slice((*byte)(unsafe.Pointer(&buf3[0])), frameCount*4)

	if !bytes.Equal(b1, b2) {
		t.Fatal("identical seeds produced different noise output")
	}
	if bytes.Equal(b1, b3) {
		t.Fatal("different seeds produced identical noise output")
	}
}

func TestNoiseTypes(t *testing.T) {
	lib := newNullLibrary(t)

	types := []NoiseType{
		NoiseTypeWhite,
		NoiseTypePink,
		NoiseTypeBrownian,
	}

	for _, ntype := range types {
		n, err := lib.NewNoise(NoiseConfig{
			Format:    FormatF32,
			Channels:  2,
			Type:      ntype,
			Seed:      42,
			Amplitude: 0.6,
		})
		if err != nil {
			t.Fatalf("NewNoise(%v): %v", ntype, err)
		}

		buf := make([]float32, 200) // 100 frames * 2 channels
		read, err := n.ReadPCMFrames(unsafe.Pointer(&buf[0]), 100)
		if err != nil {
			_ = n.Close()
			t.Fatalf("ReadPCMFrames(%v): %v", ntype, err)
		}
		if read != 100 {
			_ = n.Close()
			t.Fatalf("read %d frames for %v, want 100", read, ntype)
		}

		hasNonZero := false
		for _, v := range buf {
			if v != 0 {
				hasNonZero = true
			}
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || math.Abs(float64(v)) > 10.0 {
				_ = n.Close()
				t.Fatalf("sample %f exceeds reasonable bounds for %v", v, ntype)
			}
		}
		if !hasNonZero {
			_ = n.Close()
			t.Fatalf("noise %v generated all zeroes", ntype)
		}

		if err := n.Close(); err != nil {
			t.Fatalf("Close(%v): %v", ntype, err)
		}
	}
}

func TestNoiseControls(t *testing.T) {
	lib := newNullLibrary(t)

	n, err := lib.NewNoise(NoiseConfig{
		Format:    FormatF32,
		Channels:  1,
		Type:      NoiseTypeWhite,
		Seed:      100,
		Amplitude: 0.5,
	})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}
	defer func() { _ = n.Close() }()

	if err := n.SetAmplitude(0.2); err != nil {
		t.Errorf("SetAmplitude: %v", err)
	}

	// Reseeding to the same initial seed should repeat the pseudo-random sequence
	if err := n.SetSeed(100); err != nil {
		t.Errorf("SetSeed: %v", err)
	}

	// miniaudio rejects dynamic SetType with MA_INVALID_OPERATION
	var opErr *OpError
	if err := n.SetType(NoiseTypePink); err == nil {
		t.Error("SetType expected error (unsupported by miniaudio), got nil")
	} else if errors.As(err, &opErr) && opErr.Code != InvalidOperation {
		t.Errorf("SetType expected InvalidOperation (%d), got %d", InvalidOperation, opErr.Code)
	}
}

func TestNoiseLifecycle(t *testing.T) {
	lib := newNullLibrary(t)

	n, err := lib.NewNoise(NoiseConfig{
		Format:    FormatF32,
		Channels:  1,
		Type:      NoiseTypeWhite,
		Seed:      1,
		Amplitude: 0.5,
	})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}

	if err := n.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := n.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var buf [4]float32
	if _, err := n.ReadPCMFrames(unsafe.Pointer(&buf[0]), 4); err == nil {
		t.Fatal("read on closed noise should return error")
	}
}
