package mago

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"testing"
)

func decoderTestWAV(t *testing.T, samples []float32, channels int, sampleRate int) []byte {
	t.Helper()

	var pcm bytes.Buffer
	for _, sample := range samples {
		scaled := math.Max(-32768, math.Min(32767, float64(sample)*32767))
		if err := binary.Write(&pcm, binary.LittleEndian, int16(scaled)); err != nil {
			t.Fatalf("write pcm: %v", err)
		}
		for ch := 1; ch < channels; ch++ {
			if err := binary.Write(&pcm, binary.LittleEndian, int16(scaled)); err != nil {
				t.Fatalf("write pcm: %v", err)
			}
		}
	}

	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+pcm.Len()))
	wav.WriteString("WAVE")
	wav.WriteString("fmt ")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(16))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(1))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*channels*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(channels*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(16))
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(pcm.Len()))
	wav.Write(pcm.Bytes())
	return wav.Bytes()
}

func TestDecoderReadsWAVFromMemory(t *testing.T) {
	lib := newNullLibrary(t)

	samples := make([]float32, 64)
	for i := range samples {
		samples[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/48_000))
	}
	data := decoderTestWAV(t, samples, 1, 48_000)

	decoder, err := lib.NewDecoderMemory(data, DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderMemory: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	format, channels, sampleRate, err := decoder.DataFormat()
	if err != nil {
		t.Fatalf("DataFormat: %v", err)
	}
	if format != FormatF32 || channels != 1 || sampleRate != 48_000 {
		t.Fatalf("format = %v channels = %d rate = %d", format, channels, sampleRate)
	}

	length, err := decoder.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != uint64(len(samples)) {
		t.Fatalf("length = %d, want %d", length, len(samples))
	}

	out := make([]float32, len(samples))
	read, err := decoder.ReadF32(out)
	if err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if read != uint64(len(samples)) {
		t.Fatalf("read %d frames, want %d", read, len(samples))
	}
	if math.Abs(float64(out[1]-samples[1])) > 0.001 {
		t.Fatalf("decoded sample %v, want %v", out[1], samples[1])
	}
}

func TestDecoderSeekAndCursor(t *testing.T) {
	lib := newNullLibrary(t)

	samples := make([]float32, 32)
	for i := range samples {
		samples[i] = float32(i) / 32
	}
	decoder, err := lib.NewDecoderMemory(decoderTestWAV(t, samples, 1, 48_000), DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderMemory: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	first := make([]float32, 4)
	if _, err := decoder.ReadF32(first); err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if cursor, err := decoder.CursorInPCMFrames(); err != nil || cursor != 4 {
		t.Fatalf("cursor = %d (%v), want 4", cursor, err)
	}

	if err := decoder.SeekToPCMFrame(0); err != nil {
		t.Fatalf("SeekToPCMFrame: %v", err)
	}
	if cursor, err := decoder.CursorInPCMFrames(); err != nil || cursor != 0 {
		t.Fatalf("cursor after seek = %d (%v), want 0", cursor, err)
	}

	again := make([]float32, 4)
	if _, err := decoder.ReadF32(again); err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if math.Abs(float64(again[1]-first[1])) > 0.001 {
		t.Fatalf("re-read sample %v, want %v", again[1], first[1])
	}
}

func TestDecoderReadsWAVFromFile(t *testing.T) {
	lib := newNullLibrary(t)

	samples := make([]float32, 16)
	for i := range samples {
		samples[i] = float32(i) / 16
	}

	path := t.TempDir() + "/tone.wav"
	if err := os.WriteFile(path, decoderTestWAV(t, samples, 1, 48_000), 0o600); err != nil {
		t.Fatalf("write test wav: %v", err)
	}

	decoder, err := lib.NewDecoderFile(path, DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderFile: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	out := make([]float32, len(samples))
	read, err := decoder.ReadF32(out)
	if err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if read != uint64(len(samples)) {
		t.Fatalf("read %d frames, want %d", read, len(samples))
	}
}
