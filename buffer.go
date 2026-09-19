//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// AudioBufferConfig describes an in-memory PCM buffer. When Data is nil,
// miniaudio allocates the storage itself; otherwise it references the memory
// Data points at.
type AudioBufferConfig struct {
	Format       Format
	Channels     uint32
	SampleRate   uint32
	SizeInFrames uint64
	Data         unsafe.Pointer

	// DataRef, when set, is retained by the AudioBuffer so that the memory Data
	// points at cannot be garbage collected while miniaudio references it.
	DataRef any
}

// AudioBuffer is an in-memory PCM buffer that behaves as a data source.
type AudioBuffer struct {
	lib     *Library
	handle  *audioBufferHandle
	dataRef any
}

// NewAudioBuffer creates a buffer referencing config.Data (or allocating its own
// storage when Data is nil).
func (lib *Library) NewAudioBuffer(config AudioBufferConfig) (*AudioBuffer, error) {
	return lib.newAudioBuffer("ma_audio_buffer_init", config, false)
}

// NewAudioBufferCopy creates a buffer that copies config.Data.
func (lib *Library) NewAudioBufferCopy(config AudioBufferConfig) (*AudioBuffer, error) {
	return lib.newAudioBuffer("ma_audio_buffer_init_copy", config, true)
}

func (lib *Library) newAudioBuffer(op string, config AudioBufferConfig, copyData bool) (*AudioBuffer, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	native := audioBufferConfigNative{
		Format:       config.Format,
		Channels:     config.Channels,
		SampleRate:   config.SampleRate,
		SizeInFrames: config.SizeInFrames,
		Data:         config.Data,
	}

	handle := (*audioBufferHandle)(lib.bindings.magoAlloc(magoObjectAudioBuffer))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate audio buffer: out of memory")
	}

	var result Result
	if copyData {
		result = lib.bindings.maAudioBufferInitCopy(&native, handle)
	} else {
		result = lib.bindings.maAudioBufferInit(&native, handle)
	}
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError(op, result)
	}

	return &AudioBuffer{lib: lib, handle: handle, dataRef: config.DataRef}, nil
}

// ReadPCMFrames writes up to frameCount frames into out, returning how many were
// written. Set loop to restart from the beginning at the end of the buffer.
func (b *AudioBuffer) ReadPCMFrames(out unsafe.Pointer, frameCount uint64, loop bool) (uint64, error) {
	if b == nil || b.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return 0, err
	}
	return b.lib.bindings.maAudioBufferReadPCMFrames(b.handle, out, frameCount, boolToBool32(loop)), nil
}

// SeekToPCMFrame moves the read cursor.
func (b *AudioBuffer) SeekToPCMFrame(frameIndex uint64) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	return b.lib.resultError("ma_audio_buffer_seek_to_pcm_frame", b.lib.bindings.maAudioBufferSeekToPCMFrame(b.handle, frameIndex))
}

// Map returns a pointer to the remaining frames and the number available.
func (b *AudioBuffer) Map() (unsafe.Pointer, uint64, error) {
	if b == nil || b.handle == nil {
		return nil, 0, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return nil, 0, err
	}

	var frames unsafe.Pointer
	var frameCount uint64
	if result := b.lib.bindings.maAudioBufferMap(b.handle, &frames, &frameCount); result != Success {
		return nil, 0, b.lib.resultError("ma_audio_buffer_map", result)
	}
	return frames, frameCount, nil
}

// Unmap releases a region returned by Map after frameCount frames were consumed.
// Reaching the end of the buffer is not an error.
func (b *AudioBuffer) Unmap(frameCount uint64) error {
	if b == nil || b.handle == nil {
		return fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}
	result := b.lib.bindings.maAudioBufferUnmap(b.handle, frameCount)
	if result == Success || result == AtEnd {
		return nil
	}
	return b.lib.resultError("ma_audio_buffer_unmap", result)
}

// CursorInPCMFrames reports the current read position.
func (b *AudioBuffer) CursorInPCMFrames() (uint64, error) {
	return b.readFrameCount("ma_audio_buffer_get_cursor_in_pcm_frames", b.lib.bindings.maAudioBufferGetCursorInPCMFrames)
}

// LengthInPCMFrames reports the total length of the buffer.
func (b *AudioBuffer) LengthInPCMFrames() (uint64, error) {
	return b.readFrameCount("ma_audio_buffer_get_length_in_pcm_frames", b.lib.bindings.maAudioBufferGetLengthInPCMFrames)
}

// AvailableFrames reports how many frames remain before the end.
func (b *AudioBuffer) AvailableFrames() (uint64, error) {
	return b.readFrameCount("ma_audio_buffer_get_available_frames", b.lib.bindings.maAudioBufferGetAvailableFrames)
}

func (b *AudioBuffer) readFrameCount(op string, fn func(*audioBufferHandle, *uint64) Result) (uint64, error) {
	if b == nil || b.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return 0, err
	}

	var value uint64
	if result := fn(b.handle, &value); result != Success {
		return 0, b.lib.resultError(op, result)
	}
	return value, nil
}

