package mago

import (
	"math"
	"testing"
)

// newDeviceLessEngine creates an engine that is mixed by hand with Read, so it
// needs no audio hardware. Cleanup is registered last-in-first-out, so sounds
// created afterwards are closed before the engine.
func newDeviceLessEngine(t *testing.T, channels uint32) *Engine {
	t.Helper()

	lib := newNullLibrary(t)
	config := DefaultEngineConfig(channels, 48000)
	config.NoDevice = true

	engine, err := lib.NewEngine(config)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	})
	return engine
}

// newSineWaveform creates a mono 440 Hz sine at the given amplitude and registers
// cleanup. Using a native Waveform rather than a Go data source keeps the engine
// reading straight from miniaudio.
func newSineWaveform(t *testing.T, lib *Library, channels uint32, amplitude float64) *Waveform {
	t.Helper()

	waveform, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   channels,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  amplitude,
		Frequency:  440,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	t.Cleanup(func() {
		if err := waveform.Close(); err != nil {
			t.Errorf("close waveform: %v", err)
		}
	})
	return waveform
}

// readEngine reads frames frames and reports the peak amplitude of each channel
// along with how many frames the graph actually produced. out is always fully
// written: miniaudio silences whatever the graph had nothing to contribute to.
func readEngine(t *testing.T, engine *Engine, frames int) ([]float64, uint64) {
	t.Helper()

	channels := int(engine.Channels())
	samples := make([]float32, frames*channels)
	read, err := engine.Read(samples)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	peaks := make([]float64, channels)
	for i, sample := range samples {
		channel := i % channels
		if magnitude := math.Abs(float64(sample)); magnitude > peaks[channel] {
			peaks[channel] = magnitude
		}
	}
	return peaks, read
}

// readEnginePeaks is readEngine for tests that expect the graph to produce every
// requested frame.
func readEnginePeaks(t *testing.T, engine *Engine, frames int) []float64 {
	t.Helper()

	peaks, read := readEngine(t, engine, frames)
	if int(read) != frames {
		t.Fatalf("read %d frames, want %d", read, frames)
	}
	return peaks
}

func peakOf(peaks []float64) float64 {
	peak := 0.0
	for _, value := range peaks {
		if value > peak {
			peak = value
		}
	}
	return peak
}

func readEnginePeak(t *testing.T, engine *Engine, frames int) float64 {
	t.Helper()
	return peakOf(readEnginePeaks(t, engine, frames))
}

func TestEngineDeviceLessReadsSilence(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)

	if got := engine.Channels(); got != 2 {
		t.Errorf("Channels = %d, want 2", got)
	}
	if got := engine.SampleRate(); got != 48000 {
		t.Errorf("SampleRate = %d, want 48000", got)
	}
	if got := engine.ListenerCount(); got != 1 {
		t.Errorf("ListenerCount = %d, want 1", got)
	}
	if got := engine.Volume(); got != 1 {
		t.Errorf("Volume = %f, want 1", got)
	}
	if got := engine.Time(); got != 0 {
		t.Errorf("Time = %d, want 0", got)
	}
	// An engine with no sounds produces nothing, so miniaudio reports zero frames
	// produced while still silencing the whole buffer.
	peaks, read := readEngine(t, engine, 256)
	if read != 0 {
		t.Errorf("empty engine produced %d frames, want 0", read)
	}
	if peak := peakOf(peaks); peak != 0 {
		t.Errorf("empty engine peak = %f, want 0", peak)
	}
	if got := engine.Time(); got != 0 {
		t.Errorf("Time after reading an empty engine = %d, want 0", got)
	}

	if err := engine.SetVolume(0.5); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if got := engine.Volume(); got != 0.5 {
		t.Errorf("Volume = %f, want 0.5", got)
	}
	if err := engine.SetGainDB(0); err != nil {
		t.Fatalf("SetGainDB: %v", err)
	}
	if got := engine.GainDB(); got != 0 {
		t.Errorf("GainDB = %f, want 0", got)
	}

	if err := engine.SetTime(1024); err != nil {
		t.Fatalf("SetTime: %v", err)
	}
	if got := engine.Time(); got != 1024 {
		t.Errorf("Time after SetTime = %d, want 1024", got)
	}

	// The engine's node graph is borrowed: routing works, closing does not.
	graph := engine.NodeGraph()
	if graph == nil {
		t.Fatal("NodeGraph returned nil")
	}
	if got := graph.Channels(); got != 2 {
		t.Errorf("graph Channels = %d, want 2", got)
	}
	if graph.Channels() != engine.Channels() {
		t.Error("engine node graph has different channels from the engine")
	}
	if err := graph.Close(); err == nil {
		t.Error("closing the engine's node graph was allowed")
	}
	if engine.Endpoint() == nil {
		t.Error("engine Endpoint returned nil")
	}

	if err := engine.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := engine.Start(); err == nil {
		t.Error("Start on a closed engine was allowed")
	}
	if engine.Channels() != 0 {
		t.Error("Channels on a closed engine was non-zero")
	}
}

