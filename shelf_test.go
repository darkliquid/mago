package mago

import (
	"testing"
)

func TestNotchFilterFrequencyAttenuation(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		notchFreq  = 1000.0
		frames     = 2048
	)

	nf, err := lib.NewNotchFilter(NotchFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Frequency:  notchFreq,
		Q:          5.0,
	})
	if err != nil {
		t.Fatalf("NewNotchFilter: %v", err)
	}
	defer func() { _ = nf.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 200.0, frames)
	midFreq := generateSine(t, lib, sampleRate, 1000.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 5000.0, frames)

	outLow := make([]float32, frames)
	outMid := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := nf.Process(outLow, lowFreq); err != nil {
		t.Fatalf("Process low: %v", err)
	}
	if err := nf.Process(outMid, midFreq); err != nil {
		t.Fatalf("Process mid: %v", err)
	}
	if err := nf.Process(outHigh, highFreq); err != nil {
		t.Fatalf("Process high: %v", err)
	}

	lowRMS := rms(outLow)
	midRMS := rms(outMid)
	highRMS := rms(outHigh)

	if lowRMS < 0.4 {
		t.Errorf("expected passband low RMS >= 0.4, got %f", lowRMS)
	}
	if highRMS < 0.4 {
		t.Errorf("expected passband high RMS >= 0.4, got %f", highRMS)
	}
	if midRMS > lowRMS*0.2 {
		t.Errorf("expected notch frequency to be heavily attenuated (< %f), got %f", lowRMS*0.2, midRMS)
	}
}

func TestPeakFilterGain(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		centerFreq = 1000.0
		frames     = 2048
	)

	// Test +6 dB boost
	pfBoost, err := lib.NewPeakFilter(PeakFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Frequency:  centerFreq,
		GainDB:     6.0,
		Q:          2.0,
	})
	if err != nil {
		t.Fatalf("NewPeakFilter boost: %v", err)
	}
	defer func() { _ = pfBoost.Close() }()

	midFreq := generateSine(t, lib, sampleRate, 1000.0, frames)
	inputRMS := rms(midFreq)

	outBoost := make([]float32, frames)
	if err := pfBoost.Process(outBoost, midFreq); err != nil {
		t.Fatalf("Process boost: %v", err)
	}
	boostRMS := rms(outBoost)

	if boostRMS <= inputRMS*1.3 {
		t.Errorf("expected boosted RMS to be significantly higher than input (%f vs %f)", boostRMS, inputRMS)
	}

	// Test -12 dB cut
	pfCut, err := lib.NewPeakFilter(PeakFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Frequency:  centerFreq,
		GainDB:     -12.0,
		Q:          2.0,
	})
	if err != nil {
		t.Fatalf("NewPeakFilter cut: %v", err)
	}
	defer func() { _ = pfCut.Close() }()

	outCut := make([]float32, frames)
	if err := pfCut.Process(outCut, midFreq); err != nil {
		t.Fatalf("Process cut: %v", err)
	}
	cutRMS := rms(outCut)

	if cutRMS >= inputRMS*0.6 {
		t.Errorf("expected cut RMS to be significantly lower than input (%f vs %f)", cutRMS, inputRMS)
	}
}

func TestLowShelfFilterGain(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		shelfFreq  = 500.0
		frames     = 2048
	)

	ls, err := lib.NewLowShelfFilter(LowShelfFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Frequency:  shelfFreq,
		GainDB:     6.0,
		ShelfSlope: 1.0,
	})
	if err != nil {
		t.Fatalf("NewLowShelfFilter: %v", err)
	}
	defer func() { _ = ls.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 100.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 5000.0, frames)

	inputLowRMS := rms(lowFreq)
	inputHighRMS := rms(highFreq)

	outLow := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := ls.Process(outLow, lowFreq); err != nil {
		t.Fatalf("Process low: %v", err)
	}
	if err := ls.Process(outHigh, highFreq); err != nil {
		t.Fatalf("Process high: %v", err)
	}

	lowRMS := rms(outLow)
	highRMS := rms(outHigh)

	if lowRMS <= inputLowRMS*1.3 {
		t.Errorf("expected low shelf to boost low frequency (%f vs input %f)", lowRMS, inputLowRMS)
	}
	// High frequency above shelf should remain close to input
	if highRMS > inputHighRMS*1.2 || highRMS < inputHighRMS*0.8 {
		t.Errorf("expected high frequency to remain near unity gain (%f vs input %f)", highRMS, inputHighRMS)
	}
}

