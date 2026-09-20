// Command decode shows miniaudio's decoder: it synthesises a WAV in memory,
// decodes it, then decodes the same bytes from a temporary file. It needs no
// audio device.
package main

import (
	"bytes"
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
	wav := synthWAV(frames, sampleRate)

	fmt.Printf("synthesised WAV: %d bytes\n", len(wav))

	memory, err := lib.NewDecoderMemory(wav, mago.DefaultDecoderConfig())
	example.Must(err)
	report("memory", memory)
	example.Must(memory.Close())

	path := filepath.Join(os.TempDir(), "mago-decode-example.wav")
	example.Must(os.WriteFile(path, wav, 0o600))
	defer func() { _ = os.Remove(path) }()

	file, err := lib.NewDecoderFile(path, mago.DefaultDecoderConfig())
	example.Must(err)
	report("file", file)
	example.Must(file.SeekToPCMFrame(0))
	fmt.Println("                 seeked back to the start")
	example.Must(file.Close())
}

func report(source string, decoder *mago.Decoder) {
	format, channels, sampleRate, err := decoder.DataFormat()
	example.Must(err)

	length, err := decoder.LengthInPCMFrames()
	example.Must(err)

	peak := 0.0
	buffer := make([]float32, 1024*int(channels))
	var total uint64
	for {
		read, err := decoder.ReadF32(buffer)
		example.Must(err)
		if read == 0 {
			break
		}
		total += read
		for _, sample := range buffer[:read*uint64(channels)] {
			if magnitude := math.Abs(float64(sample)); magnitude > peak {
				peak = magnitude
			}
		}
	}

	fmt.Printf("%-6s decoder:  format %d, %d channel(s), %d Hz, %d frames, read %d, peak %.3f\n",
		source, format, channels, sampleRate, length, total, peak)
}

// synthWAV builds a mono 16-bit PCM WAV holding a short sine tone.
func synthWAV(frames, sampleRate int) []byte {
	var pcm bytes.Buffer
	for i := 0; i < frames; i++ {
		value := math.Sin(2 * math.Pi * 440 * float64(i) / float64(sampleRate))
		_ = binary.Write(&pcm, binary.LittleEndian, int16(value*0.5*32767))
	}

	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+pcm.Len()))
	wav.WriteString("WAVE")
	wav.WriteString("fmt ")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(16))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(1))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(1))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(16))
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(pcm.Len()))
	wav.Write(pcm.Bytes())
	return wav.Bytes()
}
