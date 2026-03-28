package audio

import (
	"bytes"
	"testing"
	"time"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/internal/testlib"
)

func TestEnginePlayPauseResumeWithNullBackend(t *testing.T) {
	libPath := testlib.BuildRuntimeLibrary(t, "..")

	engine, err := Open(Config{
		LibraryPath:        libPath,
		Backends:           []mago.Backend{mago.BackendNull},
		SampleRate:         48000,
		Channels:           1,
		PeriodSizeInFrames: 64,
		DeviceIndex:        -1,
	})
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Fatalf("close engine: %v", err)
		}
	}()

	samples := make([]float32, 48000)
	for i := range samples {
		samples[i] = 0.2
	}
	clip, err := engine.Load(bytes.NewReader(mustTestWAV(t, samples, 1, 48000)))
	if err != nil {
		t.Fatalf("load clip: %v", err)
	}

	stream, err := engine.Play(clip, PlayOptions{Loop: true, Volume: 1, Speed: 1})
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	defer stream.Close()

	waitForPositionAdvance(t, stream, 250*time.Millisecond)

	stream.Pause()
	paused := stream.Position()
	time.Sleep(40 * time.Millisecond)
	pos2 := stream.Position()
	if pos2 != paused {
		t.Fatalf("expected paused position to stay fixed, got %v then %v", paused, pos2)
	}

	if err := stream.Resume(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	pos3 := waitForPositionGreaterThan(t, stream, pos2, 250*time.Millisecond)
	if pos3 <= pos2 {
		t.Fatalf("expected resumed playback to advance, got %v then %v", pos2, pos3)
	}
}

func waitForPositionAdvance(t *testing.T, stream *Stream, timeout time.Duration) time.Duration {
	t.Helper()
	return waitForPositionGreaterThan(t, stream, 0, timeout)
}

func waitForPositionGreaterThan(t *testing.T, stream *Stream, threshold time.Duration, timeout time.Duration) time.Duration {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pos := stream.Position()
		if pos > threshold {
			return pos
		}
		time.Sleep(10 * time.Millisecond)
	}

	pos := stream.Position()
	t.Fatalf("expected playback position > %v within %v, got %v", threshold, timeout, pos)
	return 0
}
