//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"unsafe"
)

// This file adapts the existing mago sources to the DataSource interface. The
// byte-oriented ReadPCMFrames renders the object's typed read into the raw PCM
// layout miniaudio expects; the typed reads remain the efficient path.

// nativeDataSource is implemented by the objects that miniaudio already treats
// as data sources. A node graph or a sound can read one directly instead of
// bouncing every frame through Go callbacks, which is faster and keeps
// miniaudio's own seeking and looping behaviour.
type nativeDataSource interface {
	DataSource
	nativeDataSourceHandle() *dataSourceHandle
}

// resolveDataSource turns any DataSource into the ma_data_source* a node graph or
// a sound needs. Sources miniaudio already understands are used directly; any
// other Go source is registered through the bridge, and the returned wrapper is
// then owned by the caller and must be closed after whatever reads it.
//
// miniaudio only mixes f32, so a source reporting anything else is rejected here
// rather than failing later with an opaque result code.
func (lib *Library) resolveDataSource(source DataSource) (*dataSourceHandle, *CustomDataSource, error) {
	if source == nil {
		return nil, nil, errors.New("mago: nil data source")
	}

	format, _, _, err := source.DataFormat()
	if err != nil {
		return nil, nil, fmt.Errorf("mago: read data source format: %w", err)
	}
	if format != FormatF32 {
		return nil, nil, fmt.Errorf("mago: miniaudio only consumes f32 data sources, got format %d", format)
	}

	if native, ok := source.(nativeDataSource); ok {
		if handle := native.nativeDataSourceHandle(); handle != nil {
			return handle, nil, nil
		}
		return nil, nil, errors.New("mago: data source is not initialised")
	}

	wrapper, err := lib.NewCustomDataSource(source)
	if err != nil {
		return nil, nil, err
	}
	return wrapper.handle, wrapper, nil
}

// nativeDataSourceHandle implements nativeDataSource for Decoder. ma_decoder
// begins with ma_data_source_base, so the object pointer is also a data source.
func (d *Decoder) nativeDataSourceHandle() *dataSourceHandle {
	if d == nil || d.handle == nil {
		return nil
	}
	return (*dataSourceHandle)(unsafe.Pointer(d.handle))
}

// nativeDataSourceHandle implements nativeDataSource for AudioBuffer.
func (b *AudioBuffer) nativeDataSourceHandle() *dataSourceHandle {
	if b == nil || b.handle == nil {
		return nil
	}
	return (*dataSourceHandle)(unsafe.Pointer(b.handle))
}

// nativeDataSourceHandle implements nativeDataSource for Waveform.
func (w *Waveform) nativeDataSourceHandle() *dataSourceHandle {
	if w == nil || w.handle == nil {
		return nil
	}
	return (*dataSourceHandle)(unsafe.Pointer(w.handle))
}

// nativeDataSourceHandle implements nativeDataSource for Noise.
func (n *Noise) nativeDataSourceHandle() *dataSourceHandle {
	if n == nil || n.handle == nil {
		return nil
	}
	return (*dataSourceHandle)(unsafe.Pointer(n.handle))
}

func sourceFrameSize(format Format, channels uint32) (int, error) {
	size := frameSizeOf(format, channels)
	if size == 0 {
		return 0, fmt.Errorf("mago: unsupported source format %d with %d channels", format, channels)
	}
	return size, nil
}

func writeF32Bytes(out []byte, values []float32) {
	for i, value := range values {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(value))
	}
}

func writeS16Bytes(out []byte, values []int16) {
	for i, value := range values {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(value))
	}
}

// ReadPCMFrames implements DataSource for Decoder.
func (d *Decoder) ReadPCMFrames(out []byte) (uint64, error) {
	if d == nil || d.handle == nil {
		return 0, fmt.Errorf("mago: nil decoder")
	}
	frameSize, err := sourceFrameSize(d.format, d.channels)
	if err != nil {
		return 0, err
	}
	frames := len(out) / frameSize
	if frames == 0 {
		return 0, nil
	}

	switch d.format {
	case FormatF32:
		buffer := make([]float32, frames*int(d.channels))
		read, err := d.ReadF32(buffer)
		if err != nil {
			return read, err
		}
		writeF32Bytes(out, buffer[:int(read)*int(d.channels)])
		return read, nil
	case FormatS16:
		buffer := make([]int16, frames*int(d.channels))
		read, err := d.ReadS16(buffer)
		if err != nil {
			return read, err
		}
		writeS16Bytes(out, buffer[:int(read)*int(d.channels)])
		return read, nil
	default:
		return 0, fmt.Errorf("mago: decoder format %d is not supported by ReadPCMFrames", d.format)
	}
}

// SetLooping implements DataSource. Decoder looping is not wired up yet, so this
// accepts the request and has no effect.
func (d *Decoder) SetLooping(bool) error { return nil }

