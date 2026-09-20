//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// AudioBufferConfig describes an in-memory PCM buffer. When Data and DataF32
// are nil/empty, miniaudio allocates the storage itself; otherwise it references
// the memory backing the slice.
type AudioBufferConfig struct {
	Format       Format
	Channels     uint32
	SampleRate   uint32
	SizeInFrames uint64
	Data         []byte
	DataF32      []float32
}

// AudioBuffer is an in-memory PCM buffer that behaves as a data source.
type AudioBuffer struct {
	lib        *Library
	handle     *audioBufferHandle
	format     Format
	channels   uint32
	sampleRate uint32
	dataRef    any
	looping    bool
}

// NewAudioBuffer creates a buffer referencing config.Data or config.DataF32
// (or allocating its own storage when both are empty).
func (lib *Library) NewAudioBuffer(config AudioBufferConfig) (*AudioBuffer, error) {
	return lib.newAudioBuffer("ma_audio_buffer_init", config, false)
}

// NewAudioBufferCopy creates a buffer that copies config.Data or config.DataF32.
func (lib *Library) NewAudioBufferCopy(config AudioBufferConfig) (*AudioBuffer, error) {
	return lib.newAudioBuffer("ma_audio_buffer_init_copy", config, true)
}

func (lib *Library) newAudioBuffer(op string, config AudioBufferConfig, copyData bool) (*AudioBuffer, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	var dataPtr unsafe.Pointer
	var dataRef any
	sizeInFrames := config.SizeInFrames

	if len(config.DataF32) > 0 {
		if config.Channels == 0 || len(config.DataF32)%int(config.Channels) != 0 {
			return nil, ErrInvalidSliceLength
		}
		dataPtr = unsafe.Pointer(&config.DataF32[0])
		dataRef = config.DataF32
		if sizeInFrames == 0 {
			sizeInFrames = uint64(len(config.DataF32) / int(config.Channels))
		}
	} else if len(config.Data) > 0 {
		bytesPerFrame := uint64(config.Channels) * uint64(BytesPerSample(config.Format))
		if bytesPerFrame > 0 && uint64(len(config.Data))%bytesPerFrame != 0 {
			return nil, ErrInvalidSliceLength
		}
		dataPtr = unsafe.Pointer(&config.Data[0])
		dataRef = config.Data
		if sizeInFrames == 0 && bytesPerFrame > 0 {
			sizeInFrames = uint64(len(config.Data)) / bytesPerFrame
		}
	}

	native := audioBufferConfigNative{
		Format:       config.Format,
		Channels:     config.Channels,
		SampleRate:   config.SampleRate,
		SizeInFrames: sizeInFrames,
		Data:         dataPtr,
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

	return &AudioBuffer{
		lib:        lib,
		handle:     handle,
		format:     config.Format,
		channels:   config.Channels,
		sampleRate: config.SampleRate,
		dataRef:    dataRef,
	}, nil
}

// Read fills out with audio frames, returning the number of frames read.
// out must contain an exact multiple of the configured channel count.
func (b *AudioBuffer) Read(out []float32, loop bool) (uint64, error) {
	if b == nil || b.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if b.channels == 0 || len(out)%int(b.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCount := uint64(len(out) / int(b.channels))
	read := b.lib.bindings.maAudioBufferReadPCMFrames(b.handle, unsafe.Pointer(&out[0]), frameCount, boolToBool32(loop))
	return read, nil
}

// ReadS16 fills out with int16 audio frames, returning the number of frames read.
// out must contain an exact multiple of the configured channel count.
func (b *AudioBuffer) ReadS16(out []int16, loop bool) (uint64, error) {
	if b == nil || b.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if b.channels == 0 || len(out)%int(b.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCount := uint64(len(out) / int(b.channels))
	read := b.lib.bindings.maAudioBufferReadPCMFrames(b.handle, unsafe.Pointer(&out[0]), frameCount, boolToBool32(loop))
	return read, nil
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

// MapF32 returns a direct slice over the remaining float32 frames in the buffer.
func (b *AudioBuffer) MapF32() ([]float32, error) {
	if b == nil || b.handle == nil {
		return nil, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return nil, err
	}

	frameCount, err := b.AvailableFrames()
	if err != nil {
		return nil, err
	}
	if frameCount == 0 || b.channels == 0 {
		return nil, nil
	}

	var frames unsafe.Pointer
	if result := b.lib.bindings.maAudioBufferMap(b.handle, &frames, &frameCount); result != Success {
		return nil, b.lib.resultError("ma_audio_buffer_map", result)
	}
	if frames == nil || frameCount == 0 {
		return nil, nil
	}
	return unsafe.Slice((*float32)(frames), int(frameCount*uint64(b.channels))), nil
}

// MapBytes returns a direct slice over the remaining bytes in the buffer.
func (b *AudioBuffer) MapBytes() ([]byte, error) {
	if b == nil || b.handle == nil {
		return nil, fmt.Errorf("mago: nil audio buffer")
	}
	if err := b.lib.ensureOpen(); err != nil {
		return nil, err
	}

	frameCount, err := b.AvailableFrames()
	if err != nil {
		return nil, err
	}
	bytesPerFrame := uint64(b.channels) * uint64(BytesPerSample(b.format))
	if frameCount == 0 || bytesPerFrame == 0 {
		return nil, nil
	}

	var frames unsafe.Pointer
	if result := b.lib.bindings.maAudioBufferMap(b.handle, &frames, &frameCount); result != Success {
		return nil, b.lib.resultError("ma_audio_buffer_map", result)
	}
	if frames == nil || frameCount == 0 {
		return nil, nil
	}
	return unsafe.Slice((*byte)(frames), int(frameCount*bytesPerFrame)), nil
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
	lib      *Library
	handle   *audioBufferRefHandle
	format   Format
	channels uint32
	dataRef  any
}

// NewAudioBufferRefF32 creates a non-owning reference over a float32 sample slice.
func (lib *Library) NewAudioBufferRefF32(channels uint32, data []float32) (*AudioBufferRef, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if channels == 0 || len(data)%int(channels) != 0 {
		return nil, ErrInvalidSliceLength
	}
	frameCount := uint64(len(data) / int(channels))

	handle := (*audioBufferRefHandle)(lib.bindings.magoAlloc(magoObjectAudioBufferRef))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate audio buffer ref: out of memory")
	}

	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	if result := lib.bindings.maAudioBufferRefInit(FormatF32, channels, dataPtr, frameCount, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_audio_buffer_ref_init", result)
	}

	return &AudioBufferRef{
		lib:      lib,
		handle:   handle,
		format:   FormatF32,
		channels: channels,
		dataRef:  data,
	}, nil
}

// NewAudioBufferRef creates a non-owning reference over a byte slice formatted as specified.
func (lib *Library) NewAudioBufferRef(format Format, channels uint32, data []byte) (*AudioBufferRef, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	bytesPerFrame := uint64(channels) * uint64(BytesPerSample(format))
	if bytesPerFrame == 0 || uint64(len(data))%bytesPerFrame != 0 {
		return nil, ErrInvalidSliceLength
	}
	frameCount := uint64(len(data)) / bytesPerFrame

	handle := (*audioBufferRefHandle)(lib.bindings.magoAlloc(magoObjectAudioBufferRef))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate audio buffer ref: out of memory")
	}

	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	if result := lib.bindings.maAudioBufferRefInit(format, channels, dataPtr, frameCount, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_audio_buffer_ref_init", result)
	}

	return &AudioBufferRef{
		lib:      lib,
		handle:   handle,
		format:   format,
		channels: channels,
		dataRef:  data,
	}, nil
}

// SetDataF32 points the reference at a new float32 slice.
func (r *AudioBufferRef) SetDataF32(data []float32) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	if r.channels == 0 || len(data)%int(r.channels) != 0 {
		return ErrInvalidSliceLength
	}
	frameCount := uint64(len(data) / int(r.channels))
	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	if result := r.lib.bindings.maAudioBufferRefSetData(r.handle, dataPtr, frameCount); result != Success {
		return r.lib.resultError("ma_audio_buffer_ref_set_data", result)
	}
	r.dataRef = data
	return nil
}

// SetData points the reference at a new byte slice.
func (r *AudioBufferRef) SetData(data []byte) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	bytesPerFrame := uint64(r.channels) * uint64(BytesPerSample(r.format))
	if bytesPerFrame == 0 || uint64(len(data))%bytesPerFrame != 0 {
		return ErrInvalidSliceLength
	}
	frameCount := uint64(len(data)) / bytesPerFrame
	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}
	if result := r.lib.bindings.maAudioBufferRefSetData(r.handle, dataPtr, frameCount); result != Success {
		return r.lib.resultError("ma_audio_buffer_ref_set_data", result)
	}
	r.dataRef = data
	return nil
}

