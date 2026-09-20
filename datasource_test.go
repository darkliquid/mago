package mago

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

// rampSource is a tiny Go data source: a fixed number of mono f32 frames.
type rampSource struct {
	frames []float32
	pos    uint64
}

func (s *rampSource) DataFormat() (Format, uint32, uint32, error) { return FormatF32, 1, 48_000, nil }
func (s *rampSource) LengthInPCMFrames() (uint64, error)          { return uint64(len(s.frames)), nil }
func (s *rampSource) CursorInPCMFrames() (uint64, error)          { return s.pos, nil }
func (s *rampSource) SetLooping(bool) error                       { return nil }

func (s *rampSource) SeekToPCMFrame(frameIndex uint64) error {
	if frameIndex > uint64(len(s.frames)) {
		return fmt.Errorf("seek out of range")
	}
	s.pos = frameIndex
	return nil
}

func (s *rampSource) ReadPCMFrames(out []byte) (uint64, error) {
	available := (uint64(len(s.frames)) - s.pos) * 4
	if uint64(len(out)) > available {
		out = out[:available]
	}
	frames := uint64(len(out)) / 4
	for i := uint64(0); i < frames; i++ {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(s.frames[s.pos+i]))
	}
	s.pos += frames
	return frames, nil
}

func TestCustomDataSourceRoundTrip(t *testing.T) {
	lib := newNullLibrary(t)

	ramp := &rampSource{frames: []float32{0, 1, 2, 3, 4, 5, 6, 7}}
	source, err := lib.NewCustomDataSource(ramp)
	if err != nil {
		t.Fatalf("NewCustomDataSource: %v", err)
	}
	defer func() { _ = source.Close() }()

	format, channels, sampleRate, err := source.DataFormat()
	if err != nil {
		t.Fatalf("DataFormat: %v", err)
	}
	if format != FormatF32 || channels != 1 || sampleRate != 48_000 {
		t.Fatalf("format = %v channels = %d rate = %d", format, channels, sampleRate)
	}

	length, err := source.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != uint64(len(ramp.frames)) {
		t.Fatalf("length = %d, want %d", length, len(ramp.frames))
	}

	buffer := make([]byte, 4*4)
	read, err := source.ReadPCMFrames(buffer)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != 4 {
		t.Fatalf("read %d frames, want 4", read)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(buffer[4:8])); got != 1 {
		t.Fatalf("second frame = %v, want 1", got)
	}

	if err := source.SeekToPCMFrame(0); err != nil {
		t.Fatalf("SeekToPCMFrame: %v", err)
	}
	if cursor, err := source.CursorInPCMFrames(); err != nil || cursor != 0 {
		t.Fatalf("cursor = %d (%v), want 0", cursor, err)
	}
}

func TestCustomDataSourceForwardSeek(t *testing.T) {
	lib := newNullLibrary(t)

	ramp := &rampSource{frames: []float32{0, 1, 2, 3, 4, 5, 6, 7}}
	source, err := lib.NewCustomDataSource(ramp)
	if err != nil {
		t.Fatalf("NewCustomDataSource: %v", err)
	}
	defer func() { _ = source.Close() }()

	// miniaudio uses a NULL output buffer to mean "seek forward": exercise that
	// path directly through the binding.
	var read uint64
	if result := lib.bindings.maDataSourceReadPCMFrames(source.handle, nil, 3, &read); result != Success {
		t.Fatalf("forward seek failed with result %d", result)
	}
	if read != 3 {
		t.Fatalf("skipped %d frames, want 3", read)
	}
	if ramp.pos != 3 {
		t.Fatalf("source cursor = %d, want 3", ramp.pos)
	}

	buffer := make([]byte, 4)
	if _, err := source.ReadPCMFrames(buffer); err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(buffer)); got != 3 {
		t.Fatalf("first frame after seek = %v, want 3", got)
	}
}
