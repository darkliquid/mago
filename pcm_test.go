package mago

import (
	"testing"
	"unsafe"
)

func TestConvertPCMSamplesF32ToS16(t *testing.T) {
	lib := newNullLibrary(t)

	in := []float32{0, 0.5, -0.5, 1, -1}
	out := make([]int16, len(in))

	if err := lib.ConvertPCMSamples(
		unsafe.Pointer(&out[0]), FormatS16,
		unsafe.Pointer(&in[0]), FormatF32,
		uint64(len(in)), DitherModeNone,
	); err != nil {
		t.Fatalf("ConvertPCMSamples: %v", err)
	}

	want := []int16{0, 16384, -16384, 32767, -32768}
	for i := range want {
		if diff := int(out[i]) - int(want[i]); diff > 1 || diff < -1 {
			t.Errorf("sample %d = %d, want ~%d", i, out[i], want[i])
		}
	}
}

func TestConvertPCMFramesFormatS16ToF32(t *testing.T) {
	lib := newNullLibrary(t)

	in := []int16{0, 16384, -16384}
	out := make([]float32, len(in))

	if err := lib.ConvertPCMFramesFormat(
		unsafe.Pointer(&out[0]), FormatF32,
		unsafe.Pointer(&in[0]), FormatS16,
		uint64(len(in)), 1, DitherModeNone,
	); err != nil {
		t.Fatalf("ConvertPCMFramesFormat: %v", err)
	}

	if out[1] < 0.49 || out[1] > 0.51 {
		t.Errorf("half-scale sample = %v, want ~0.5", out[1])
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

	written, err := lib.ConvertFrames(
		unsafe.Pointer(&out[0]), framesIn, FormatS16, 1, 48_000,
		unsafe.Pointer(&in[0]), framesIn, FormatF32, 2, 48_000,
	)
	if err != nil {
		t.Fatalf("ConvertFrames: %v", err)
	}
	if written != framesIn {
		t.Fatalf("wrote %d frames, want %d", written, framesIn)
	}
	if out[0] == 0 && out[1] == 0 {
		t.Fatal("expected non-silent output from a down-mix of non-zero input")
	}
}
