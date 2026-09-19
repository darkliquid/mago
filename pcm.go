//go:build darwin || freebsd || linux || netbsd || windows

package mago

import "unsafe"

// ConvertPCMSamples converts sampleCount samples from formatIn to formatOut.
// The output buffer must have room for sampleCount samples of formatOut.
func (lib *Library) ConvertPCMSamples(out unsafe.Pointer, formatOut Format, in unsafe.Pointer, formatIn Format, sampleCount uint64, dither DitherMode) error {
	if err := lib.ensureOpen(); err != nil {
		return err
	}
	lib.bindings.maPCMConvert(out, formatOut, in, formatIn, sampleCount, dither)
	return nil
}

// ConvertPCMFramesFormat converts frameCount interleaved frames between two PCM
// formats without changing the channel count or sample rate.
func (lib *Library) ConvertPCMFramesFormat(out unsafe.Pointer, formatOut Format, in unsafe.Pointer, formatIn Format, frameCount uint64, channels uint32, dither DitherMode) error {
	if err := lib.ensureOpen(); err != nil {
		return err
	}
	lib.bindings.maConvertPCMFramesFormat(out, formatOut, in, formatIn, frameCount, channels, dither)
	return nil
}

// ConvertFrames converts between formats, channel counts and sample rates in a
// single call and returns the number of frames written to out. The output buffer
// must have room for frameCountOut frames of formatOut with channelsOut channels.
func (lib *Library) ConvertFrames(
	out unsafe.Pointer, frameCountOut uint64, formatOut Format, channelsOut uint32, sampleRateOut uint32,
	in unsafe.Pointer, frameCountIn uint64, formatIn Format, channelsIn uint32, sampleRateIn uint32,
) (uint64, error) {
	if err := lib.ensureOpen(); err != nil {
		return 0, err
	}
	return lib.bindings.maConvertFrames(
		out, frameCountOut, formatOut, channelsOut, sampleRateOut,
		in, frameCountIn, formatIn, channelsIn, sampleRateIn,
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