func TestEngineRejectsTooManyListeners(t *testing.T) {
	lib := newNullLibrary(t)

	config := DefaultEngineConfig(2, 48000)
	config.NoDevice = true
	config.ListenerCount = maxEngineListeners + 1

	if _, err := lib.NewEngine(config); err == nil {
		t.Error("NewEngine accepted more listeners than miniaudio allows")
	}
}

func TestEngineListenerRoundTrip(t *testing.T) {
	engine := newDeviceLessEngine(t, 2)
	listener := engine.Listener(0)

	if got := listener.Index(); got != 0 {
		t.Errorf("listener index = %d, want 0", got)
	}
	if got := listener.Engine(); got != engine {
		t.Errorf("listener engine = %p, want %p", got, engine)
	}

	position := Vec3{X: 1, Y: 2, Z: 3}
	if err := listener.SetPosition(position); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	if got := listener.Position(); got != position {
		t.Errorf("Position = %+v, want %+v", got, position)
	}

	direction := Vec3{X: 0, Y: 0, Z: -1}
	if err := listener.SetDirection(direction); err != nil {
		t.Fatalf("SetDirection: %v", err)
	}
	if got := listener.Direction(); got != direction {
		t.Errorf("Direction = %+v, want %+v", got, direction)
	}

	velocity := Vec3{X: -1, Y: 0, Z: 0}
	if err := listener.SetVelocity(velocity); err != nil {
		t.Fatalf("SetVelocity: %v", err)
	}
	if got := listener.Velocity(); got != velocity {
		t.Errorf("Velocity = %+v, want %+v", got, velocity)
	}

	worldUp := Vec3{X: 0, Y: 1, Z: 0}
	if err := listener.SetWorldUp(worldUp); err != nil {
		t.Fatalf("SetWorldUp: %v", err)
	}
	if got := listener.WorldUp(); got != worldUp {
		t.Errorf("WorldUp = %+v, want %+v", got, worldUp)
	}

	if err := listener.SetCone(0.5, 1.0, 0.25); err != nil {
		t.Fatalf("SetCone: %v", err)
	}
	inner, outer, outerGain := listener.Cone()
	if math.Abs(float64(inner)-0.5) > 1e-6 || math.Abs(float64(outer)-1.0) > 1e-6 || math.Abs(float64(outerGain)-0.25) > 1e-6 {
		t.Errorf("Cone = (%f, %f, %f), want (0.5, 1, 0.25)", inner, outer, outerGain)
	}

	if !listener.Enabled() {
		t.Error("listener 0 is disabled by default")
	}
	if err := listener.SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if listener.Enabled() {
		t.Error("Enabled = true after SetEnabled(false)")
	}

	if got := engine.ClosestListener(Vec3{X: 1, Y: 2, Z: 3}); got != 0 {
		t.Errorf("ClosestListener = %d, want 0", got)
	}
}

func TestEngineNullDeviceStartStop(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	config := DefaultEngineConfig(0, 0)
	config.Context = ctx

	engine, err := lib.NewEngine(config)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Errorf("close engine: %v", err)
		}
	})

	// The null device supplies the mixing format when the config leaves it unset.
	if got := engine.Channels(); got == 0 {
		t.Error("device-backed engine has no channels")
	}
	if got := engine.SampleRate(); got == 0 {
		t.Error("device-backed engine has no sample rate")
	}

	if err := engine.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := engine.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
}
