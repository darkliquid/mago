package audio

import (
	"bytes"
	"encoding/binary"
	"math"
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

func TestEngineLoadFallsBackToDecoder(t *testing.T) {
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
	defer func() { _ = engine.Close() }()

	samples := make([]float32, 512)
	for i := range samples {
		samples[i] = 0.25
	}
	wav := extensibleTestWAV(t, samples, 1, 48000)

	// WAVE_FORMAT_EXTENSIBLE is valid WAV that miniaudio (via dr_wav) accepts but
	// the pure-Go parser does not understand, so it exercises the fallback.
	if _, err := decodeWAV(bytes.NewReader(wav)); err == nil {
		t.Fatal("expected the pure-Go WAV parser to reject an extensible fmt chunk")
	}

	clip, err := engine.Load(bytes.NewReader(wav))
	if err != nil {
		t.Fatalf("Load fallback: %v", err)
	}
	if clip.Duration() == 0 {
		t.Fatal("expected a non-zero clip duration from the decoder fallback")
	}
}

// extensibleTestWAV builds a 16-bit PCM WAV that describes its format with a
// WAVE_FORMAT_EXTENSIBLE fmt chunk, which miniaudio accepts but decodeWAV does
// not.
func extensibleTestWAV(t *testing.T, samples []float32, channels int, sampleRate int) []byte {
	t.Helper()

	var pcm bytes.Buffer
	for _, sample := range samples {
		value := int16(math.Max(-32768, math.Min(32767, float64(sample)*32767)))
		for ch := 0; ch < channels; ch++ {
			if err := binary.Write(&pcm, binary.LittleEndian, value); err != nil {
				t.Fatalf("write pcm: %v", err)
			}
		}
	}

	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(4+8+40+8+pcm.Len()))
	wav.WriteString("WAVE")

	wav.WriteString("fmt ")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(40))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(0xFFFE)) // WAVE_FORMAT_EXTENSIBLE
	_ = binary.Write(&wav, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*channels*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(channels*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(16))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(22)) // cbSize
	_ = binary.Write(&wav, binary.LittleEndian, uint16(16)) // valid bits
	_ = binary.Write(&wav, binary.LittleEndian, uint32(0))  // channel mask
	wav.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0xAA, 0x00, 0x38, 0x9B, 0x71})

	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(pcm.Len()))
	wav.Write(pcm.Bytes())
	return wav.Bytes()
}