// Read writes up to len(out)/channels frames into out, returning the number of frames read.
func (r *AudioBufferRef) Read(out []float32, loop bool) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if r.channels == 0 || len(out)%int(r.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCount := uint64(len(out) / int(r.channels))
	read := r.lib.bindings.maAudioBufferRefReadPCMFrames(r.handle, unsafe.Pointer(&out[0]), frameCount, boolToBool32(loop))
	return read, nil
}

// ReadS16 writes up to len(out)/channels frames into out, returning the number of frames read.
func (r *AudioBufferRef) ReadS16(out []int16, loop bool) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if r.channels == 0 || len(out)%int(r.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCount := uint64(len(out) / int(r.channels))
	read := r.lib.bindings.maAudioBufferRefReadPCMFrames(r.handle, unsafe.Pointer(&out[0]), frameCount, boolToBool32(loop))
	return read, nil
}

// MapF32 returns a direct slice over the remaining float32 frames in the referenced buffer.
func (r *AudioBufferRef) MapF32() ([]float32, error) {
	if r == nil || r.handle == nil {
		return nil, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return nil, err
	}

	frameCount, err := r.AvailableFrames()
	if err != nil {
		return nil, err
	}
	if frameCount == 0 || r.channels == 0 {
		return nil, nil
	}

	var frames unsafe.Pointer
	if result := r.lib.bindings.maAudioBufferRefMap(r.handle, &frames, &frameCount); result != Success {
		return nil, r.lib.resultError("ma_audio_buffer_ref_map", result)
	}
	if frames == nil || frameCount == 0 {
		return nil, nil
	}
	return unsafe.Slice((*float32)(frames), int(frameCount*uint64(r.channels))), nil
}

