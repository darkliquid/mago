package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func TestDecodeWAVAndClipDuration(t *testing.T) {
	wavData := mustTestWAV(t, []float32{0, 0.5, -0.5, 0}, 1, 48000)
	clip, err := decodeWAV(bytes.NewReader(wavData))
	if err != nil {
		t.Fatalf("decode wav: %v", err)
	}
	if clip.Channels() != 1 {
		t.Fatalf("channels = %d, want 1", clip.Channels())
	}
	if clip.SampleRate() != 48000 {
		t.Fatalf("sample rate = %d, want 48000", clip.SampleRate())
	}
	if got := clip.Duration(); got != durationFromFrames(4, 48000) {
		t.Fatalf("duration = %v", got)
	}
}

func TestStreamMixReverseSpeedAndFade(t *testing.T) {
	engine := &Engine{sampleRate: 4, channels: 1}
	clip := &Clip{
		samples:    []float32{0, 1, 0, -1},
		channels:   1,
		sampleRate: 4,
		frameCount: 4,
	}
	stream := &Stream{
		engine:  engine,
		clip:    clip,
		volume:  1,
		speed:   2,
		reverse: false,
		state:   streamPlaying,
	}
	stream.resetPositionLocked(clip.frameCount)
	out := make([]float32, 2)
	if !stream.mixInto(out, 2, 1, 4) {
		t.Fatal("expected stream to stay alive")
	}
	if math.Abs(float64(out[0]-0)) > 0.001 || math.Abs(float64(out[1]-0)) > 0.001 {
		t.Fatalf("unexpected resampled output: %v", out)
	}

	stream.SetReverse(true)
	if err := stream.Seek(750 * time.Millisecond); err != nil {
		t.Fatalf("seek: %v", err)
	}
	out = make([]float32, 2)
	if !stream.mixInto(out, 2, 1, 4) {
		t.Fatal("expected reverse stream to stay alive")
	}
	if out[0] >= out[1] {
		t.Fatalf("expected reverse playback to move backward through samples: %v", out)
	}

	stream.SetVolume(1)
	stream.FadeOutAndStop(250 * time.Millisecond)
	out = make([]float32, 2)
	stream.state = streamPlaying
	stream.position = 0
	stream.mixInto(out, 2, 1, 4)
	if stream.state != streamStopped {
		t.Fatalf("expected stream to stop after fade, state=%v", stream.state)
	}
}

func TestCrossfadeStartsIncomingStream(t *testing.T) {
	engine := &Engine{sampleRate: 48000, channels: 2, streams: map[*Stream]struct{}{}}
	clip := &Clip{samples: make([]float32, 8), channels: 2, sampleRate: 48000, frameCount: 4}
	from, err := engine.Play(clip, DefaultPlayOptions())
	if err != nil {
		t.Fatalf("play from: %v", err)
	}
	to, err := engine.Crossfade(from, clip, 100*time.Millisecond, DefaultPlayOptions())
	if err != nil {
		t.Fatalf("crossfade: %v", err)
	}
	if to == nil {
		t.Fatal("expected incoming stream")
	}
	if !to.fade.active {
		t.Fatal("expected incoming fade to be active")
	}
	if !from.fade.active || !from.fade.stopWhenDone {
		t.Fatal("expected outgoing fade-out")
	}
}

func mustTestWAV(t *testing.T, samples []float32, channels int, sampleRate int) []byte {
	t.Helper()
	var pcm bytes.Buffer
	for _, sample := range samples {
		value := int16(clampFloat(float64(sample), -1, 1) * 32767)
		if err := binary.Write(&pcm, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	dataSize := pcm.Len()
	blockAlign := channels * 2
	byteRate := sampleRate * blockAlign
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	mustBinaryWrite(t, &wav, uint32(36+dataSize))
	wav.WriteString("WAVE")
	wav.WriteString("fmt ")
	mustBinaryWrite(t, &wav, uint32(16))
	mustBinaryWrite(t, &wav, uint16(1))
	mustBinaryWrite(t, &wav, uint16(channels))
	mustBinaryWrite(t, &wav, uint32(sampleRate))
	mustBinaryWrite(t, &wav, uint32(byteRate))
	mustBinaryWrite(t, &wav, uint16(blockAlign))
	mustBinaryWrite(t, &wav, uint16(16))
	wav.WriteString("data")
	mustBinaryWrite(t, &wav, uint32(dataSize))
	wav.Write(pcm.Bytes())
	return wav.Bytes()
}

func mustBinaryWrite(t *testing.T, buf *bytes.Buffer, value any) {
	t.Helper()
	if err := binary.Write(buf, binary.LittleEndian, value); err != nil {
		t.Fatalf("binary write %T: %v", value, err)
	}
}
