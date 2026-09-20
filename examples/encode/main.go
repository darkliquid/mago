// Command encode shows miniaudio's encoder: it synthesises a tone, encodes it to
// WAV in memory and to a temporary file, then decodes both back to confirm the
// frames survived. It needs no audio device.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	const (
		sampleRate = 48_000
		frames     = 4_800
	)

	tone := make([]float32, frames)
	for i := range tone {
		tone[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/sampleRate))
	}

	config := mago.DefaultEncoderConfig(1, sampleRate)

	encoder, err := lib.NewEncoderWriter(config)
	example.Must(err)
	written, err := encoder.WriteF32(tone)
	example.Must(err)
	example.Must(encoder.Finish())
	encoded := encoder.Bytes()
	example.Must(encoder.Close())

	riffSize := int(binary.LittleEndian.Uint32(encoded[4:8]))
	fmt.Printf("memory encoder: wrote %d frames, %d bytes (RIFF size %d)\n", written, len(encoded), riffSize)

	memoryDecoder, err := lib.NewDecoderMemory(encoded, mago.DefaultDecoderConfig())
	example.Must(err)
	reportDecoded(memoryDecoder, "memory")
	example.Must(memoryDecoder.Close())

	path := filepath.Join(os.TempDir(), "mago-encode-example.wav")
	defer func() { _ = os.Remove(path) }()

	fileEncoder, err := lib.NewEncoderFile(path, config)
	example.Must(err)
	written, err = fileEncoder.WriteF32(tone)
	example.Must(err)
	example.Must(fileEncoder.Finish())
	example.Must(fileEncoder.Close())

	info, err := os.Stat(path)
	example.Must(err)
	fmt.Printf("file encoder:   wrote %d frames to %s (%d bytes)\n", written, filepath.Base(path), info.Size())

	fileDecoder, err := lib.NewDecoderFile(path, mago.DefaultDecoderConfig())
	example.Must(err)
	reportDecoded(fileDecoder, "file")
	example.Must(fileDecoder.Close())
}

func reportDecoded(decoder *mago.Decoder, source string) {
	_, channels, sampleRate, err := decoder.DataFormat()
	example.Must(err)
	length, err := decoder.LengthInPCMFrames()
	example.Must(err)

	out := make([]float32, 1024*int(channels))
	peak := 0.0
	var total uint64
	for {
		read, err := decoder.ReadF32(out)
		example.Must(err)
		if read == 0 {
			break
		}
		total += read
		for _, sample := range out[:read*uint64(channels)] {
			if magnitude := math.Abs(float64(sample)); magnitude > peak {
				peak = magnitude
			}
		}
	}

	fmt.Printf("%-6s decoded:  %d frames reported, %d read, %d Hz, peak %.3f\n",
		source, length, total, sampleRate, peak)
}