// MapBytes returns a direct slice over the remaining bytes in the referenced buffer.
func (r *AudioBufferRef) MapBytes() ([]byte, error) {
	if r == nil || r.handle == nil {
		return nil, fmt.Errorf("mago: nil audio buffer ref")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return nil, err
	}

	frameCount, err := r.AvailableFrames()
	if err != nil {
		return nil, err
	}
	bytesPerFrame := uint64(r.channels) * uint64(BytesPerSample(r.format))
	if frameCount == 0 || bytesPerFrame == 0 {
		return nil, nil
	}

	var frames unsafe.Pointer
	if result := r.lib.bindings.maAudioBufferRefMap(r.handle, &frames, &frameCount); result != Success {
		return nil, r.lib.resultError("ma_audio_buffer_ref_map", result)
	}
	if frames == nil || frameCount == 0 {
		return nil, nil
	}
	return unsafe.Slice((*byte)(frames), int(frameCount*bytesPerFrame)), nil
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

// RingBuffer is a byte-oriented ring buffer.
type RingBuffer struct {
	lib    *Library
	handle *ringBufferHandle
}

// NewRingBuffer creates a byte ring buffer of the given size.
func (lib *Library) NewRingBuffer(bufferSizeInBytes uint) (*RingBuffer, error) {
	return lib.newRingBuffer("ma_rb_init", func(handle *ringBufferHandle) Result {
		return lib.bindings.maRBInit(uintptr(bufferSizeInBytes), nil, nil, handle)
	})
}

// NewRingBufferEx creates a ring buffer with explicit sub-buffer geometry.
func (lib *Library) NewRingBufferEx(subbufferSizeInBytes, subbufferCount, subbufferStrideInBytes uint) (*RingBuffer, error) {
	return lib.newRingBuffer("ma_rb_init_ex", func(handle *ringBufferHandle) Result {
		return lib.bindings.maRBInitEx(uintptr(subbufferSizeInBytes), uintptr(subbufferCount), uintptr(subbufferStrideInBytes), nil, nil, handle)
	})
}

func (lib *Library) newRingBuffer(op string, init func(*ringBufferHandle) Result) (*RingBuffer, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	handle := (*ringBufferHandle)(lib.bindings.magoAlloc(magoObjectRingBuffer))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate ring buffer: out of memory")
	}
	if result := init(handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError(op, result)
	}

	return &RingBuffer{lib: lib, handle: handle}, nil
}

