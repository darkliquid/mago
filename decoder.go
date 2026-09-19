//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// DecoderConfig configures a decoder. Leaving Channels or SampleRate at zero
// keeps the stream's native values.
type DecoderConfig struct {
	Format         Format
	Channels       uint32
	SampleRate     uint32
	ChannelMixMode ChannelMixMode
	DitherMode     DitherMode
	SeekPointCount uint32
}

// DefaultDecoderConfig returns a decoder that outputs 32-bit float frames at the
// stream's native channel count and sample rate.
func DefaultDecoderConfig() DecoderConfig {
	return DecoderConfig{
		Format:         FormatF32,
		ChannelMixMode: ChannelMixModeRectangular,
		DitherMode:     DitherModeNone,
	}
}

// Decoder decodes an encoded stream (WAV, FLAC or MP3) into PCM frames.
type Decoder struct {
	lib        *Library
	handle     *decoderHandle
	dataRef    any
	format     Format
	channels   uint32
	sampleRate uint32
}

// NewDecoderMemory decodes from an in-memory encoded stream. The data slice is
// retained so it cannot be collected while the decoder reads from it.
func (lib *Library) NewDecoderMemory(data []byte, config DecoderConfig) (*Decoder, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("mago: decoder memory is empty")
	}
	return lib.newDecoder("ma_decoder_init_memory", config, data, func(native *decoderConfigNative, handle *decoderHandle) Result {
		return lib.bindings.maDecoderInitMemory(unsafe.Pointer(&data[0]), uintptr(len(data)), native, handle)
	})
}

// NewDecoderFile decodes from a file on disk.
func (lib *Library) NewDecoderFile(path string, config DecoderConfig) (*Decoder, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	return lib.newDecoder("ma_decoder_init_file", config, nil, func(native *decoderConfigNative, handle *decoderHandle) Result {
		return lib.bindings.maDecoderInitFile(path, native, handle)
	})
}

func (lib *Library) newDecoder(op string, config DecoderConfig, dataRef any, init func(*decoderConfigNative, *decoderHandle) Result) (*Decoder, error) {
	native := decoderConfigNative{
		Format:         config.Format,
		Channels:       config.Channels,
		SampleRate:     config.SampleRate,
		ChannelMixMode: config.ChannelMixMode,
		DitherMode:     config.DitherMode,
		SeekPointCount: config.SeekPointCount,
		Resampling: resamplerConfigNative{
			Algorithm: ResampleAlgorithmLinear,
			Linear:    resamplerLinearConfigNative{LPFOrder: 4},
		},
	}

	handle := (*decoderHandle)(lib.bindings.magoAlloc(magoObjectDecoder))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate decoder: out of memory")
	}
	if result := init(&native, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError(op, result)
	}

	decoder := &Decoder{lib: lib, handle: handle, dataRef: dataRef}
	format, channels, sampleRate, err := decoder.DataFormat()
	if err != nil {
		lib.bindings.maDecoderUninit(handle)
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, err
	}
	decoder.format = format
	decoder.channels = channels
	decoder.sampleRate = sampleRate
	return decoder, nil
}

// DataFormat reports the decoder's output sample format, channels and sample
// rate.
func (d *Decoder) DataFormat() (Format, uint32, uint32, error) {
	if d == nil || d.handle == nil {
		return FormatUnknown, 0, 0, fmt.Errorf("mago: nil decoder")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return FormatUnknown, 0, 0, err
	}

	var format Format
	var channels, sampleRate uint32
	if result := d.lib.bindings.maDecoderGetDataFormat(d.handle, &format, &channels, &sampleRate, nil, 0); result != Success {
		return FormatUnknown, 0, 0, d.lib.resultError("ma_decoder_get_data_format", result)
	}
	return format, channels, sampleRate, nil
}

// ReadF32 decodes up to len(out) float32 samples into out, returning the number
// of frames read. The decoder must be configured for f32 output.
func (d *Decoder) ReadF32(out []float32) (uint64, error) {
	if d == nil || d.handle == nil {
		return 0, fmt.Errorf("mago: nil decoder")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if d.format != FormatF32 {
		return 0, fmt.Errorf("mago: decoder output format is %d, not f32", d.format)
	}
	frameCount, err := d.frameCount(len(out))
	if err != nil {
		return 0, err
	}

	var read uint64
	if result := d.lib.bindings.maDecoderReadPCMFrames(d.handle, unsafe.Pointer(&out[0]), frameCount, &read); result != Success {
		return read, d.lib.resultError("ma_decoder_read_pcm_frames", result)
	}
	return read, nil
}

// ReadS16 decodes up to len(out) signed 16-bit samples into out, returning the
// number of frames read. The decoder must be configured for s16 output.
func (d *Decoder) ReadS16(out []int16) (uint64, error) {
	if d == nil || d.handle == nil {
		return 0, fmt.Errorf("mago: nil decoder")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if d.format != FormatS16 {
		return 0, fmt.Errorf("mago: decoder output format is %d, not s16", d.format)
	}
	frameCount, err := d.frameCount(len(out))
	if err != nil {
		return 0, err
	}

	var read uint64
	if result := d.lib.bindings.maDecoderReadPCMFrames(d.handle, unsafe.Pointer(&out[0]), frameCount, &read); result != Success {
		return read, d.lib.resultError("ma_decoder_read_pcm_frames", result)
	}
	return read, nil
}

func (d *Decoder) frameCount(sampleCount int) (uint64, error) {
	if d.channels == 0 {
		return 0, fmt.Errorf("mago: decoder reports zero channels")
	}
	return uint64(sampleCount) / uint64(d.channels), nil
}

// SeekToPCMFrame moves the decode position.
func (d *Decoder) SeekToPCMFrame(frameIndex uint64) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil decoder")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}
	return d.lib.resultError("ma_decoder_seek_to_pcm_frame", d.lib.bindings.maDecoderSeekToPCMFrame(d.handle, frameIndex))
}

// CursorInPCMFrames reports the current decode position.
func (d *Decoder) CursorInPCMFrames() (uint64, error) {
	return d.readFrameCount("ma_decoder_get_cursor_in_pcm_frames", d.lib.bindings.maDecoderGetCursorInPCMFrames)
}

// LengthInPCMFrames reports the total length of the stream.
func (d *Decoder) LengthInPCMFrames() (uint64, error) {
	return d.readFrameCount("ma_decoder_get_length_in_pcm_frames", d.lib.bindings.maDecoderGetLengthInPCMFrames)
}

// AvailableFrames reports how many frames remain before the end of the stream.
func (d *Decoder) AvailableFrames() (uint64, error) {
	return d.readFrameCount("ma_decoder_get_available_frames", d.lib.bindings.maDecoderGetAvailableFrames)
}

func (d *Decoder) readFrameCount(op string, fn func(*decoderHandle, *uint64) Result) (uint64, error) {
	if d == nil || d.handle == nil {
		return 0, fmt.Errorf("mago: nil decoder")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0, err
	}

	var value uint64
	if result := fn(d.handle, &value); result != Success {
		return 0, d.lib.resultError(op, result)
	}
	return value, nil
}

// Close uninitializes the decoder and frees it. Any retained input data is
// released.
func (d *Decoder) Close() error {
	if d == nil || d.handle == nil {
		return nil
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}

	d.lib.bindings.maDecoderUninit(d.handle)
	d.lib.bindings.magoFree(unsafe.Pointer(d.handle))
	d.handle = nil
	d.dataRef = nil
	return nil
}