func TestHighShelfFilterGain(t *testing.T) {
	lib := newNullLibrary(t)
	const (
		sampleRate = 48000
		shelfFreq  = 2000.0
		frames     = 2048
	)

	hs, err := lib.NewHighShelfFilter(HighShelfFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: sampleRate,
		Frequency:  shelfFreq,
		GainDB:     6.0,
		ShelfSlope: 1.0,
	})
	if err != nil {
		t.Fatalf("NewHighShelfFilter: %v", err)
	}
	defer func() { _ = hs.Close() }()

	lowFreq := generateSine(t, lib, sampleRate, 200.0, frames)
	highFreq := generateSine(t, lib, sampleRate, 8000.0, frames)

	inputLowRMS := rms(lowFreq)
	inputHighRMS := rms(highFreq)

	outLow := make([]float32, frames)
	outHigh := make([]float32, frames)

	if err := hs.Process(outLow, lowFreq); err != nil {
		t.Fatalf("Process low: %v", err)
	}
	if err := hs.Process(outHigh, highFreq); err != nil {
		t.Fatalf("Process high: %v", err)
	}

	lowRMS := rms(outLow)
	highRMS := rms(outHigh)

	if highRMS <= inputHighRMS*1.3 {
		t.Errorf("expected high shelf to boost high frequency (%f vs input %f)", highRMS, inputHighRMS)
	}
	// Low frequency below shelf should remain close to input
	if lowRMS > inputLowRMS*1.2 || lowRMS < inputLowRMS*0.8 {
		t.Errorf("expected low frequency to remain near unity gain (%f vs input %f)", lowRMS, inputLowRMS)
	}
}

func TestShelfFiltersLifecycleAndControls(t *testing.T) {
	lib := newNullLibrary(t)

	// Notch filter
	nf, err := lib.NewNotchFilter(NotchFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  1000,
		Q:          1,
	})
	if err != nil {
		t.Fatalf("NewNotchFilter: %v", err)
	}
	_ = nf.Latency()
	if err := nf.Reinit(NotchFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  2000,
		Q:          1,
	}); err != nil {
		t.Errorf("NotchFilter.Reinit: %v", err)
	}
	if err := nf.Close(); err != nil {
		t.Errorf("NotchFilter.Close: %v", err)
	}
	if err := nf.Close(); err != nil {
		t.Errorf("NotchFilter double Close: %v", err)
	}

	// Peak filter
	pf, err := lib.NewPeakFilter(PeakFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  1000,
		GainDB:     3,
		Q:          1,
	})
	if err != nil {
		t.Fatalf("NewPeakFilter: %v", err)
	}
	_ = pf.Latency()
	if err := pf.Reinit(PeakFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  2000,
		GainDB:     -3,
		Q:          1,
	}); err != nil {
		t.Errorf("PeakFilter.Reinit: %v", err)
	}
	if err := pf.Close(); err != nil {
		t.Errorf("PeakFilter.Close: %v", err)
	}

	// Low shelf filter
	ls, err := lib.NewLowShelfFilter(LowShelfFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  500,
		GainDB:     3,
		ShelfSlope: 1,
	})
	if err != nil {
		t.Fatalf("NewLowShelfFilter: %v", err)
	}
	_ = ls.Latency()
	if err := ls.Reinit(LowShelfFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  600,
		GainDB:     -3,
		ShelfSlope: 1,
	}); err != nil {
		t.Errorf("LowShelfFilter.Reinit: %v", err)
	}
	if err := ls.Close(); err != nil {
		t.Errorf("LowShelfFilter.Close: %v", err)
	}

	// High shelf filter
	hs, err := lib.NewHighShelfFilter(HighShelfFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  2000,
		GainDB:     3,
		ShelfSlope: 1,
	})
	if err != nil {
		t.Fatalf("NewHighShelfFilter: %v", err)
	}
	_ = hs.Latency()
	if err := hs.Reinit(HighShelfFilterConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 44100,
		Frequency:  3000,
		GainDB:     -3,
		ShelfSlope: 1,
	}); err != nil {
		t.Errorf("HighShelfFilter.Reinit: %v", err)
	}
	if err := hs.Close(); err != nil {
		t.Errorf("HighShelfFilter.Close: %v", err)
	}

	// Nil receiver checks
	var nilNF *NotchFilter
	if err := nilNF.Close(); err != nil {
		t.Errorf("nilNF.Close: %v", err)
	}
	if err := nilNF.Process(nil, nil); err == nil {
		t.Error("nilNF.Process expected error")
	}

	var nilPF *PeakFilter
	if err := nilPF.Close(); err != nil {
		t.Errorf("nilPF.Close: %v", err)
	}
	if err := nilPF.Process(nil, nil); err == nil {
		t.Error("nilPF.Process expected error")
	}

	var nilLS *LowShelfFilter
	if err := nilLS.Close(); err != nil {
		t.Errorf("nilLS.Close: %v", err)
	}
	if err := nilLS.Process(nil, nil); err == nil {
		t.Error("nilLS.Process expected error")
	}

	var nilHS *HighShelfFilter
	if err := nilHS.Close(); err != nil {
		t.Errorf("nilHS.Close: %v", err)
	}
	if err := nilHS.Process(nil, nil); err == nil {
		t.Error("nilHS.Process expected error")
	}
}