// Close uninitializes the buffer and frees it, releasing any data miniaudio
// allocated itself.
func (b *AudioBuffer) Close() error {
	if b == nil || b.handle == nil {
		return nil
	}
	if err := b.lib.ensureOpen(); err != nil {
		return err
	}

	b.lib.bindings.maAudioBufferUninit(b.handle)
	b.lib.bindings.magoFree(unsafe.Pointer(b.handle))
	b.handle = nil
	b.dataRef = nil
	return nil
}

// AudioBufferRef is a non-owning view over PCM memory supplied by the caller.
type AudioBufferRef struct {
	lib     *Library
	handle  *audioBufferRefHandle
	dataRef any
}

// NewAudioBufferRef creates a reference over data, which must stay alive for the
// lifetime of the reference.
func (lib *Library) NewAudioBufferRef(format Format, channels uint32, data unsafe.Pointer, sizeInFrames uint64, dataRef ...any) (*AudioBufferRef, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	handle := (*audioBufferRefHandle)(lib.bindings.magoAlloc(magoObjectAudioBufferRef))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate audio buffer ref: out of memory")
	}
	if result := lib.bindings.maAudioBufferRefInit(format, channels, data, sizeInFrames, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_audio_buffer_ref_init", result)
	}

	ref := &AudioBufferRef{lib: lib, handle: handle}
	if len(dataRef) > 0 {
		ref.dataRef = dataRef[0]
	}
	return ref, nil
}

// SetData points the reference at a new region.
func (r *AudioBufferRef) SetData(data unsafe.Pointer, sizeInFrames uint64, dataRef ...any) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	if result := r.lib.bindings.maAudioBufferRefSetData(r.handle, data, sizeInFrames); result != Success {
		return r.lib.resultError("ma_audio_buffer_ref_set_data", result)
	}
	if len(dataRef) > 0 {
		r.dataRef = dataRef[0]
	}
	return nil
}

// ReadPCMFrames writes up to frameCount frames into out.
func (r *AudioBufferRef) ReadPCMFrames(out unsafe.Pointer, frameCount uint64, loop bool) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	return r.lib.bindings.maAudioBufferRefReadPCMFrames(r.handle, out, frameCount, boolToBool32(loop)), nil
}

// SeekToPCMFrame moves the read cursor.
func (r *AudioBufferRef) SeekToPCMFrame(frameIndex uint64) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_audio_buffer_ref_seek_to_pcm_frame", r.lib.bindings.maAudioBufferRefSeekToPCMFrame(r.handle, frameIndex))
}

// Map returns a pointer to the remaining frames and the number available.
func (r *AudioBufferRef) Map() (unsafe.Pointer, uint64, error) {
	if r == nil || r.handle == nil {
		return nil, 0, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return nil, 0, err
	}

	var frames unsafe.Pointer
	var frameCount uint64
	if result := r.lib.bindings.maAudioBufferRefMap(r.handle, &frames, &frameCount); result != Success {
		return nil, 0, r.lib.resultError("ma_audio_buffer_ref_map", result)
	}
	return frames, frameCount, nil
}

// Unmap releases a region returned by Map. Reaching the end is not an error.
func (r *AudioBufferRef) Unmap(frameCount uint64) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	result := r.lib.bindings.maAudioBufferRefUnmap(r.handle, frameCount)
	if result == Success || result == AtEnd {
		return nil
	}
	return r.lib.resultError("ma_audio_buffer_ref_unmap", result)
}

// AtEnd reports whether the reference has consumed all of its frames.
func (r *AudioBufferRef) AtEnd() bool {
	if r == nil || r.handle == nil {
		return true
	}
	return r.lib.bindings.maAudioBufferRefAtEnd(r.handle) != 0
}

// CursorInPCMFrames reports the current read position.
func (r *AudioBufferRef) CursorInPCMFrames() (uint64, error) {
	return r.readFrameCount("ma_audio_buffer_ref_get_cursor_in_pcm_frames", r.lib.bindings.maAudioBufferRefGetCursorInPCMFrames)
}

// LengthInPCMFrames reports the total length of the referenced region.
func (r *AudioBufferRef) LengthInPCMFrames() (uint64, error) {
	return r.readFrameCount("ma_audio_buffer_ref_get_length_in_pcm_frames", r.lib.bindings.maAudioBufferRefGetLengthInPCMFrames)
}

// AvailableFrames reports how many frames remain before the end.
func (r *AudioBufferRef) AvailableFrames() (uint64, error) {
	return r.readFrameCount("ma_audio_buffer_ref_get_available_frames", r.lib.bindings.maAudioBufferRefGetAvailableFrames)
}

func (r *AudioBufferRef) readFrameCount(op string, fn func(*audioBufferRefHandle, *uint64) Result) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}

	var value uint64
	if result := fn(r.handle, &value); result != Success {
		return 0, r.lib.resultError(op, result)
	}
	return value, nil
}

// Close uninitializes the reference. The referenced memory is not freed.
func (r *AudioBufferRef) Close() error {
	if r == nil || r.handle == nil {
		return nil
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}

	r.lib.bindings.maAudioBufferRefUninit(r.handle)
	r.lib.bindings.magoFree(unsafe.Pointer(r.handle))
	r.handle = nil
	r.dataRef = nil
	return nil
}
