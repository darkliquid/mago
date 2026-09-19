//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// ConvertPCMFrames converts interleaved frames between two PCM formats
// without changing the channel count or sample rate.
func (lib *Library) ConvertPCMFrames(out, in []byte, formatOut, formatIn Format, channels uint32, dither DitherMode) error {
	if lib == nil {
		return fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return err
	}
	if len(in) == 0 {
		return nil
	}
	bytesIn := BytesPerSample(formatIn) * channels
	bytesOut := BytesPerSample(formatOut) * channels
	if bytesIn == 0 || len(in)%int(bytesIn) != 0 || bytesOut == 0 {
		return ErrInvalidSliceLength
	}
	frameCount := uint64(len(in) / int(bytesIn))
	requiredOut := int(frameCount * uint64(bytesOut))
	if len(out) < requiredOut {
		return ErrOutputTooSmall
	}
	lib.bindings.maConvertPCMFramesFormat(unsafe.Pointer(&out[0]), formatOut, unsafe.Pointer(&in[0]), formatIn, frameCount, channels, dither)
	return nil
}

// ConvertF32ToS16 converts float32 PCM samples to int16 PCM samples.
func (lib *Library) ConvertF32ToS16(out []int16, in []float32, dither DitherMode) error {
	if lib == nil {
		return fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return err
	}
	if len(in) == 0 {
		return nil
	}
	if len(out) < len(in) {
		return ErrOutputTooSmall
	}
	lib.bindings.maPCMConvert(unsafe.Pointer(&out[0]), FormatS16, unsafe.Pointer(&in[0]), FormatF32, uint64(len(in)), dither)
	return nil
}

// ConvertS16ToF32 converts int16 PCM samples to float32 PCM samples.
func (lib *Library) ConvertS16ToF32(out []float32, in []int16) error {
	if lib == nil {
		return fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return err
	}
	if len(in) == 0 {
		return nil
	}
	if len(out) < len(in) {
		return ErrOutputTooSmall
	}
	lib.bindings.maPCMConvert(unsafe.Pointer(&out[0]), FormatF32, unsafe.Pointer(&in[0]), FormatS16, uint64(len(in)), DitherModeNone)
	return nil
}

// ConvertFrames converts between formats, channel counts and sample rates in a
// single call and returns the number of frames written to out.
func (lib *Library) ConvertFrames(
	out []byte, formatOut Format, channelsOut uint32, sampleRateOut uint32,
	in []byte, formatIn Format, channelsIn uint32, sampleRateIn uint32,
) (uint64, error) {
	if lib == nil {
		return 0, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, nil
	}
	bytesIn := BytesPerSample(formatIn) * channelsIn
	bytesOut := BytesPerSample(formatOut) * channelsOut
	if bytesIn == 0 || len(in)%int(bytesIn) != 0 || bytesOut == 0 || len(out)%int(bytesOut) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCountIn := uint64(len(in) / int(bytesIn))
	frameCountOut := uint64(len(out) / int(bytesOut))
	return lib.bindings.maConvertFrames(
		unsafe.Pointer(&out[0]), frameCountOut, formatOut, channelsOut, sampleRateOut,
		unsafe.Pointer(&in[0]), frameCountIn, formatIn, channelsIn, sampleRateIn,
	), nil
}

// ConvertFramesF32ToS16 converts between formats, channel counts and sample rates
// from float32 to int16 in a single call and returns the number of frames written to out.
func (lib *Library) ConvertFramesF32ToS16(
	out []int16, channelsOut uint32, sampleRateOut uint32,
	in []float32, channelsIn uint32, sampleRateIn uint32,
) (uint64, error) {
	if lib == nil {
		return 0, fmt.Errorf("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, nil
	}
	if channelsIn == 0 || len(in)%int(channelsIn) != 0 || channelsOut == 0 || len(out)%int(channelsOut) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCountIn := uint64(len(in) / int(channelsIn))
	frameCountOut := uint64(len(out) / int(channelsOut))
	return lib.bindings.maConvertFrames(
		unsafe.Pointer(&out[0]), frameCountOut, FormatS16, channelsOut, sampleRateOut,
		unsafe.Pointer(&in[0]), frameCountIn, FormatF32, channelsIn, sampleRateIn,
	), nil
}

// BytesPerSample reports the size in bytes of a single sample in a PCM format. It
// returns 0 for an unknown format.
func BytesPerSample(format Format) uint32 {
	switch format {
	case FormatU8:
		return 1
	case FormatS16:
		return 2
	case FormatS24:
		return 3
	case FormatS32, FormatF32:
		return 4
	default:
		return 0
	}
}
