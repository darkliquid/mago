package mago

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestEncoderWriterProducesWAV(t *testing.T) {
	lib := newNullLibrary(t)

	encoder, err := lib.NewEncoderWriter(DefaultEncoderConfig(1, 48_000))
	if err != nil {
		t.Fatalf("NewEncoderWriter: %v", err)
	}

	frames := make([]float32, 128)
	for i := range frames {
		frames[i] = 0.25
	}
	if written, err := encoder.WriteF32(frames); err != nil {
		t.Fatalf("WriteF32: %v", err)
	} else if written != uint64(len(frames)) {
		t.Fatalf("wrote %d frames, want %d", written, len(frames))
	}
	if err := encoder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	defer func() { _ = encoder.Close() }()

	data := encoder.Bytes()
	if len(data) < 44 {
		t.Fatalf("encoded only %d bytes, want a WAV header plus data", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("output is not a WAV stream: % x", data[:12])
	}
	if riffSize := binary.LittleEndian.Uint32(data[4:8]); int(riffSize) != len(data)-8 {
		t.Fatalf("RIFF size = %d, want %d", riffSize, len(data)-8)
	}
}

func TestEncoderFileWritesWAV(t *testing.T) {
	lib := newNullLibrary(t)

	path := t.TempDir() + "/out.wav"
	encoder, err := lib.NewEncoderFile(path, DefaultEncoderConfig(1, 48_000))
	if err != nil {
		t.Fatalf("NewEncoderFile: %v", err)
	}

	frames := make([]float32, 64)
	if _, err := encoder.WriteF32(frames); err != nil {
		t.Fatalf("WriteF32: %v", err)
	}
	if err := encoder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encoded file: %v", err)
	}
	if len(data) < 4 || string(data[0:4]) != "RIFF" {
		t.Fatalf("file is not a WAV stream: % x", data[:4])
	}

	// The output must round-trip through the decoder from Phase 7.
	decoder, err := lib.NewDecoderFile(path, DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderFile: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	length, err := decoder.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != uint64(len(frames)) {
		t.Fatalf("decoded %d frames, want %d", length, len(frames))
	}
}

func TestEncoderWriterRoundTripsThroughDecoder(t *testing.T) {
	lib := newNullLibrary(t)

	encoder, err := lib.NewEncoderWriter(DefaultEncoderConfig(1, 48_000))
	if err != nil {
		t.Fatalf("NewEncoderWriter: %v", err)
	}

	frames := make([]float32, 32)
	for i := range frames {
		frames[i] = float32(i) / 32
	}
	if _, err := encoder.WriteF32(frames); err != nil {
		t.Fatalf("WriteF32: %v", err)
	}
	if err := encoder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	defer func() { _ = encoder.Close() }()

	decoder, err := lib.NewDecoderMemory(encoder.Bytes(), DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderMemory: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	out := make([]float32, len(frames))
	read, err := decoder.ReadF32(out)
	if err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if read != uint64(len(frames)) {
		t.Fatalf("decoded %d frames, want %d", read, len(frames))
	}
	if diff := out[1] - frames[1]; diff > 0.001 || diff < -0.001 {
		t.Fatalf("round-tripped sample %v, want %v", out[1], frames[1])
	}
}

func TestEncoderRejectsUnsupportedConfig(t *testing.T) {
	lib := newNullLibrary(t)

	if _, err := lib.NewEncoderWriter(EncoderConfig{EncodingFormat: EncodingFormatWAV}); err == nil {
		t.Fatal("expected a channel-count error")
	}
	if _, err := lib.NewEncoderWriter(EncoderConfig{
		EncodingFormat: EncodingFormatVorbis,
		Format:         FormatF32,
		Channels:       1,
		SampleRate:     48_000,
	}); err == nil {
		t.Fatal("expected an unsupported encoding format error")
	}
}