// AcquireRead returns the next readable region as a byte slice, requesting up to sizeInBytes.
// The returned slice length may be smaller because the region is always contiguous.
func (r *RingBuffer) AcquireRead(sizeInBytes uint) ([]byte, error) {
	ptr, sz, err := r.acquire("ma_rb_acquire_read", sizeInBytes, r.lib.bindings.maRBAcquireRead)
	if err != nil {
		return nil, err
	}
	if ptr == nil || sz == 0 {
		return nil, nil
	}
	return unsafe.Slice((*byte)(ptr), int(sz)), nil
}

// AcquireWrite returns the next writable region as a byte slice, requesting up to sizeInBytes.
// The returned slice length may be smaller because the region is always contiguous.
func (r *RingBuffer) AcquireWrite(sizeInBytes uint) ([]byte, error) {
	ptr, sz, err := r.acquire("ma_rb_acquire_write", sizeInBytes, r.lib.bindings.maRBAcquireWrite)
	if err != nil {
		return nil, err
	}
	if ptr == nil || sz == 0 {
		return nil, nil
	}
	return unsafe.Slice((*byte)(ptr), int(sz)), nil
}

func (r *RingBuffer) acquire(op string, sizeInBytes uint, fn func(*ringBufferHandle, *uintptr, *unsafe.Pointer) Result) (unsafe.Pointer, uint, error) {
	if r == nil || r.handle == nil {
		return nil, 0, fmt.Errorf("mago: nil ring buffer")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return nil, 0, err
	}

	size := uintptr(sizeInBytes)
	var buffer unsafe.Pointer
	if result := fn(r.handle, &size, &buffer); result != Success {
		return nil, 0, r.lib.resultError(op, result)
	}
	return buffer, uint(size), nil
}

// CommitRead marks sizeInBytes as consumed.
func (r *RingBuffer) CommitRead(sizeInBytes uint) error {
	return r.commit("ma_rb_commit_read", func(handle *ringBufferHandle) Result {
		return r.lib.bindings.maRBCommitRead(handle, uintptr(sizeInBytes))
	})
}

// CommitWrite marks sizeInBytes as written.
func (r *RingBuffer) CommitWrite(sizeInBytes uint) error {
	return r.commit("ma_rb_commit_write", func(handle *ringBufferHandle) Result {
		return r.lib.bindings.maRBCommitWrite(handle, uintptr(sizeInBytes))
	})
}

func (r *RingBuffer) commit(op string, fn func(*ringBufferHandle) Result) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil ring buffer")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError(op, fn(r.handle))
}

// SeekRead advances the read pointer by offsetInBytes.
func (r *RingBuffer) SeekRead(offsetInBytes uint) error {
	return r.commit("ma_rb_seek_read", func(handle *ringBufferHandle) Result {
		return r.lib.bindings.maRBSeekRead(handle, uintptr(offsetInBytes))
	})
}

// SeekWrite advances the write pointer by offsetInBytes.
func (r *RingBuffer) SeekWrite(offsetInBytes uint) error {
	return r.commit("ma_rb_seek_write", func(handle *ringBufferHandle) Result {
		return r.lib.bindings.maRBSeekWrite(handle, uintptr(offsetInBytes))
	})
}

