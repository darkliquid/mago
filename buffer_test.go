package mago

import (
	"testing"
	"unsafe"
)

func TestAudioBufferReadAndSeek(t *testing.T) {
	lib := newNullLibrary(t)

	samples := []float32{0, 1, 2, 3, 4, 5, 6, 7}
	buffer, err := lib.NewAudioBuffer(AudioBufferConfig{
		Format:       FormatF32,
		Channels:     1,
		SampleRate:   48_000,
		SizeInFrames: uint64(len(samples)),
		Data:         unsafe.Pointer(&samples[0]),
		DataRef:      samples,
	})
	if err != nil {
		t.Fatalf("NewAudioBuffer: %v", err)
	}
	defer func() { _ = buffer.Close() }()

	if length, err := buffer.LengthInPCMFrames(); err != nil || length != uint64(len(samples)) {
		t.Fatalf("length = %d (%v), want %d", length, err, len(samples))
	}

	out := make([]float32, 4)
	read, err := buffer.ReadPCMFrames(unsafe.Pointer(&out[0]), uint64(len(out)), false)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != uint64(len(out)) {
		t.Fatalf("read %d frames, want %d", read, len(out))
	}
	if out[0] != samples[0] || out[3] != samples[3] {
		t.Fatalf("read %v, want the first four source samples", out)
	}

	if err := buffer.SeekToPCMFrame(0); err != nil {
		t.Fatalf("SeekToPCMFrame: %v", err)
	}
	if cursor, err := buffer.CursorInPCMFrames(); err != nil || cursor != 0 {
		t.Fatalf("cursor = %d (%v), want 0", cursor, err)
	}
}

func TestAudioBufferRefDoesNotOwnData(t *testing.T) {
	lib := newNullLibrary(t)

	samples := []float32{1, 2, 3, 4}
	ref, err := lib.NewAudioBufferRef(FormatF32, 1, unsafe.Pointer(&samples[0]), uint64(len(samples)), samples)
	if err != nil {
		t.Fatalf("NewAudioBufferRef: %v", err)
	}
	defer func() { _ = ref.Close() }()

	if ref.AtEnd() {
		t.Fatal("a fresh reference should not report end-of-buffer")
	}

	out := make([]float32, 2)
	read, err := ref.ReadPCMFrames(unsafe.Pointer(&out[0]), 2, false)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != 2 || out[0] != 1 {
		t.Fatalf("read %d frames %v, want 2 starting at 1", read, out)
	}
}
