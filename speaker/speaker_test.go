package speaker

import (
	"sync/atomic"
	"testing"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/internal/testlib"
	"github.com/gopxl/beep"
	"github.com/gopxl/beep/generators"
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

func TestInitWithConfigNullBackend(t *testing.T) {
	libPath := testlib.BuildRuntimeLibrary(t, "..")
	t.Setenv("MAGO_MINIAUDIO_LIB", libPath)

	if err := InitWithConfig(48000, 256, Config{
		Backends:    []mago.Backend{mago.BackendNull},
		DeviceIndex: -1,
	}); err != nil {
		t.Fatalf("init speaker with null backend: %v", err)
	}
	defer Close()

	seq := beep.Seq(generators.Silence(1), generators.Silence(1), generators.Silence(1))
	PlayAndWait(seq)
}

func TestFindDeviceIndex(t *testing.T) {
	devices := []mago.DeviceInfo{
		{Name: "Null Output", IsDefault: false},
		{Name: "Null Output", IsDefault: true},
	}
	if got := findDeviceIndex(devices, ""); got != 1 {
		t.Fatalf("default device index = %d, want 1", got)
	}
	if got := findDeviceIndex(devices, "null"); got != 0 {
		t.Fatalf("named device index = %d, want 0", got)
	}
	if got := findDeviceIndex(devices, "missing"); got != -1 {
		t.Fatalf("missing device index = %d, want -1", got)
	}
	if got := findDeviceIndex(nil, ""); got != -1 {
		t.Fatalf("empty device list index = %d, want -1", got)
	}
}

func TestDrainSignalStreamerSignalsOnce(t *testing.T) {
	var calls atomic.Int32
	streamer := &drainSignalStreamer{
		Streamer: beep.StreamerFunc(func(_ [][2]float64) (n int, ok bool) {
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