// ReadPCMFrames implements DataSource for AudioBuffer.
func (b *AudioBuffer) ReadPCMFrames(out []byte) (uint64, error) {
	if b == nil || b.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer")
	}
	frameSize, err := sourceFrameSize(b.format, b.channels)
	if err != nil {
		return 0, err
	}
	frames := len(out) / frameSize
	if frames == 0 {
		return 0, nil
	}

	switch b.format {
	case FormatF32:
		buffer := make([]float32, frames*int(b.channels))
		read, err := b.Read(buffer, b.looping)
		if err != nil {
			return read, err
		}
		writeF32Bytes(out, buffer[:int(read)*int(b.channels)])
		return read, nil
	case FormatS16:
		buffer := make([]int16, frames*int(b.channels))
		read, err := b.ReadS16(buffer, b.looping)
		if err != nil {
			return read, err
		}
		writeS16Bytes(out, buffer[:int(read)*int(b.channels)])
		return read, nil
	default:
		return 0, fmt.Errorf("mago: audio buffer format %d is not supported by ReadPCMFrames", b.format)
	}
}

// SetLooping implements DataSource for AudioBuffer.
func (b *AudioBuffer) SetLooping(looping bool) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil audio buffer")
	}
	b.looping = looping
	return nil
}

// DataFormat implements DataSource for AudioBuffer.
func (b *AudioBuffer) DataFormat() (Format, uint32, uint32, error) {
	if b == nil || b.handle == nil {
		return FormatUnknown, 0, 0, fmt.Errorf("mago: nil audio buffer")
	}
	return b.format, b.channels, b.sampleRate, nil
}

// ReadPCMFrames implements DataSource for Waveform.
func (w *Waveform) ReadPCMFrames(out []byte) (uint64, error) {
	if w == nil || w.handle == nil {
		return 0, fmt.Errorf("mago: nil waveform")
	}
	if w.format != FormatF32 {
		return 0, fmt.Errorf("mago: waveform format %d is not supported by ReadPCMFrames", w.format)
	}
	frameSize, err := sourceFrameSize(w.format, w.channels)
	if err != nil {
		return 0, err
	}
	frames := len(out) / frameSize
	if frames == 0 {
		return 0, nil
	}

	buffer := make([]float32, frames*int(w.channels))
	read, err := w.Read(buffer)
	if err != nil {
		return read, err
	}
	writeF32Bytes(out, buffer[:int(read)*int(w.channels)])
	return read, nil
}

// DataFormat implements DataSource for Waveform.
func (w *Waveform) DataFormat() (Format, uint32, uint32, error) {
	if w == nil || w.handle == nil {
		return FormatUnknown, 0, 0, fmt.Errorf("mago: nil waveform")
	}
	return w.format, w.channels, w.sampleRate, nil
}

// CursorInPCMFrames implements DataSource. A waveform is an endless generator, so
// it has no cursor.
func (w *Waveform) CursorInPCMFrames() (uint64, error) {
	return 0, fmt.Errorf("mago: waveform has no cursor")
}

// LengthInPCMFrames implements DataSource. A waveform is an endless generator, so
// it has no length.
func (w *Waveform) LengthInPCMFrames() (uint64, error) {
	return 0, fmt.Errorf("mago: waveform has no length")
}

// SetLooping implements DataSource. A waveform never ends, so looping is a no-op.
func (w *Waveform) SetLooping(bool) error { return nil }

// ReadPCMFrames implements DataSource for Noise.
func (n *Noise) ReadPCMFrames(out []byte) (uint64, error) {
	if n == nil || n.handle == nil {
		return 0, fmt.Errorf("mago: nil noise")
	}
	if n.format != FormatF32 {
		return 0, fmt.Errorf("mago: noise format %d is not supported by ReadPCMFrames", n.format)
	}
	frameSize, err := sourceFrameSize(n.format, n.channels)
	if err != nil {
		return 0, err
	}
	frames := len(out) / frameSize
	if frames == 0 {
		return 0, nil
	}

	buffer := make([]float32, frames*int(n.channels))
	read, err := n.Read(buffer)
	if err != nil {
		return read, err
	}
	writeF32Bytes(out, buffer[:int(read)*int(n.channels)])
	return read, nil
}

// DataFormat implements DataSource for Noise. Noise has no sample rate.
func (n *Noise) DataFormat() (Format, uint32, uint32, error) {
	if n == nil || n.handle == nil {
		return FormatUnknown, 0, 0, fmt.Errorf("mago: nil noise")
	}
	return n.format, n.channels, 0, nil
}

// CursorInPCMFrames implements DataSource. Noise is an endless generator, so it
// has no cursor.
func (n *Noise) CursorInPCMFrames() (uint64, error) {
	return 0, fmt.Errorf("mago: noise has no cursor")
}

// LengthInPCMFrames implements DataSource. Noise is an endless generator, so it
// has no length.
func (n *Noise) LengthInPCMFrames() (uint64, error) {
	return 0, fmt.Errorf("mago: noise has no length")
}

// SeekToPCMFrame implements DataSource. Noise is an endless generator with no
// seek support, so this always reports that seeking is unavailable.
func (n *Noise) SeekToPCMFrame(uint64) error {
	return fmt.Errorf("mago: noise has no seek")
}

// SetLooping implements DataSource. Noise never ends, so looping is a no-op.
func (n *Noise) SetLooping(bool) error { return nil }
