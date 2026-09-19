package mago

import (
	"math"
	"testing"
)

func TestDelayImpulseResponse(t *testing.T) {
	lib := newNullLibrary(t)

	const (
		delayFrames = 4
		totalFrames = 16
	)

	delay, err := lib.NewDelay(DelayConfig{
		Channels:      1,
		SampleRate:    44100,
		DelayInFrames: delayFrames,
		DelayStart:    true,
		Wet:           1.0,
		Dry:           1.0,
		Decay:         0.5,
	})
	if err != nil {
		t.Fatalf("NewDelay: %v", err)
	}
	defer func() { _ = delay.Close() }()

	in := make([]float32, totalFrames)
	in[0] = 1.0 // Impulse at t = 0

	out := make([]float32, totalFrames)
	if err := delay.Process(out, in); err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Frames 0..3 should be 0
	for i := 0; i < delayFrames; i++ {
		if math.Abs(float64(out[i])) > 1e-4 {
			t.Errorf("frame %d: expected 0, got %f", i, out[i])
		}
	}

	// Frame 4 should be 1.0 (first echo)
	if math.Abs(float64(out[4]-1.0)) > 1e-4 {
		t.Errorf("frame 4: expected 1.0, got %f", out[4])
	}

	// Frame 8 should be 0.5 (second echo with decay 0.5)
	if math.Abs(float64(out[8]-0.5)) > 1e-4 {
		t.Errorf("frame 8: expected 0.5, got %f", out[8])
	}

	// Frame 12 should be 0.25 (third echo with decay 0.5^2)
	if math.Abs(float64(out[12]-0.25)) > 1e-4 {
		t.Errorf("frame 12: expected 0.25, got %f", out[12])
	}
}

func TestDelayGettersAndSetters(t *testing.T) {
	lib := newNullLibrary(t)

	cfg := DefaultDelayConfig(2, 48000, 100, 0.4)
	delay, err := lib.NewDelay(cfg)
	if err != nil {
		t.Fatalf("NewDelay: %v", err)
	}
	defer func() { _ = delay.Close() }()

	if math.Abs(float64(delay.Wet()-1.0)) > 1e-4 {
		t.Errorf("expected initial wet 1.0, got %f", delay.Wet())
	}
	if math.Abs(float64(delay.Dry()-1.0)) > 1e-4 {
		t.Errorf("expected initial dry 1.0, got %f", delay.Dry())
	}
	if math.Abs(float64(delay.Decay()-0.4)) > 1e-4 {
		t.Errorf("expected initial decay 0.4, got %f", delay.Decay())
	}

	if err := delay.SetWet(0.75); err != nil {
		t.Errorf("SetWet: %v", err)
	}
	if math.Abs(float64(delay.Wet()-0.75)) > 1e-4 {
		t.Errorf("expected wet 0.75, got %f", delay.Wet())
	}

	if err := delay.SetDry(0.25); err != nil {
		t.Errorf("SetDry: %v", err)
	}
	if math.Abs(float64(delay.Dry()-0.25)) > 1e-4 {
		t.Errorf("expected dry 0.25, got %f", delay.Dry())
	}

	if err := delay.SetDecay(0.85); err != nil {
		t.Errorf("SetDecay: %v", err)
	}
	if math.Abs(float64(delay.Decay()-0.85)) > 1e-4 {
		t.Errorf("expected decay 0.85, got %f", delay.Decay())
	}
}

func TestDelayLifecycleAndSafety(t *testing.T) {
	lib := newNullLibrary(t)

	delay, err := lib.NewDelay(DefaultDelayConfig(1, 44100, 50, 0.2))
	if err != nil {
		t.Fatalf("NewDelay: %v", err)
	}

	if err := delay.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := delay.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var dummy [4]float32
	if err := delay.Process(dummy[:], dummy[:]); err == nil {
		t.Error("Process on closed delay should fail")
	}
	if err := delay.SetWet(0.5); err == nil {
		t.Error("SetWet on closed delay should fail")
	}
	if err := delay.SetDry(0.5); err == nil {
		t.Error("SetDry on closed delay should fail")
	}
	if err := delay.SetDecay(0.5); err == nil {
		t.Error("SetDecay on closed delay should fail")
	}

	var nilDelay *Delay
	if err := nilDelay.Close(); err != nil {
		t.Errorf("nil Close: %v", err)
	}
	if err := nilDelay.Process(nil, nil); err == nil {
		t.Error("nil Process should return error")
	}
	if err := nilDelay.SetWet(0.5); err == nil {
		t.Error("nil SetWet should return error")
	}
	if err := nilDelay.SetDry(0.5); err == nil {
		t.Error("nil SetDry should return error")
	}
	if err := nilDelay.SetDecay(0.5); err == nil {
		t.Error("nil SetDecay should return error")
	}
	if got := nilDelay.Wet(); got != 0 {
		t.Errorf("nil Wet should return 0, got %f", got)
	}
	if got := nilDelay.Dry(); got != 0 {
		t.Errorf("nil Dry should return 0, got %f", got)
	}
	if got := nilDelay.Decay(); got != 0 {
		t.Errorf("nil Decay should return 0, got %f", got)
	}
}
