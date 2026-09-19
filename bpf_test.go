package mago

import (
	"testing"
	"unsafe"
)

func TestBPF2FrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		centerFreq = 1000.0
		frames     = 2048
	)

	bpf, err := lib.NewBandPassFilter2(BandPassFilter2Config{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: centerFreq,
		Q:               4.0,
	})
	if err != nil {
		t.Fatalf("NewBandPassFilter2: %v", err)
	}
	defer func() { _ = bpf.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 100.0, frames)
	midFreq := generateSine(t, lib, sampleRate, 1000.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 8000.0, frames)

	outLow := make([]float32, frames)
	outMid := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&outLow[0]), unsafe.Pointer(&lowFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames low: %v", err)
	}
	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&outMid[0]), unsafe.Pointer(&midFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames mid: %v", err)
	}
	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&outHigh[0]), unsafe.Pointer(&highFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames high: %v", err)
	}

	lowRMS := rms(outLow)
	midRMS := rms(outMid)
	highRMS := rms(outHigh)

	if midRMS < 0.4 {
		t.Errorf("expected center band RMS >= 0.4, got %f", midRMS)
	}
	if lowRMS > midRMS*0.3 {
		t.Errorf("expected low frequency to be attenuated relative to center (%f vs %f)", lowRMS, midRMS)
	}
	if highRMS > midRMS*0.3 {
		t.Errorf("expected high frequency to be attenuated relative to center (%f vs %f)", highRMS, midRMS)
	}
}

func TestBPFNthOrderFrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		centerFreq = 1000.0
		frames     = 2048
	)

	// BPF order must be an even number
	bpf, err := lib.NewBandPassFilter(BandPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      sampleRate,
		CutoffFrequency: centerFreq,
		Order:           4,
	})
	if err != nil {
		t.Fatalf("NewBandPassFilter: %v", err)
	}
	defer func() { _ = bpf.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 100.0, frames)
	midFreq := generateSine(t, lib, sampleRate, 1000.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 8000.0, frames)

	outLow := make([]float32, frames)
	outMid := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&outLow[0]), unsafe.Pointer(&lowFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames low: %v", err)
	}
	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&outMid[0]), unsafe.Pointer(&midFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames mid: %v", err)
	}
	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&outHigh[0]), unsafe.Pointer(&highFreq[0]), frames); err != nil {
		t.Fatalf("ProcessPCMFrames high: %v", err)
	}

	lowRMS := rms(outLow)
	midRMS := rms(outMid)
	highRMS := rms(outHigh)

	if midRMS < 0.25 {
		t.Errorf("expected center band RMS >= 0.25, got %f", midRMS)
	}
	if lowRMS > midRMS*0.2 {
		t.Errorf("expected low frequency to be attenuated relative to center (%f vs %f)", lowRMS, midRMS)
	}
	if highRMS > midRMS*0.2 {
		t.Errorf("expected high frequency to be attenuated relative to center (%f vs %f)", highRMS, midRMS)
	}
}

func TestBPFLifecycleAndControls(t *testing.T) {
	lib := newNullLibrary(t)

	// Odd order should fail for BPF
	_, err := lib.NewBandPassFilter(BandPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 1000,
		Order:           3,
	})
	if err == nil {
		t.Error("expected error when initializing BPF with odd order")
	}

	bpf, err := lib.NewBandPassFilter(BandPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 1000,
		Order:           2,
	})
	if err != nil {
		t.Fatalf("NewBandPassFilter: %v", err)
	}

	_ = bpf.Latency()

	if err := bpf.Reinit(BandPassFilterConfig{
		Format:          FormatF32,
		Channels:        1,
		SampleRate:      44100,
		CutoffFrequency: 2000,
		Order:           2,
	}); err != nil {
		t.Errorf("Reinit: %v", err)
	}

	if err := bpf.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := bpf.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var dummy [4]float32
	if err := bpf.ProcessPCMFrames(unsafe.Pointer(&dummy[0]), unsafe.Pointer(&dummy[0]), 4); err == nil {
		t.Fatal("process on closed BPF should fail")
	}

	var nilBPF *BandPassFilter
	if err := nilBPF.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
	if err := nilBPF.ProcessPCMFrames(nil, nil, 0); err == nil {
		t.Error("nil ProcessPCMFrames should return error")
	}
}
