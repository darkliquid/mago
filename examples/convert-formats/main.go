// Command convert-formats shows PCM conversion: changing sample formats with
// ConvertF32ToS16 and ConvertS16ToF32, reporting BytesPerSample, and converting
// format, channel count and sample rate in one ConvertFramesF32ToS16 call.
// It needs no audio device.
package main

import (
	"fmt"
	"math"

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
	example.Must(lib.ConvertF32ToS16(s16, source, mago.DitherModeNone))
	fmt.Println("converted to s16:")
	fmt.Printf("  %v\n", s16)

	f32Restored := make([]float32, frames)
	example.Must(lib.ConvertS16ToF32(f32Restored, s16))
	fmt.Println("converted back to f32:")
	fmt.Printf("  %v\n", f32Restored)

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
	written, err := lib.ConvertFramesF32ToS16(
		downsampled, 1, sampleRate/2,
		stereo, 2, sampleRate,
	)
	example.Must(err)
	fmt.Printf("ConvertFrames: stereo f32 @%dHz -> mono s16 @%dHz, wrote %d frames\n", sampleRate, sampleRate/2, written)
	fmt.Printf("  %v\n", downsampled[:written])
}
