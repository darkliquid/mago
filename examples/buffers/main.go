// Command buffers shows the in-memory buffer and ring buffer APIs:
// ma_audio_buffer, ma_audio_buffer_ref, ma_rb and ma_pcm_rb. It needs no audio
// device.
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

	samples := make([]float32, 16)
	for i := range samples {
		samples[i] = 0.5 * float32(math.Sin(2*math.Pi*440*float64(i)/48_000))
	}

	audioBuffer(lib, samples)
	audioBufferRef(lib, samples)
	ringBuffer(lib)
	pcmRingBuffer(lib)
}

func audioBuffer(lib *mago.Library, samples []float32) {
	buffer, err := lib.NewAudioBuffer(mago.AudioBufferConfig{
		Format:       mago.FormatF32,
		Channels:     1,
		SampleRate:   48_000,
		SizeInFrames: uint64(len(samples)),
		Data:         unsafe.Pointer(&samples[0]),
		DataRef:      samples,
	})
	example.Must(err)
	defer func() { example.Must(buffer.Close()) }()

	length, err := buffer.LengthInPCMFrames()
	example.Must(err)
	fmt.Printf("AudioBuffer:     %d frames, cursor %d\n", length, cursorOf(buffer))

	out := make([]float32, 4)
	read, err := buffer.ReadPCMFrames(unsafe.Pointer(&out[0]), uint64(len(out)), false)
	example.Must(err)
	fmt.Printf("                 read %d frames, first sample %.4f, cursor now %d\n", read, out[0], cursorOf(buffer))
}

func audioBufferRef(lib *mago.Library, samples []float32) {
	ref, err := lib.NewAudioBufferRef(mago.FormatF32, 1, unsafe.Pointer(&samples[0]), uint64(len(samples)), samples)
	example.Must(err)
	defer func() { example.Must(ref.Close()) }()

	out := make([]float32, len(samples))
	read, err := ref.ReadPCMFrames(unsafe.Pointer(&out[0]), uint64(len(out)), false)
	example.Must(err)
	fmt.Printf("AudioBufferRef:  read %d frames, at end: %v\n", read, ref.AtEnd())
}

func ringBuffer(lib *mago.Library) {
	ring, err := lib.NewRingBuffer(256)
	example.Must(err)
	defer func() { example.Must(ring.Close()) }()

	ptr, size, err := ring.AcquireWrite(64)
	example.Must(err)
	region := unsafe.Slice((*byte)(ptr), size)
	for i := range region {
		region[i] = byte(i)
	}
	example.Must(ring.CommitWrite(size))

	readPtr, readSize, err := ring.AcquireRead(64)
	example.Must(err)
	read := unsafe.Slice((*byte)(readPtr), readSize)
	example.Must(ring.CommitRead(readSize))
	fmt.Printf("RingBuffer:      wrote %d bytes, read back %v\n", size, read[:4])
}

func pcmRingBuffer(lib *mago.Library) {
	ring, err := lib.NewPCMRingBuffer(mago.FormatF32, 1, 64)
	example.Must(err)
	defer func() { example.Must(ring.Close()) }()

	ptr, frames, err := ring.AcquireWrite(16)
	example.Must(err)
	region := unsafe.Slice((*float32)(ptr), frames)
	for i := range region {
		region[i] = float32(i) * 0.1
	}
	example.Must(ring.CommitWrite(frames))

	readPtr, readFrames, err := ring.AcquireRead(16)
	example.Must(err)
	read := unsafe.Slice((*float32)(readPtr), readFrames)
	example.Must(ring.CommitRead(readFrames))
	fmt.Printf("PCMRingBuffer:   f32 channels %d, wrote %d frames, read back %v\n",
		ring.Channels(), frames, read[:4])
}

func cursorOf(buffer *mago.AudioBuffer) uint64 {
	cursor, err := buffer.CursorInPCMFrames()
	if err != nil {
		return 0
	}
	return cursor
}