// Reset empties the buffer.
func (r *RingBuffer) Reset() {
	if r == nil || r.handle == nil {
		return
	}
	r.lib.bindings.maRBReset(r.handle)
}

// PointerDistance reports the bytes readable before the read pointer catches the
// write pointer.
func (r *RingBuffer) PointerDistance() int32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maRBPointerDistance(r.handle)
}

// AvailableRead reports how many bytes can be read.
func (r *RingBuffer) AvailableRead() uint32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maRBAvailableRead(r.handle)
}

// AvailableWrite reports how many bytes can be written.
func (r *RingBuffer) AvailableWrite() uint32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maRBAvailableWrite(r.handle)
}

// Close uninitializes the ring buffer and frees it.
func (r *RingBuffer) Close() error {
	if r == nil || r.handle == nil {
		return nil
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}

	r.lib.bindings.maRBUninit(r.handle)
	r.lib.bindings.magoFree(unsafe.Pointer(r.handle))
	r.handle = nil
	return nil
}

// PCMRingBuffer is a frame-oriented ring buffer that also tracks the sample
// format, channel count and sample rate of the data flowing through it.
type PCMRingBuffer struct {
	lib    *Library
	handle *pcmRingBufferHandle
}

// NewPCMRingBuffer creates a PCM ring buffer of the given size in frames.
func (lib *Library) NewPCMRingBuffer(format Format, channels, bufferSizeInFrames uint32) (*PCMRingBuffer, error) {
	return lib.newPCMRingBuffer("ma_pcm_rb_init", func(handle *pcmRingBufferHandle) Result {
		return lib.bindings.maPCMRBInit(format, channels, bufferSizeInFrames, nil, nil, handle)
	})
}

// NewPCMRingBufferEx creates a PCM ring buffer with explicit sub-buffer geometry.
func (lib *Library) NewPCMRingBufferEx(format Format, channels, subbufferSizeInFrames, subbufferCount, subbufferStrideInFrames uint32) (*PCMRingBuffer, error) {
	return lib.newPCMRingBuffer("ma_pcm_rb_init_ex", func(handle *pcmRingBufferHandle) Result {
		return lib.bindings.maPCMRBInitEx(format, channels, subbufferSizeInFrames, subbufferCount, subbufferStrideInFrames, nil, nil, handle)
	})
}

func (lib *Library) newPCMRingBuffer(op string, init func(*pcmRingBufferHandle) Result) (*PCMRingBuffer, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	handle := (*pcmRingBufferHandle)(lib.bindings.magoAlloc(magoObjectPCMRingBuffer))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate PCM ring buffer: out of memory")
	}
	if result := init(handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError(op, result)
	}

	return &PCMRingBuffer{lib: lib, handle: handle}, nil
}

// AcquireRead returns the next readable region as an interleaved float32 slice, requesting up to sizeInFrames.
func (r *PCMRingBuffer) AcquireRead(sizeInFrames uint32) ([]float32, error) {
	ptr, frames, err := r.acquire("ma_pcm_rb_acquire_read", sizeInFrames, r.lib.bindings.maPCMRBAcquireRead)
	if err != nil {
		return nil, err
	}
	channels := r.Channels()
	if ptr == nil || frames == 0 || channels == 0 {
		return nil, nil
	}
	return unsafe.Slice((*float32)(ptr), int(frames*channels)), nil
}

// AcquireWrite returns the next writable region as an interleaved float32 slice, requesting up to sizeInFrames.
func (r *PCMRingBuffer) AcquireWrite(sizeInFrames uint32) ([]float32, error) {
	ptr, frames, err := r.acquire("ma_pcm_rb_acquire_write", sizeInFrames, r.lib.bindings.maPCMRBAcquireWrite)
	if err != nil {
		return nil, err
	}
	channels := r.Channels()
	if ptr == nil || frames == 0 || channels == 0 {
		return nil, nil
	}
	return unsafe.Slice((*float32)(ptr), int(frames*channels)), nil
}

