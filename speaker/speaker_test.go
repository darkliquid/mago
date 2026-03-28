package speaker

import (
	"sync/atomic"
	"testing"

	"github.com/gopxl/beep"
)

func TestStreamToFloat32ClampsAndPads(t *testing.T) {
	streamer := beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		if len(samples) < 2 {
			t.Fatalf("requested %d samples, want at least 2", len(samples))
		}
		samples[0] = [2]float64{-1.5, 0.25}
		samples[1] = [2]float64{0.5, 1.5}
		return 2, false
	})

	out := make([]float32, 6)
	var scratch [][2]float64
	streamToFloat32(streamer, &scratch, out)

	want := []float32{-1, 0.25, 0.5, 1, 0, 0}
	for i, got := range out {
		if got != want[i] {
			t.Fatalf("out[%d] = %v, want %v", i, got, want[i])
		}
	}
}

func TestDrainSignalStreamerSignalsOnce(t *testing.T) {
	var calls atomic.Int32
	streamer := &drainSignalStreamer{
		Streamer: beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
			return 0, false
		}),
		done: func() {
			calls.Add(1)
		},
	}

	for range 3 {
		if _, ok := streamer.Stream(make([][2]float64, 2)); ok {
			t.Fatal("ok = true, want false")
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("done calls = %d, want 1", got)
	}
}
