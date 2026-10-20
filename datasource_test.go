package mago

import (
	"encoding/binary"
	"errors"
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

func TestExistingSourcesImplementDataSource(t *testing.T) {
	lib := newNullLibrary(t)

	waveform, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48_000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = waveform.Close() }()

	var fromWaveform DataSource = waveform
	if _, _, rate, err := fromWaveform.DataFormat(); err != nil || rate != 48_000 {
		t.Fatalf("waveform DataFormat rate = %d (%v), want 48000", rate, err)
	}
	if read, err := fromWaveform.ReadPCMFrames(make([]byte, 4*16)); err != nil || read == 0 {
		t.Fatalf("waveform ReadPCMFrames = %d (%v), want frames", read, err)
	}

	samples := []float32{0, 0.5, 1, -0.5}
	buffer, err := lib.NewAudioBuffer(AudioBufferConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48_000,
		DataF32:    samples,
	})
	if err != nil {
		t.Fatalf("NewAudioBuffer: %v", err)
	}
	defer func() { _ = buffer.Close() }()

	var fromBuffer DataSource = buffer
	out := make([]byte, 4*len(samples))
	read, err := fromBuffer.ReadPCMFrames(out)
	if err != nil {
		t.Fatalf("audio buffer ReadPCMFrames: %v", err)
	}
	if read != uint64(len(samples)) {
		t.Fatalf("audio buffer read %d frames, want %d", read, len(samples))
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(out[4:8])); got != samples[1] {
		t.Fatalf("second byte-encoded frame = %v, want %v", got, samples[1])
	}

	decoder, err := lib.NewDecoderMemory(decoderTestWAV(t, samples, 1, 48_000), DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderMemory: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	var fromDecoder DataSource = decoder
	if read, err := fromDecoder.ReadPCMFrames(make([]byte, 4*len(samples))); err != nil || read != uint64(len(samples)) {
		t.Fatalf("decoder ReadPCMFrames = %d (%v), want %d", read, err, len(samples))
	}

	noise, err := lib.NewNoise(NoiseConfig{Format: FormatF32, Channels: 1, Type: NoiseTypeWhite, Seed: 1, Amplitude: 0.5})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}
	defer func() { _ = noise.Close() }()

	var fromNoise DataSource = noise
	if read, err := fromNoise.ReadPCMFrames(make([]byte, 4*16)); err != nil || read == 0 {
		t.Fatalf("noise ReadPCMFrames = %d (%v), want frames", read, err)
	}
}

func TestGeneratorOperationsReportNotSupported(t *testing.T) {
	lib := newNullLibrary(t)

	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48_000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = wf.Close() }()

	if _, err := wf.CursorInPCMFrames(); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("waveform cursor error = %v, want ErrNotSupported", err)
	}
	if _, err := wf.LengthInPCMFrames(); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("waveform length error = %v, want ErrNotSupported", err)
	}

	noise, err := lib.NewNoise(NoiseConfig{Format: FormatF32, Channels: 1, Type: NoiseTypeWhite, Seed: 1, Amplitude: 0.5})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}
	defer func() { _ = noise.Close() }()

	if _, err := noise.CursorInPCMFrames(); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("noise cursor error = %v, want ErrNotSupported", err)
	}
	if _, err := noise.LengthInPCMFrames(); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("noise length error = %v, want ErrNotSupported", err)
	}
	if err := noise.SeekToPCMFrame(0); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("noise seek error = %v, want ErrNotSupported", err)
	}

	// A genuine failure (nil receiver) must not be mistaken for an unsupported
	// operation.
	if _, err := (*Waveform)(nil).CursorInPCMFrames(); errors.Is(err, ErrNotSupported) {
		t.Fatalf("nil waveform error = %v, should not match ErrNotSupported", err)
	}
}

func TestCustomDataSourceChain(t *testing.T) {
	lib := newNullLibrary(t)

	a, err := lib.NewCustomDataSource(&rampSource{frames: []float32{0.1, 0.2, 0.3, 0.4}})
	if err != nil {
		t.Fatalf("NewCustomDataSource a: %v", err)
	}
	defer func() { _ = a.Close() }()

	b, err := lib.NewCustomDataSource(&rampSource{frames: []float32{0.5, 0.6}})
	if err != nil {
		t.Fatalf("NewCustomDataSource b: %v", err)
	}
	defer func() { _ = b.Close() }()

	if err := a.SetNext(b); err != nil {
		t.Fatalf("SetNext: %v", err)
	}
	if a.GetNext() != b {
		t.Fatal("GetNext did not return the chained source")
	}

	out := make([]byte, 6*4)
	read, err := a.ReadPCMFrames(out)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != 6 {
		t.Fatalf("read %d frames, want 6", read)
	}

	want := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6}
	for i, w := range want {
		v := math.Float32frombits(binary.LittleEndian.Uint32(out[i*4 : i*4+4]))
		if math.Abs(float64(v-w)) > 1e-4 {
			t.Fatalf("frame %d = %v, want %v", i, v, w)
		}
	}
}

func TestCustomDataSourceRangeAndLoopPoint(t *testing.T) {
	lib := newNullLibrary(t)

	src, err := lib.NewCustomDataSource(&rampSource{frames: make([]float32, 8)})
	if err != nil {
		t.Fatalf("NewCustomDataSource: %v", err)
	}
	defer func() { _ = src.Close() }()

	if err := src.SetRangeInPCMFrames(2, 6); err != nil {
		t.Fatalf("SetRangeInPCMFrames: %v", err)
	}
	beg, end, err := src.RangeInPCMFrames()
	if err != nil || beg != 2 || end != 6 {
		t.Fatalf("range = (%d,%d) (%v), want (2,6)", beg, end, err)
	}

	// The loop point end is clamped to the range length (range 2..6 has length 4).
	if err := src.SetLoopPointInPCMFrames(1, 4); err != nil {
		t.Fatalf("SetLoopPointInPCMFrames: %v", err)
	}
	lbeg, lend, err := src.LoopPointInPCMFrames()
	if err != nil || lbeg != 1 || lend != 4 {
		t.Fatalf("loop point = (%d,%d) (%v), want (1,4)", lbeg, lend, err)
	}
}

func TestCustomDataSourceSecondsAndForwardSeek(t *testing.T) {
	lib := newNullLibrary(t)

	const frames = 8
	const rate = 48_000
	src, err := lib.NewCustomDataSource(&rampSource{frames: make([]float32, frames)})
	if err != nil {
		t.Fatalf("NewCustomDataSource: %v", err)
	}
	defer func() { _ = src.Close() }()

	length, err := src.LengthInSeconds()
	if err != nil {
		t.Fatalf("LengthInSeconds: %v", err)
	}
	if want := float32(frames) / rate; math.Abs(float64(length-want)) > 1e-4 {
		t.Fatalf("length seconds = %v, want ~%v", length, want)
	}

	skipped, err := src.SeekPCMFrames(3)
	if err != nil {
		t.Fatalf("SeekPCMFrames: %v", err)
	}
	if skipped != 3 {
		t.Fatalf("skipped %d frames, want 3", skipped)
	}
	cursor, err := src.CursorInSeconds()
	if err != nil {
		t.Fatalf("CursorInSeconds: %v", err)
	}
	if want := float32(3) / rate; math.Abs(float64(cursor-want)) > 1e-4 {
		t.Fatalf("cursor seconds = %v, want ~%v", cursor, want)
	}
}
