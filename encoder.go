//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// EncoderConfig configures an encoder. WAV and FLAC containers are supported,
// with f32 or s16 samples.
type EncoderConfig struct {
	EncodingFormat EncodingFormat
	Format         Format
	Channels       uint32
	SampleRate     uint32
}

// DefaultEncoderConfig returns a WAV encoder writing 32-bit float samples.
func DefaultEncoderConfig(channels, sampleRate uint32) EncoderConfig {
	return EncoderConfig{
		EncodingFormat: EncodingFormatWAV,
		Format:         FormatF32,
		Channels:       channels,
		SampleRate:     sampleRate,
	}
}

func (c EncoderConfig) validate() error {
	if c.EncodingFormat != EncodingFormatWAV && c.EncodingFormat != EncodingFormatFLAC {
		return fmt.Errorf("mago: unsupported encoding format %d", c.EncodingFormat)
	}
	if c.Format != FormatF32 && c.Format != FormatS16 {
		return fmt.Errorf("mago: unsupported encoder sample format %d", c.Format)
	}
	if c.Channels == 0 {
		return fmt.Errorf("mago: encoder requires a channel count")
	}
	if c.SampleRate == 0 {
		return fmt.Errorf("mago: encoder requires a sample rate")
	}
	return nil
}

func (c EncoderConfig) native() encoderConfigNative {
	return encoderConfigNative{
		EncodingFormat: c.EncodingFormat,
		Format:         c.Format,
		Channels:       c.Channels,
		SampleRate:     c.SampleRate,
	}
}

// encoderSink accumulates encoded bytes in Go so the encoder can be driven
// without a file. It must support seeking because miniaudio patches the
// container header when the encoder is uninitialized.
type encoderSink struct {
	data []byte
	pos  int64
}

func (s *encoderSink) write(p []byte) int {
	if s.pos < 0 {
		return 0
	}
	if end := s.pos + int64(len(p)); end > int64(len(s.data)) {
		s.data = append(s.data, make([]byte, end-int64(len(s.data)))...)
	}
	copy(s.data[s.pos:], p)
	s.pos += int64(len(p))
	return len(p)
}

func (s *encoderSink) seek(offset int64, origin uint32) {
	switch origin {
	case 0: // ma_seek_origin_start
		s.pos = offset
	case 1: // ma_seek_origin_current
		s.pos += offset
	case 2: // ma_seek_origin_end
		s.pos = int64(len(s.data)) + offset
	}
	if s.pos < 0 {
		s.pos = 0
	}
}

// callbackResult converts a miniaudio result into the value a purego callback
// must return, which is the result code in the low 32 bits of the register.
func callbackResult(result Result) uintptr {
	return uintptr(uint32(int32(result))) //nolint:gosec // result codes are int32 values
}

var (
	encoderSeq   atomic.Uint64
	encoderSinks sync.Map // token -> *encoderSink

	encoderWritePtr = purego.NewCallback(func(userData uintptr, buffer unsafe.Pointer, bytesToWrite uintptr, bytesWritten unsafe.Pointer) uintptr {
		value, ok := encoderSinks.Load(userData)
		if !ok || buffer == nil {
			return callbackResult(Error)
		}
		written := value.(*encoderSink).write(unsafe.Slice((*byte)(buffer), int(bytesToWrite)))
		if bytesWritten != nil {
			*(*uintptr)(bytesWritten) = uintptr(written)
		}
		return callbackResult(Success)
	})

	encoderSeekPtr = purego.NewCallback(func(userData uintptr, offset int64, origin uint32) uintptr {
		value, ok := encoderSinks.Load(userData)
		if !ok {
			return callbackResult(Error)
		}
		value.(*encoderSink).seek(offset, origin)
		return callbackResult(Success)
	})
)

// Encoder writes encoded audio to a file or to an in-memory buffer.
type Encoder struct {
	lib      *Library
	handle   *encoderHandle
	format   Format
	channels uint32
	token    uintptr
	bridge   *encoderBridgeNative
	finished bool
}

