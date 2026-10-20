package mago

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestConvertPCMSamplesF32ToS16(t *testing.T) {
	lib := newNullLibrary(t)

	in := []float32{0, 0.5, -0.5, 1, -1}
	out := make([]int16, len(in))

	if err := lib.ConvertF32ToS16(out, in, DitherModeNone); err != nil {
		t.Fatalf("ConvertF32ToS16: %v", err)
	}

	want := []int16{0, 16384, -16384, 32767, -32768}
	for i := range want {
		if diff := int(out[i]) - int(want[i]); diff > 1 || diff < -1 {
			t.Errorf("sample %d = %d, want ~%d", i, out[i], want[i])
		}
	}

	// Output too small
	shortOut := make([]int16, len(in)-1)
	if err := lib.ConvertF32ToS16(shortOut, in, DitherModeNone); err != ErrOutputTooSmall {
		t.Errorf("expected ErrOutputTooSmall, got %v", err)
	}
}

func TestConvertPCMFramesFormatS16ToF32(t *testing.T) {
	lib := newNullLibrary(t)

	in := []int16{0, 16384, -16384}
	out := make([]float32, len(in))

	if err := lib.ConvertS16ToF32(out, in); err != nil {
		t.Fatalf("ConvertS16ToF32: %v", err)
	}

	if out[1] < 0.49 || out[1] > 0.51 {
		t.Errorf("half-scale sample = %v, want ~0.5", out[1])
	}

	// Output too small
	shortOut := make([]float32, len(in)-1)
	if err := lib.ConvertS16ToF32(shortOut, in); err != ErrOutputTooSmall {
		t.Errorf("expected ErrOutputTooSmall, got %v", err)
	}
}

func TestConvertPCMFramesBytes(t *testing.T) {
	lib := newNullLibrary(t)

	in := []byte{0x00, 0x40} // one int16 sample (16384) in little-endian
	out := make([]byte, 4)   // one float32 sample

	if err := lib.ConvertPCMFrames(out, in, FormatF32, FormatS16, 1, DitherModeNone); err != nil {
		t.Fatalf("ConvertPCMFrames: %v", err)
	}

	// Misaligned in buffer
	oddIn := []byte{0x00}
	if err := lib.ConvertPCMFrames(out, oddIn, FormatF32, FormatS16, 1, DitherModeNone); err != ErrInvalidSliceLength {
		t.Errorf("expected ErrInvalidSliceLength, got %v", err)
	}

	// Output too small
	shortOut := make([]byte, 2)
	if err := lib.ConvertPCMFrames(shortOut, in, FormatF32, FormatS16, 1, DitherModeNone); err != ErrOutputTooSmall {
		t.Errorf("expected ErrOutputTooSmall, got %v", err)
	}
}

func TestBytesPerSample(t *testing.T) {
	cases := map[Format]uint32{
		FormatU8: 1, FormatS16: 2, FormatS24: 3, FormatS32: 4, FormatF32: 4,
	}
	for format, want := range cases {
		if got := BytesPerSample(format); got != want {
			t.Errorf("BytesPerSample(%v) = %d, want %d", format, got, want)
		}
	}
	if got := BytesPerSample(FormatUnknown); got != 0 {
		t.Errorf("BytesPerSample(unknown) = %d, want 0", got)
	}
}

func TestConvertFramesStereoF32ToMonoS16(t *testing.T) {
	lib := newNullLibrary(t)

	const framesIn = 4
	in := []float32{0.5, 0.5, 0.25, 0.25, 0, 0, 1, 1}
	out := make([]int16, framesIn)

	written, err := lib.ConvertFramesF32ToS16(
		out, 1, 48_000,
		in, 2, 48_000,
	)
	if err != nil {
		t.Fatalf("ConvertFramesF32ToS16: %v", err)
	}
	if written != framesIn {
		t.Fatalf("wrote %d frames, want %d", written, framesIn)
	}
	if out[0] == 0 && out[1] == 0 {
		t.Fatal("expected non-silent output from a down-mix of non-zero input")
	}
}

func TestConvertFramesExCombined(t *testing.T) {
	lib := newNullLibrary(t)

	// Mono s16 DC of ~0.5 builds 44.1kHz, converted to stereo f32 at 48kHz.
	const framesIn = 512
	inBytes := make([]byte, framesIn*2)
	for i := range framesIn {
		inBytes[i*2] = 0x00
		inBytes[i*2+1] = 0x40 // 16384
	}
	outBytes := make([]byte, 600*2*4) // up to 600 stereo f32 frames
	config := ConvertFramesConfig{
		FormatIn:      FormatS16,
		FormatOut:     FormatF32,
		ChannelsIn:    1,
		ChannelsOut:   2,
		SampleRateIn:  44_100,
		SampleRateOut: 48_000,
	}

	written, err := lib.ConvertFramesEx(outBytes, inBytes, config)
	if err != nil {
		t.Fatalf("ConvertFramesEx: %v", err)
	}
	if written < framesIn {
		t.Fatalf("wrote %d frames, want at least %d", written, framesIn)
	}

	middle := int(written) / 2
	for ch := 0; ch < 2; ch++ {
		offset := (middle*2 + ch) * 4
		v := math.Float32frombits(binary.LittleEndian.Uint32(outBytes[offset : offset+4]))
		if v < 0.45 || v > 0.55 {
			t.Fatalf("sample at frame %d ch %d = %v, want ~0.5", middle, ch, v)
		}
	}

	if _, err := lib.ConvertFramesEx(outBytes, []byte{0}, config); err != ErrInvalidSliceLength {
		t.Fatalf("expected ErrInvalidSliceLength, got %v", err)
	}
}