func (r *PCMRingBuffer) acquire(op string, sizeInFrames uint32, fn func(*pcmRingBufferHandle, *uint32, *unsafe.Pointer) Result) (unsafe.Pointer, uint32, error) {
	if r == nil || r.handle == nil {
		return nil, 0, fmt.Errorf("mago: nil PCM ring buffer")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return nil, 0, err
	}

	size := sizeInFrames
	var buffer unsafe.Pointer
	if result := fn(r.handle, &size, &buffer); result != Success {
		return nil, 0, r.lib.resultError(op, result)
	}
	return buffer, size, nil
}

// CommitRead marks sizeInFrames as consumed.
func (r *PCMRingBuffer) CommitRead(sizeInFrames uint32) error {
	return r.commit("ma_pcm_rb_commit_read", func(handle *pcmRingBufferHandle) Result {
		return r.lib.bindings.maPCMRBCommitRead(handle, sizeInFrames)
	})
}

// CommitWrite marks sizeInFrames as written.
func (r *PCMRingBuffer) CommitWrite(sizeInFrames uint32) error {
	return r.commit("ma_pcm_rb_commit_write", func(handle *pcmRingBufferHandle) Result {
		return r.lib.bindings.maPCMRBCommitWrite(handle, sizeInFrames)
	})
}

func (r *PCMRingBuffer) commit(op string, fn func(*pcmRingBufferHandle) Result) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil PCM ring buffer")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError(op, fn(r.handle))
}

// SeekRead advances the read pointer by offsetInFrames.
func (r *PCMRingBuffer) SeekRead(offsetInFrames uint32) error {
	return r.commit("ma_pcm_rb_seek_read", func(handle *pcmRingBufferHandle) Result {
		return r.lib.bindings.maPCMRBSeekRead(handle, offsetInFrames)
	})
}

// SeekWrite advances the write pointer by offsetInFrames.
func (r *PCMRingBuffer) SeekWrite(offsetInFrames uint32) error {
	return r.commit("ma_pcm_rb_seek_write", func(handle *pcmRingBufferHandle) Result {
		return r.lib.bindings.maPCMRBSeekWrite(handle, offsetInFrames)
	})
}

// Reset empties the buffer.
func (r *PCMRingBuffer) Reset() {
	if r == nil || r.handle == nil {
		return
	}
	r.lib.bindings.maPCMRBReset(r.handle)
}

// PointerDistance reports the frames readable before the read pointer catches
// the write pointer.
func (r *PCMRingBuffer) PointerDistance() int32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maPCMRBPointerDistance(r.handle)
}

// AvailableRead reports how many frames can be read.
func (r *PCMRingBuffer) AvailableRead() uint32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maPCMRBAvailableRead(r.handle)
}

// AvailableWrite reports how many frames can be written.
func (r *PCMRingBuffer) AvailableWrite() uint32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maPCMRBAvailableWrite(r.handle)
}

// Format reports the sample format the buffer was created with.
func (r *PCMRingBuffer) Format() Format {
	if r == nil || r.handle == nil {
		return FormatUnknown
	}
	return r.lib.bindings.maPCMRBGetFormat(r.handle)
}

// Channels reports the channel count the buffer was created with.
func (r *PCMRingBuffer) Channels() uint32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maPCMRBGetChannels(r.handle)
}

// SampleRate reports the sample rate associated with the buffer.
func (r *PCMRingBuffer) SampleRate() uint32 {
	if r == nil || r.handle == nil {
		return 0
	}
	return r.lib.bindings.maPCMRBGetSampleRate(r.handle)
}

// Close uninitializes the ring buffer and frees it.
func (r *PCMRingBuffer) Close() error {
	if r == nil || r.handle == nil {
		return nil
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}

	r.lib.bindings.maPCMRBUninit(r.handle)
	r.lib.bindings.magoFree(unsafe.Pointer(r.handle))
	r.handle = nil
	return nil
}
