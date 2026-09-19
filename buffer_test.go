package mago

import (
	"testing"
)

func TestAudioBufferReadAndSeek(t *testing.T) {
	lib := newNullLibrary(t)

	samples := []float32{0, 1, 2, 3, 4, 5, 6, 7}
	buffer, err := lib.NewAudioBuffer(AudioBufferConfig{
		Format:       FormatF32,
		Channels:     1,
		SampleRate:   48_000,
		SizeInFrames: uint64(len(samples)),
		DataF32:      samples,
	})
	if err != nil {
		t.Fatalf("NewAudioBuffer: %v", err)
	}
	defer func() { _ = buffer.Close() }()

	if length, err := buffer.LengthInPCMFrames(); err != nil || length != uint64(len(samples)) {
		t.Fatalf("length = %d (%v), want %d", length, err, len(samples))
	}

	out := make([]float32, 4)
	read, err := buffer.Read(out, false)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read != uint64(len(out)) {
		t.Fatalf("read %d frames, want %d", read, len(out))
	}
	if out[0] != samples[0] || out[3] != samples[3] {
		t.Fatalf("read %v, want the first four source samples", out)
	}

	mapped, err := buffer.MapF32()
	if err != nil {
		t.Fatalf("MapF32: %v", err)
	}
	if len(mapped) != 4 || mapped[0] != samples[4] {
		t.Fatalf("mapped %v, want remaining 4 samples starting at 4", mapped)
	}
	if err := buffer.Unmap(4); err != nil {
		t.Fatalf("Unmap: %v", err)
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
	ref, err := lib.NewAudioBufferRefF32(1, samples)
	if err != nil {
		t.Fatalf("NewAudioBufferRefF32: %v", err)
	}
	defer func() { _ = ref.Close() }()

	if ref.AtEnd() {
		t.Fatal("a fresh reference should not report end-of-buffer")
	}

	out := make([]float32, 2)
	read, err := ref.Read(out, false)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read != 2 || out[0] != 1 {
		t.Fatalf("read %d frames %v, want 2 starting at 1", read, out)
	}

	mapped, err := ref.MapF32()
	if err != nil {
		t.Fatalf("MapF32: %v", err)
	}
	if len(mapped) != 2 || mapped[0] != 3 {
		t.Fatalf("mapped %v, want remaining 2 samples starting at 3", mapped)
	}
	if err := ref.Unmap(2); err != nil {
		t.Fatalf("Unmap: %v", err)
	}
	if !ref.AtEnd() {
		t.Fatal("expected reference to be at end after unmapping remaining frames")
	}
}

func TestRingBufferWriteThenRead(t *testing.T) {
	lib := newNullLibrary(t)

	ring, err := lib.NewRingBuffer(256)
	if err != nil {
		t.Fatalf("NewRingBuffer: %v", err)
	}
	defer func() { _ = ring.Close() }()

	if ring.AvailableWrite() == 0 {
		t.Fatal("expected space to write into a fresh ring buffer")
	}

	region, err := ring.AcquireWrite(64)
	if err != nil {
		t.Fatalf("AcquireWrite: %v", err)
	}
	if len(region) == 0 {
		t.Fatal("expected a writable region")
	}
	for i := range region {
		region[i] = byte(i)
	}
	if err := ring.CommitWrite(uint(len(region))); err != nil {
		t.Fatalf("CommitWrite: %v", err)
	}

	if got := ring.AvailableRead(); got < uint32(len(region)) {
		t.Fatalf("available read = %d, want at least %d", got, len(region))
	}

	read, err := ring.AcquireRead(64)
	if err != nil {
		t.Fatalf("AcquireRead: %v", err)
	}
	if len(read) == 0 || read[0] != 0 || read[1] != 1 {
		t.Fatalf("read %v, want the written bytes", read[:4])
	}
	if err := ring.CommitRead(uint(len(read))); err != nil {
		t.Fatalf("CommitRead: %v", err)
	}
}

func TestPCMRingBufferRoundTrip(t *testing.T) {
	lib := newNullLibrary(t)

	ring, err := lib.NewPCMRingBuffer(FormatF32, 1, 64)
	if err != nil {
		t.Fatalf("NewPCMRingBuffer: %v", err)
	}
	defer func() { _ = ring.Close() }()

	if ring.Format() != FormatF32 || ring.Channels() != 1 {
		t.Fatalf("ring reports format %v channels %d", ring.Format(), ring.Channels())
	}

	region, err := ring.AcquireWrite(16)
	if err != nil {
		t.Fatalf("AcquireWrite: %v", err)
	}
	if len(region) == 0 {
		t.Fatal("expected writable frames")
	}
	for i := range region {
		region[i] = float32(i)
	}
	if err := ring.CommitWrite(uint32(len(region))); err != nil {
		t.Fatalf("CommitWrite: %v", err)
	}

	read, err := ring.AcquireRead(16)
	if err != nil {
		t.Fatalf("AcquireRead: %v", err)
	}
	if len(read) == 0 || read[0] != 0 || read[1] != 1 {
		t.Fatalf("read %v, want the written frames", read[:4])
	}
	if err := ring.CommitRead(uint32(len(read))); err != nil {
		t.Fatalf("CommitRead: %v", err)
	}
}
