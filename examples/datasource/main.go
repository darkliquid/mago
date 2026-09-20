// Command datasource shows how Go can supply PCM to miniaudio. It implements a
// small sine data source, registers it, and reads frames back through
// miniaudio's data source API. It needs no audio device.
package main

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

// sineSource is a Go data source producing a fixed number of mono f32 frames.
type sineSource struct {
	frames     int
	sampleRate uint32
	pos        uint64
}

func (s *sineSource) DataFormat() (mago.Format, uint32, uint32, error) {
	return mago.FormatF32, 1, s.sampleRate, nil
}

func (s *sineSource) LengthInPCMFrames() (uint64, error) { return uint64(s.frames), nil }
func (s *sineSource) CursorInPCMFrames() (uint64, error) { return s.pos, nil }
func (s *sineSource) SetLooping(bool) error              { return nil }

func (s *sineSource) SeekToPCMFrame(frameIndex uint64) error {
	if frameIndex > uint64(s.frames) {
		return fmt.Errorf("seek out of range")
	}
	s.pos = frameIndex
	return nil
}

func (s *sineSource) ReadPCMFrames(out []byte) (uint64, error) {
	available := (uint64(s.frames) - s.pos) * 4
	if uint64(len(out)) > available {
		out = out[:available]
	}

	frames := uint64(len(out)) / 4
	for i := uint64(0); i < frames; i++ {
		value := 0.5 * math.Sin(2*math.Pi*440*float64(s.pos+i)/float64(s.sampleRate))
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(float32(value)))
	}
	s.pos += frames
	return frames, nil
}

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	const (
		sampleRate = 48_000
		frames     = 4_800
	)

	source, err := lib.NewCustomDataSource(&sineSource{frames: frames, sampleRate: sampleRate})
	example.Must(err)
	defer func() { example.Must(source.Close()) }()

	format, channels, rate, err := source.DataFormat()
	example.Must(err)
	length, err := source.LengthInPCMFrames()
	example.Must(err)
	fmt.Printf("custom source: format %d, %d channel(s), %d Hz, %d frames\n", format, channels, rate, length)

	buffer := make([]byte, 4*256)
	peak := 0.0
	var total uint64
	for {
		read, err := source.ReadPCMFrames(buffer)
		example.Must(err)
		if read == 0 {
			break
		}
		total += read
		for i := uint64(0); i < read; i++ {
			value := math.Float32frombits(binary.LittleEndian.Uint32(buffer[i*4:]))
			if magnitude := math.Abs(float64(value)); magnitude > peak {
				peak = magnitude
			}
		}
	}
	fmt.Printf("               read %d frames, peak %.3f\n", total, peak)

	cursor, err := source.CursorInPCMFrames()
	example.Must(err)
	fmt.Printf("               cursor after reading: %d\n", cursor)

	example.Must(source.SeekToPCMFrame(0))
	cursor, err = source.CursorInPCMFrames()
	example.Must(err)
	fmt.Printf("               cursor after seek to 0: %d\n", cursor)
}
