//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// DataSource is the Go view of a miniaudio data source: something that can hand
// out PCM frames in one fixed format.
//
// ReadPCMFrames fills out with interleaved samples in the format reported by
// DataFormat and returns the number of frames written. Callers size out using
// BytesPerSample.
type DataSource interface {
	ReadPCMFrames(out []byte) (uint64, error)
	SeekToPCMFrame(frameIndex uint64) error
	DataFormat() (Format, uint32, uint32, error)
	CursorInPCMFrames() (uint64, error)
	LengthInPCMFrames() (uint64, error)
	SetLooping(looping bool) error
}

type dataSourceState struct {
	source   DataSource
	lib      *Library
	format   Format
	channels uint32
}

var (
	dataSourceSeq atomic.Uint64
	dataSources   sync.Map // token -> *dataSourceState

	dataSourceReadPtr = purego.NewCallback(func(token uintptr, framesOut unsafe.Pointer, frameCount uint64, framesRead unsafe.Pointer) uintptr {
		state, ok := loadDataSource(token)
		if !ok {
			return callbackResult(Error)
		}

		if framesOut == nil {
			// miniaudio uses a NULL output buffer to mean "seek forward".
			cursor, err := state.source.CursorInPCMFrames()
			if err != nil {
				return callbackResult(Error)
			}
			target := cursor + frameCount
			if length, err := state.source.LengthInPCMFrames(); err == nil && target > length {
				target = length
			}
			if err := state.source.SeekToPCMFrame(target); err != nil {
				return callbackResult(Error)
			}
			writeUint64(framesRead, target-cursor)
			return callbackResult(Success)
		}

		frameSize := frameSizeOf(state.format, state.channels)
		if frameSize == 0 {
			return callbackResult(Error)
		}
		buffer := unsafe.Slice((*byte)(framesOut), int(frameCount)*frameSize)
		read, err := state.source.ReadPCMFrames(buffer)
		if err != nil {
			return callbackResult(Error)
		}
		writeUint64(framesRead, read)
		if read == 0 {
			// miniaudio learns that a data source has finished only from MA_AT_END.
			// A Go source that produces nothing is therefore treated as exhausted,
			// which is what lets a sound set its at-end flag and fire its end
			// callback instead of looping forever on silence.
			return callbackResult(AtEnd)
		}
		return callbackResult(Success)
	})

	dataSourceSeekPtr = purego.NewCallback(func(token uintptr, frameIndex uint64) uintptr {
		state, ok := loadDataSource(token)
		if !ok {
			return callbackResult(Error)
		}
		if err := state.source.SeekToPCMFrame(frameIndex); err != nil {
			return callbackResult(Error)
		}
		return callbackResult(Success)
	})

	dataSourceFormatPtr = purego.NewCallback(func(token uintptr, formatOut unsafe.Pointer, channelsOut unsafe.Pointer, rateOut unsafe.Pointer, channelMap unsafe.Pointer, channelMapCap uintptr) uintptr {
		state, ok := loadDataSource(token)
		if !ok {
			return callbackResult(Error)
		}

		if formatOut != nil {
			*(*Format)(formatOut) = state.format
		}
		if channelsOut != nil {
			*(*uint32)(channelsOut) = state.channels
		}
		if rateOut != nil {
			if _, _, rate, err := state.source.DataFormat(); err == nil {
				*(*uint32)(rateOut) = rate
			}
		}
		if channelMap != nil {
			state.lib.bindings.maChannelMapInitStandard(StandardChannelMapDefault, (*uint8)(channelMap), channelMapCap, state.channels)
		}
		return callbackResult(Success)
	})

	dataSourceCursorPtr = purego.NewCallback(func(token uintptr, cursorOut unsafe.Pointer) uintptr {
		state, ok := loadDataSource(token)
		if !ok {
			return callbackResult(Error)
		}
		cursor, err := state.source.CursorInPCMFrames()
		if err != nil {
			return callbackResult(NotImplemented)
		}
		writeUint64(cursorOut, cursor)
		return callbackResult(Success)
	})

	dataSourceLengthPtr = purego.NewCallback(func(token uintptr, lengthOut unsafe.Pointer) uintptr {
		state, ok := loadDataSource(token)
		if !ok {
			return callbackResult(Error)
		}
		length, err := state.source.LengthInPCMFrames()
		if err != nil {
			return callbackResult(NotImplemented)
		}
		writeUint64(lengthOut, length)
		return callbackResult(Success)
	})

	dataSourceLoopingPtr = purego.NewCallback(func(token uintptr, isLooping uint32) uintptr {
		state, ok := loadDataSource(token)
		if !ok {
			return callbackResult(Error)
		}
		if err := state.source.SetLooping(isLooping != 0); err != nil {
			return callbackResult(NotImplemented)
		}
		return callbackResult(Success)
	})
)

func loadDataSource(token uintptr) (*dataSourceState, bool) {
	value, ok := dataSources.Load(token)
	if !ok {
		return nil, false
	}
	return value.(*dataSourceState), true
}

func writeUint64(ptr unsafe.Pointer, value uint64) {
	if ptr != nil {
		*(*uint64)(ptr) = value
	}
}

