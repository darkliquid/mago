package mago

import (
	"errors"
	"math"
	"slices"
	"testing"
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

	read1, err1 := n1.Read(buf1)
	read2, err2 := n2.Read(buf2)
	read3, err3 := n3.Read(buf3)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("Read failed: %v, %v, %v", err1, err2, err3)
	}
	if read1 != frameCount || read2 != frameCount || read3 != frameCount {
		t.Fatalf("read counts mismatch: %d, %d, %d", read1, read2, read3)
	}

	if !slices.Equal(buf1, buf2) {
		t.Fatal("identical seeds produced different noise output")
	}
	if slices.Equal(buf1, buf3) {
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
		read, err := n.Read(buf)
		if err != nil {
			_ = n.Close()
			t.Fatalf("Read(%v): %v", ntype, err)
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

	buf := make([]float32, 4)
	if _, err := n.Read(buf); err == nil {
		t.Fatal("read on closed noise should return error")
	}
}

func TestNoiseReadSlice(t *testing.T) {
	lib := newNullLibrary(t)
	n, err := lib.NewNoise(NoiseConfig{
		Format:    FormatF32,
		Channels:  2,
		Type:      NoiseTypeWhite,
		Seed:      1,
		Amplitude: 0.5,
	})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}
	defer func() { _ = n.Close() }()

	buf := make([]float32, 256) // 128 frames for stereo
	read, err := n.Read(buf)
	if err != nil {
		t.Fatalf("n.Read: %v", err)
	}
	if read != 128 {
		t.Errorf("read: got %d frames, want 128", read)
	}

	// Misaligned buffer should fail
	oddBuf := make([]float32, 255)
	if _, err := n.Read(oddBuf); err != ErrInvalidSliceLength {
		t.Errorf("expected ErrInvalidSliceLength, got %v", err)
	}

	// S16 read
	nS16, err := lib.NewNoise(NoiseConfig{
		Format:    FormatS16,
		Channels:  1,
		Type:      NoiseTypeWhite,
		Seed:      2,
		Amplitude: 0.5,
	})
	if err != nil {
		t.Fatalf("NewNoise(s16): %v", err)
	}
	defer func() { _ = nS16.Close() }()

	bufS16 := make([]int16, 64)
	readS16, err := nS16.ReadS16(bufS16)
	if err != nil {
		t.Fatalf("nS16.ReadS16: %v", err)
	}
	if readS16 != 64 {
		t.Errorf("readS16: got %d frames, want 64", readS16)
	}
}

