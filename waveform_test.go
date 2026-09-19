package mago

import (
	"math"
	"testing"
)

func TestWaveformSineKnownSamples(t *testing.T) {
	lib := newNullLibrary(t)

	// 48 kHz, 1 channel, 1000 Hz sine, amplitude 1.0.
	// One full cycle is 48 samples.
	waveform, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  1.0,
		Frequency:  1000.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = waveform.Close() }()

	samples := make([]float32, 48)
	read, err := waveform.Read(samples)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read != uint64(len(samples)) {
		t.Fatalf("read %d frames, want %d", read, len(samples))
	}

	// Sample 0: sin(0) = 0.0
	if math.Abs(float64(samples[0])) > 1e-4 {
		t.Errorf("sample[0] = %f, want ~0.0", samples[0])
	}
	// Sample 12 (quarter cycle): sin(pi/2) = 1.0
	if math.Abs(float64(samples[12])-1.0) > 1e-3 {
		t.Errorf("sample[12] = %f, want ~1.0", samples[12])
	}
	// Sample 24 (half cycle): sin(pi) = 0.0
	if math.Abs(float64(samples[24])) > 1e-3 {
		t.Errorf("sample[24] = %f, want ~0.0", samples[24])
	}
	// Sample 36 (three quarter cycle): sin(3pi/2) = -1.0
	if math.Abs(float64(samples[36])+1.0) > 1e-3 {
		t.Errorf("sample[36] = %f, want ~-1.0", samples[36])
	}
}

func TestWaveformShapes(t *testing.T) {
	lib := newNullLibrary(t)

	shapes := []WaveformType{
		WaveformTypeSine,
		WaveformTypeSquare,
		WaveformTypeTriangle,
		WaveformTypeSawtooth,
	}

	for _, shape := range shapes {
		wf, err := lib.NewWaveform(WaveformConfig{
			Format:     FormatF32,
			Channels:   1,
			SampleRate: 48000,
			Type:       shape,
			Amplitude:  0.8,
			Frequency:  440.0,
		})
		if err != nil {
			t.Fatalf("NewWaveform(%v): %v", shape, err)
		}

		buf := make([]float32, 100)
		read, err := wf.Read(buf)
		if err != nil {
			_ = wf.Close()
			t.Fatalf("Read(%v): %v", shape, err)
		}
		if read != uint64(len(buf)) {
			_ = wf.Close()
			t.Fatalf("read %d frames for shape %v, want %d", read, shape, len(buf))
		}

		for i, v := range buf {
			if float64(v) > 0.81 || float64(v) < -0.81 {
				_ = wf.Close()
				t.Fatalf("shape %v sample[%d] = %f out of amplitude bounds [-0.8, 0.8]", shape, i, v)
			}
		}

		if err := wf.Close(); err != nil {
			t.Fatalf("Close(%v): %v", shape, err)
		}
	}
}

func TestWaveformControls(t *testing.T) {
	lib := newNullLibrary(t)

	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = wf.Close() }()

	if err := wf.SetAmplitude(0.25); err != nil {
		t.Errorf("SetAmplitude: %v", err)
	}
	if err := wf.SetFrequency(880.0); err != nil {
		t.Errorf("SetFrequency: %v", err)
	}
	if err := wf.SetType(WaveformTypeSquare); err != nil {
		t.Errorf("SetType: %v", err)
	}
	if err := wf.SetSampleRate(44100); err != nil {
		t.Errorf("SetSampleRate: %v", err)
	}
	if err := wf.SeekToPCMFrame(0); err != nil {
		t.Errorf("SeekToPCMFrame: %v", err)
	}

	buf := make([]float32, 10)
	read, err := wf.Read(buf)
	if err != nil || read != 10 {
		t.Fatalf("read %d frames (%v), want 10", read, err)
	}
}

func TestWaveformLifecycle(t *testing.T) {
	lib := newNullLibrary(t)

	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}

	if err := wf.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := wf.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	buf := make([]float32, 4)
	if _, err := wf.Read(buf); err == nil {
		t.Fatal("read on closed waveform should return error")
	}
}

func TestWaveformReadSlice(t *testing.T) {
	lib := newNullLibrary(t)
	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   2,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = wf.Close() }()

	buf := make([]float32, 256) // 128 frames for stereo
	read, err := wf.Read(buf)
	if err != nil {
		t.Fatalf("wf.Read: %v", err)
	}
	if read != 128 {
		t.Errorf("read: got %d frames, want 128", read)
	}

	// Misaligned buffer should fail
	oddBuf := make([]float32, 255)
	if _, err := wf.Read(oddBuf); err != ErrInvalidSliceLength {
		t.Errorf("expected ErrInvalidSliceLength, got %v", err)
	}
}
