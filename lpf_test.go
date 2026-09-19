package mago

import (
	"math"
	"testing"
)

func rms(samples []float32) float64 {
	var sum float64
	for _, s := range samples {
		sum += float64(s * s)
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func generateSine(t *testing.T, lib *Library, sampleRate uint32, freq float64, frameCount uint64) []float32 {
	t.Helper()
	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Type:       WaveformTypeSine,
		Amplitude:  0.8,
		Frequency:  freq,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = wf.Close() }()

	buf := make([]float32, frameCount)
	read, err := wf.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read != frameCount {
		t.Fatalf("read %d frames, want %d", read, frameCount)
	}
	return buf
}

func TestLPF1FrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 1000.0
		frames     = 2048
	)

	lpf, err := lib.NewLowPassFilter1(LowPassFilter1Config{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
	})
	if err != nil {
		t.Fatalf("NewLowPassFilter1: %v", err)
	}
	defer func() { _ = lpf.Close() }()

	lowIn := generateSine(t, lib, sampleRate, 100.0, frames)
	lowOut := make([]float32, frames)
	if err := lpf.Process(lowOut, lowIn); err != nil {
		t.Fatalf("Process low: %v", err)
	}

	_ = lpf.ClearCache()

	highIn := generateSine(t, lib, sampleRate, 8000.0, frames)
	highOut := make([]float32, frames)
	if err := lpf.Process(highOut, highIn); err != nil {
		t.Fatalf("Process high: %v", err)
	}

	lowRMS := rms(lowOut[512:])
	highRMS := rms(highOut[512:])

	if lowRMS < 0.5 {
		t.Errorf("expected passband RMS >= 0.5, got %f", lowRMS)
	}
	if highRMS > lowRMS*0.2 {
		t.Errorf("expected stopband RMS to be attenuated (< %f), got %f", lowRMS*0.2, highRMS)
	}
}

func TestLPF2FrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 1000.0
		frames     = 2048
	)

	lpf, err := lib.NewLowPassFilter2(LowPassFilter2Config{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
		Q:               1.0,
	})
	if err != nil {
		t.Fatalf("NewLowPassFilter2: %v", err)
	}
	defer func() { _ = lpf.Close() }()

	lowIn := generateSine(t, lib, sampleRate, 100.0, frames)
	lowOut := make([]float32, frames)
	if err := lpf.Process(lowOut, lowIn); err != nil {
		t.Fatalf("Process low: %v", err)
	}

	_ = lpf.ClearCache()

	highIn := generateSine(t, lib, sampleRate, 8000.0, frames)
	highOut := make([]float32, frames)
	if err := lpf.Process(highOut, highIn); err != nil {
		t.Fatalf("Process high: %v", err)
	}

	lowRMS := rms(lowOut[512:])
	highRMS := rms(highOut[512:])

	if lowRMS < 0.5 {
		t.Errorf("expected passband RMS >= 0.5, got %f", lowRMS)
	}
	if highRMS > lowRMS*0.1 {
		t.Errorf("expected 2nd order stopband RMS to be strongly attenuated (< %f), got %f", lowRMS*0.1, highRMS)
	}
}

func TestLPFNthOrderFrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 1000.0
		frames     = 2048
	)

	lpf, err := lib.NewLowPassFilter(LowPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
		Order:           4,
	})
	if err != nil {
		t.Fatalf("NewLowPassFilter: %v", err)
	}
	defer func() { _ = lpf.Close() }()

	lowIn := generateSine(t, lib, sampleRate, 100.0, frames)
	lowOut := make([]float32, frames)
	if err := lpf.Process(lowOut, lowIn); err != nil {
		t.Fatalf("Process low: %v", err)
	}

	_ = lpf.ClearCache()

	highIn := generateSine(t, lib, sampleRate, 8000.0, frames)
	highOut := make([]float32, frames)
	if err := lpf.Process(highOut, highIn); err != nil {
		t.Fatalf("Process high: %v", err)
	}

	lowRMS := rms(lowOut[512:])
	highRMS := rms(highOut[512:])

	if lowRMS < 0.5 {
		t.Errorf("expected passband RMS >= 0.5, got %f", lowRMS)
	}
	if highRMS > lowRMS*0.05 {
		t.Errorf("expected 4th order stopband RMS to be heavily attenuated (< %f), got %f", lowRMS*0.05, highRMS)
	}
}

func TestLPFLifecycleAndControls(t *testing.T) {
	lib := newNullLibrary(t)

	lpf, err := lib.NewLowPassFilter(LowPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 1000,
		Order:           2,
	})
	if err != nil {
		t.Fatalf("NewLowPassFilter: %v", err)
	}

	_ = lpf.Latency()

	if err := lpf.ClearCache(); err != nil {
		t.Errorf("ClearCache: %v", err)
	}

	if err := lpf.Reinit(LowPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 2000,
		Order:           2,
	}); err != nil {
		t.Errorf("Reinit: %v", err)
	}

	if err := lpf.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := lpf.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var dummy [4]float32
	if err := lpf.Process(dummy[:], dummy[:]); err == nil {
		t.Fatal("process on closed LPF should fail")
	}

	var nilLPF *LowPassFilter
	if err := nilLPF.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
	if err := nilLPF.Process(nil, nil); err == nil {
		t.Error("nil Process should return error")
	}
}

func TestLPFClearCacheRestoresCoefficients(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 500.0
		frames     = 2048
	)

	// Test LPF1 after ClearCache
	lpf1, err := lib.NewLowPassFilter1(LowPassFilter1Config{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
	})
	if err != nil {
		t.Fatalf("NewLowPassFilter1: %v", err)
	}
	defer func() { _ = lpf1.Close() }()

	if err := lpf1.ClearCache(); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}

	highFreq := generateSine(t, lib, sampleRate, 8000.0, frames)
	out := make([]float32, frames)
	if err := lpf1.Process(out, highFreq); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := rms(out); got > 0.3 {
		t.Errorf("LPF1 after ClearCache expected attenuation, got RMS %f", got)
	}

	// Test odd-order LPF after ClearCache
	lpf3, err := lib.NewLowPassFilter(LowPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
		Order:           3,
	})
	if err != nil {
		t.Fatalf("NewLowPassFilter: %v", err)
	}
	defer func() { _ = lpf3.Close() }()

	if err := lpf3.ClearCache(); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}

	if err := lpf3.Process(out, highFreq); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := rms(out); got > 0.1 {
		t.Errorf("LPF (order 3) after ClearCache expected heavy attenuation, got RMS %f", got)
	}
}