func frameSizeOf(format Format, channels uint32) int {
	return int(BytesPerSample(format)) * int(channels)
}

// CustomDataSource is a Go DataSource that miniaudio drives through the bridge.
type CustomDataSource struct {
	lib       *Library
	handle    *dataSourceHandle
	token     uintptr
	format    Format
	channels  uint32
	frameSize int
}

// nativeDataSourceHandle implements nativeDataSource, so a node graph reads this
// source through the bridge object it already owns instead of registering a
// second one.
func (d *CustomDataSource) nativeDataSourceHandle() *dataSourceHandle {
	if d == nil {
		return nil
	}
	return d.handle
}

// NewCustomDataSource registers source with miniaudio so it can be consumed as
// a data source (for example by a later node graph).
func (lib *Library) NewCustomDataSource(source DataSource) (*CustomDataSource, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("mago: nil data source")
	}

	format, channels, _, err := source.DataFormat()
	if err != nil {
		return nil, fmt.Errorf("mago: read data source format: %w", err)
	}
	frameSize := frameSizeOf(format, channels)
	if frameSize == 0 {
		return nil, fmt.Errorf("mago: data source reported format %d with %d channels", format, channels)
	}

	token := uintptr(dataSourceSeq.Add(1))
	dataSources.Store(token, &dataSourceState{source: source, lib: lib, format: format, channels: channels})

	var handle *dataSourceHandle
	result := lib.bindings.magoDataSourceInit(
		dataSourceReadPtr, dataSourceSeekPtr, dataSourceFormatPtr,
		dataSourceCursorPtr, dataSourceLengthPtr, dataSourceLoopingPtr,
		token, &handle,
	)
	if result != Success {
		dataSources.Delete(token)
		return nil, lib.resultError("mago_data_source_init", result)
	}

	return &CustomDataSource{
		lib:       lib,
		handle:    handle,
		token:     token,
		format:    format,
		channels:  channels,
		frameSize: frameSize,
	}, nil
}

// ReadPCMFrames fills out with interleaved frames and reports how many frames
// were written.
func (d *CustomDataSource) ReadPCMFrames(out []byte) (uint64, error) {
	if d == nil || d.handle == nil {
		return 0, fmt.Errorf("mago: nil custom data source")
	}
	if err := d.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}

	var read uint64
	result := d.lib.bindings.maDataSourceReadPCMFrames(d.handle, unsafe.Pointer(&out[0]), uint64(len(out)/d.frameSize), &read)
	if result == AtEnd {
		return read, nil
	}
	if result != Success {
		return read, d.lib.resultError("ma_data_source_read_pcm_frames", result)
	}
	return read, nil
}

// SeekToPCMFrame moves the source's cursor.
func (d *CustomDataSource) SeekToPCMFrame(frameIndex uint64) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil custom data source")
	}
	return d.lib.resultError("ma_data_source_seek_to_pcm_frame",
		d.lib.bindings.maDataSourceSeekToPCMFrame(d.handle, frameIndex))
}

// DataFormat reports the source's output format.
func (d *CustomDataSource) DataFormat() (Format, uint32, uint32, error) {
	if d == nil || d.handle == nil {
		return FormatUnknown, 0, 0, fmt.Errorf("mago: nil custom data source")
	}

	var format Format
	var channels, rate uint32
	if result := d.lib.bindings.maDataSourceGetDataFormat(d.handle, &format, &channels, &rate, nil, 0); result != Success {
		return FormatUnknown, 0, 0, d.lib.resultError("ma_data_source_get_data_format", result)
	}
	return format, channels, rate, nil
}

// CursorInPCMFrames reports the current position.
func (d *CustomDataSource) CursorInPCMFrames() (uint64, error) {
	return d.readFrameCount("ma_data_source_get_cursor_in_pcm_frames", d.lib.bindings.maDataSourceGetCursorInPCMFrames)
}

// LengthInPCMFrames reports the total length, when known.
func (d *CustomDataSource) LengthInPCMFrames() (uint64, error) {
	return d.readFrameCount("ma_data_source_get_length_in_pcm_frames", d.lib.bindings.maDataSourceGetLengthInPCMFrames)
}

func (d *CustomDataSource) readFrameCount(op string, fn func(*dataSourceHandle, *uint64) Result) (uint64, error) {
	if d == nil || d.handle == nil {
		return 0, fmt.Errorf("mago: nil custom data source")
	}

	var value uint64
	if result := fn(d.handle, &value); result != Success {
		return 0, d.lib.resultError(op, result)
	}
	return value, nil
}

// SetLooping configures looping on the underlying source.
func (d *CustomDataSource) SetLooping(looping bool) error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("mago: nil custom data source")
	}
	return d.lib.resultError("ma_data_source_set_looping",
		d.lib.bindings.maDataSourceSetLooping(d.handle, boolToBool32(looping)))
}

// Close unregisters the source.
func (d *CustomDataSource) Close() error {
	if d == nil || d.handle == nil {
		return nil
	}
	if err := d.lib.ensureOpen(); err != nil {
		return err
	}

	d.lib.bindings.magoDataSourceUninit(d.handle)
	d.handle = nil
	dataSources.Delete(d.token)
	d.token = 0
	return nil
}
