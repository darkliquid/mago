package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/darkliquid/mago/audio"
	"github.com/darkliquid/mago/internal/buildlib"
)

func main() {
	repoRoot, err := findRepoRoot()
	must(err)

	libPath := buildlib.DefaultOutputPath(repoRoot)
	must(buildlib.Build(repoRoot, libPath, ""))

	engine, err := audio.Open(audio.Config{
		LibraryPath: libPath,
	})
	must(err)
	defer func() { must(engine.Close()) }()

	clip, err := engine.Load(bytes.NewReader(makeTestWAV()))
	must(err)
	defer clip.Release()

	stream, err := engine.Play(clip, audio.PlayOptions{
		Loop:   true,
		Volume: 0.25,
		Speed:  1,
	})
	must(err)
	defer stream.Close()

	fmt.Printf("clip duration: %v\n", clip.Duration())
	time.Sleep(400 * time.Millisecond)

	fmt.Println("speeding up...")
	must(stream.SetSpeed(1.5))
	time.Sleep(400 * time.Millisecond)

	fmt.Println("reversing...")
	stream.SetReverse(true)
	time.Sleep(400 * time.Millisecond)

	fmt.Println("fading out...")
	stream.FadeOutAndStop(500 * time.Millisecond)
	time.Sleep(700 * time.Millisecond)
}

func makeTestWAV() []byte {
	const sampleRate uint32 = 48000
	const channels uint16 = 2
	const duration = 1
	frameCount := sampleRate * duration

	pcm := bytes.Buffer{}
	for frame := uint32(0); frame < frameCount; frame++ {
		t := float64(frame) / float64(sampleRate)
		sample := int16(math.Sin(2*math.Pi*330*t) * 0.3 * 32767)
		for ch := uint16(0); ch < channels; ch++ {
			must(binary.Write(&pcm, binary.LittleEndian, sample))
		}
	}

	dataSize := pcm.Len()
	dataSize32 := mustUint32(dataSize)
	blockAlign := channels * 2
	byteRate := sampleRate * uint32(blockAlign)

	var wav bytes.Buffer
	wav.WriteString("RIFF")
	must(binary.Write(&wav, binary.LittleEndian, 36+dataSize32))
	wav.WriteString("WAVE")
	wav.WriteString("fmt ")
	must(binary.Write(&wav, binary.LittleEndian, uint32(16)))
	must(binary.Write(&wav, binary.LittleEndian, uint16(1)))
	must(binary.Write(&wav, binary.LittleEndian, channels))
	must(binary.Write(&wav, binary.LittleEndian, sampleRate))
	must(binary.Write(&wav, binary.LittleEndian, byteRate))
	must(binary.Write(&wav, binary.LittleEndian, blockAlign))
	must(binary.Write(&wav, binary.LittleEndian, uint16(16)))
	wav.WriteString("data")
	must(binary.Write(&wav, binary.LittleEndian, dataSize32))
	wav.Write(pcm.Bytes())
	return wav.Bytes()
}

func findRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func mustUint32(v int) uint32 {
	if v < 0 {
		must(fmt.Errorf("value %d exceeds uint32 range", v))
	}
	var out uint32
	if _, err := fmt.Sscan(strconv.Itoa(v), &out); err != nil {
		must(fmt.Errorf("value %d exceeds uint32 range", v))
	}
	return out
}
