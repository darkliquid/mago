package mago

import (
	"testing"
	"unsafe"
)

func TestHPF1FrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 10000.0
		frames     = 2048
	)

	hpf, err := lib.NewHighPassFilter1(HighPassFilter1Config{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
	})
	if err != nil {
		t.Fatalf("NewHighPassFilter1: %v", err)
	}
	defer func() { _ = hpf.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 100.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 20000.0, frames)

	outLow := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&outLow[0]), unsafe.Pointer(&lowFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames low: %v", err)
	}
	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&outHigh[0]), unsafe.Pointer(&highFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames high: %v", err)
	}

	lowRMS := rms(outLow)
	highRMS := rms(outHigh)

	if highRMS < 0.25 {
		t.Errorf("expected passband RMS >= 0.25, got %f", highRMS)
	}
	if lowRMS > highRMS*0.5 {
		t.Errorf("expected stopband RMS (%f) to be attenuated relative to passband (%f)", lowRMS, highRMS)
	}
}

func TestHPF2FrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 1000.0
		frames     = 2048
	)

	hpf, err := lib.NewHighPassFilter2(HighPassFilter2Config{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
		Q:               0.707,
	})
	if err != nil {
		t.Fatalf("NewHighPassFilter2: %v", err)
	}
	defer func() { _ = hpf.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 100.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 5000.0, frames)

	outLow := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&outLow[0]), unsafe.Pointer(&lowFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames low: %v", err)
	}
	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&outHigh[0]), unsafe.Pointer(&highFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames high: %v", err)
	}

	lowRMS := rms(outLow)
	highRMS := rms(outHigh)

	if highRMS < 0.5 {
		t.Errorf("expected passband RMS >= 0.5, got %f", highRMS)
	}
	if lowRMS > highRMS*0.1 {
		t.Errorf("expected 2nd order stopband RMS (%f) to be heavily attenuated relative to passband (%f)", lowRMS, highRMS)
	}
}

func TestHPFNthOrderFrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		cutoff     = 1000.0
		frames     = 2048
	)

	hpf, err := lib.NewHighPassFilter(HighPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: cutoff,
		Order:           4,
	})
	if err != nil {
		t.Fatalf("NewHighPassFilter: %v", err)
	}
	defer func() { _ = hpf.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 100.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 5000.0, frames)

	outLow := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&outLow[0]), unsafe.Pointer(&lowFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames low: %v", err)
	}
	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&outHigh[0]), unsafe.Pointer(&highFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames high: %v", err)
	}

	lowRMS := rms(outLow)
	highRMS := rms(outHigh)

	if highRMS < 0.5 {
		t.Errorf("expected passband RMS >= 0.5, got %f", highRMS)
	}
	if lowRMS > highRMS*0.05 {
		t.Errorf("expected 4th order stopband RMS to be heavily attenuated (< %f), got %f", highRMS*0.05, lowRMS)
	}
}

func TestHPFLifecycleAndControls(t *testing.T) {
	lib := newNullLibrary(t)

	hpf, err := lib.NewHighPassFilter(HighPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 1000,
		Order:           2,
	})
	if err != nil {
		t.Fatalf("NewHighPassFilter: %v", err)
	}

	_ = hpf.Latency()

	if err := hpf.Reinit(HighPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 2000,
		Order:           2,
	}); err != nil {
		t.Errorf("Reinit: %v", err)
	}

	if err := hpf.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := hpf.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var dummy [4]float32
	if err := hpf.ProcessPCMFrames(unsafe.Pointer(&dummy[0]), unsafe.Pointer(&dummy[0]), 4); err == nil {
		t.Fatal("process on closed HPF should fail")
	}

	var nilHPF *HighPassFilter
	if err := nilHPF.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
	if err := nilHPF.ProcessPCMFrames(nil, nil, 0); err == nil {
		t.Error("nil ProcessPCMFrames should return error")
	}
}
