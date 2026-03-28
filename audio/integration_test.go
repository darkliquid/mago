package audio

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/internal/buildlib"
)

func TestEnginePlayPauseResumeWithNullBackend(t *testing.T) {
	libPath := filepath.Join(t.TempDir(), buildlib.DefaultLibraryFilename("linux"))
	if err := buildlib.Build("..", libPath); err != nil {
		t.Fatalf("build shared library: %v", err)
	}

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

	time.Sleep(60 * time.Millisecond)
	pos1 := stream.Position()
	if pos1 <= 0 {
		t.Fatalf("expected playback to advance, got %v", pos1)
	}

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
	time.Sleep(40 * time.Millisecond)
	pos3 := stream.Position()
	if pos3 <= pos2 {
		t.Fatalf("expected resumed playback to advance, got %v then %v", pos2, pos3)
	}
}