// NewEncoderFile writes encoded audio to path using miniaudio's file I/O.
func (lib *Library) NewEncoderFile(path string, config EncoderConfig) (*Encoder, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	native := config.native()
	handle := (*encoderHandle)(lib.bindings.magoAlloc(magoObjectEncoder))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate encoder: out of memory")
	}
	if result := lib.bindings.maEncoderInitFile(path, &native, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_encoder_init_file", result)
	}

	return &Encoder{lib: lib, handle: handle, format: config.Format, channels: config.Channels}, nil
}

// NewEncoderWriter encodes into memory. Call Finish and then Bytes to read the
// encoded stream.
func (lib *Library) NewEncoderWriter(config EncoderConfig) (*Encoder, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if err := config.validate(); err != nil {
		return nil, err
	}

	native := config.native()
	handle := (*encoderHandle)(lib.bindings.magoAlloc(magoObjectEncoder))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate encoder: out of memory")
	}

	token := uintptr(encoderSeq.Add(1))
	encoderSinks.Store(token, &encoderSink{})

	var bridge *encoderBridgeNative
	if result := lib.bindings.magoEncoderInit(handle, &native, encoderWritePtr, encoderSeekPtr, token, &bridge); result != Success {
		encoderSinks.Delete(token)
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("mago_encoder_init", result)
	}

	return &Encoder{lib: lib, handle: handle, format: config.Format, channels: config.Channels, token: token, bridge: bridge}, nil
}

// WriteF32 encodes the given interleaved f32 frames. The encoder must be
// configured for f32 output.
func (e *Encoder) WriteF32(frames []float32) (uint64, error) {
	if err := e.writable(FormatF32); err != nil {
		return 0, err
	}
	if len(frames) == 0 {
		return 0, nil
	}
	return e.write(unsafe.Pointer(&frames[0]), uint64(len(frames))/uint64(e.channels))
}

// WriteS16 encodes the given interleaved s16 frames. The encoder must be
// configured for s16 output.
func (e *Encoder) WriteS16(frames []int16) (uint64, error) {
	if err := e.writable(FormatS16); err != nil {
		return 0, err
	}
	if len(frames) == 0 {
		return 0, nil
	}
	return e.write(unsafe.Pointer(&frames[0]), uint64(len(frames))/uint64(e.channels))
}

func (e *Encoder) write(ptr unsafe.Pointer, frameCount uint64) (uint64, error) {
	var written uint64
	if result := e.lib.bindings.maEncoderWritePCMFrames(e.handle, ptr, frameCount, &written); result != Success {
		return written, e.lib.resultError("ma_encoder_write_pcm_frames", result)
	}
	return written, nil
}

func (e *Encoder) writable(format Format) error {
	if e == nil || e.handle == nil {
		return fmt.Errorf("mago: nil encoder")
	}
	if e.finished {
		return fmt.Errorf("mago: encoder is already finished")
	}
	if err := e.lib.ensureOpen(); err != nil {
		return err
	}
	if e.format != format {
		return fmt.Errorf("mago: encoder format is %d, not %d", e.format, format)
	}
	return nil
}

// Finish finalizes the stream, patching the container header. It is safe to call
// more than once.
func (e *Encoder) Finish() error {
	if e == nil || e.handle == nil {
		return nil
	}
	if e.finished {
		return nil
	}
	if err := e.lib.ensureOpen(); err != nil {
		return err
	}

	e.lib.bindings.maEncoderUninit(e.handle)
	e.lib.bindings.magoFree(unsafe.Pointer(e.handle))
	e.handle = nil
	if e.bridge != nil {
		e.lib.bindings.magoFree(unsafe.Pointer(e.bridge))
		e.bridge = nil
	}
	e.finished = true
	return nil
}

// Bytes returns the encoded stream for an encoder created with NewEncoderWriter.
// It is only complete once Finish has been called, and returns nil for a file
// encoder.
func (e *Encoder) Bytes() []byte {
	if e == nil || e.token == 0 {
		return nil
	}
	value, ok := encoderSinks.Load(e.token)
	if !ok {
		return nil
	}
	sink := value.(*encoderSink)
	out := make([]byte, len(sink.data))
	copy(out, sink.data)
	return out
}

// Close finalizes the stream if needed and releases the encoder.
func (e *Encoder) Close() error {
	if e == nil {
		return nil
	}
	if e.token != 0 {
		encoderSinks.Delete(e.token)
		e.token = 0
	}
	return e.Finish()
}
