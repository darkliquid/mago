// Command convert-formats shows PCM conversion: changing sample formats with
// ConvertPCMSamples, reporting BytesPerSample, and converting format, channel
// count and sample rate in one ConvertFrames call. It needs no audio device.
package main

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	const (
		sampleRate = 48_000
		frames     = 8
	)

	source := make([]float32, frames)
	for i := range source {
		source[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/sampleRate))
	}

	fmt.Println("source f32 samples:")
	for i, value := range source {
		fmt.Printf("  [%d] %+.4f\n", i, value)
	}

	s16 := make([]int16, frames)
	example.Must(lib.ConvertPCMSamples(
		unsafe.Pointer(&s16[0]), mago.FormatS16,
		unsafe.Pointer(&source[0]), mago.FormatF32,
		uint64(frames), mago.DitherModeNone,
	))
	fmt.Println("converted to s16:")
	fmt.Printf("  %v\n", s16)

	u8 := make([]uint8, frames)
	example.Must(lib.ConvertPCMSamples(
		unsafe.Pointer(&u8[0]), mago.FormatU8,
		unsafe.Pointer(&source[0]), mago.FormatF32,
		uint64(frames), mago.DitherModeNone,
	))
	fmt.Println("converted to u8:")
	fmt.Printf("  %v\n", u8)

	fmt.Println("bytes per sample:")
	for _, format := range []mago.Format{mago.FormatU8, mago.FormatS16, mago.FormatS24, mago.FormatS32, mago.FormatF32} {
		fmt.Printf("  format %d: %d byte(s)\n", format, mago.BytesPerSample(format))
	}

	// Interleave the mono signal to stereo, then convert it to mono s16 at half
	// the sample rate in a single call.
	stereo := make([]float32, frames*2)
	for i := range frames {
		stereo[2*i] = source[i]
		stereo[2*i+1] = source[i]
	}

	downsampled := make([]int16, frames)
	written, err := lib.ConvertFrames(
		unsafe.Pointer(&downsampled[0]), uint64(len(downsampled)), mago.FormatS16, 1, sampleRate/2,
		unsafe.Pointer(&stereo[0]), uint64(frames), mago.FormatF32, 2, sampleRate,
	)
	example.Must(err)
	fmt.Printf("ConvertFrames: stereo f32 @%dHz -> mono s16 @%dHz, wrote %d frames\n", sampleRate, sampleRate/2, written)
	fmt.Printf("  %v\n", downsampled[:written])
}
