# Safe Slice API Design: Eliminating `unsafe.Pointer` from the Public Interface

**Date:** 2026-09-19  
**Status:** Approved  
**Author:** Antigravity / Andrew Montgomery  

---

## 1. Background & Motivation

`mago` is a pure-Go binding for `miniaudio.h` that loads precompiled native binaries via `purego`. While the root package avoids CGO at build time, several of its early public interfaces mirrored miniaudio's raw C function signatures directly—demanding that callers pass `unsafe.Pointer(&buf[0])` along with explicit `frameCount` parameters.

Requiring `unsafe.Pointer` in the public API creates multiple friction points:
1. **Developer Experience**: Callers must `import "unsafe"` and perform pointer math or conversions (`unsafe.Pointer(&slice[0])`, `unsafe.Slice(...)`).
2. **Safety & Robustness**: Callers can accidentally pass invalid frame counts or miscalculated slice lengths, causing out-of-bounds memory accesses in native code.
3. **Idiomatic Go**: Standard Go audio and stream libraries accept slices (`[]float32`, `[]int16`, `[]byte`), deriving pointers and validating bounds internally.

This specification outlines the complete elimination of `unsafe.Pointer` from `mago`'s public API.

---

## 2. Goals & Non-Goals

### Goals
- Completely remove `unsafe.Pointer` from public types, structs, constructors, methods, and callbacks.
- Accept and return standard Go slices (`[]float32`, `[]int16`, `[]byte`) for all audio buffer operations.
- Derive native pointers and calculate/validate frame counts internally.
- Support zero-allocation in-place processing (`filter.Process(buf, buf)`).
- Replace raw `unsafe.Pointer` device IDs with a type-safe opaque `DeviceID` type.
- Provide a zero-allocation `DeviceIO` struct for device data callbacks with format-aware slice accessors.
- Update subpackages (`audio`, `speaker`) and all 11 runnable examples (`examples/*`) to demonstrate clean, idiomatic Go with zero `unsafe` imports.

### Non-Goals
- Changing the underlying C bridge (`native/miniaudio_bridge.c`) or `purego` bindings: the C ABI remains fast and unchanged; all transformations happen in the Go package boundary.
- Introducing heavy memory allocations or buffering: slicing into memory passed from native callbacks will use zero-allocation `unsafe.Slice` calls inside `mago`.

---

## 3. Detailed API Specifications

### 3.1 Device Callbacks & `DeviceIO`

#### Current API
```go
type DataCallback func(device *Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32)
```

#### New API
```go
// DataCallback is invoked by miniaudio when the audio hardware requests or provides audio frames.
type DataCallback func(device *Device, io DeviceIO)

// DeviceIO provides access to the playback and capture frame buffers during a callback.
type DeviceIO struct {
	output           unsafe.Pointer
	input            unsafe.Pointer
	frameCount       uint32
	playbackChannels uint32
	captureChannels  uint32
}

// FrameCount returns the number of frames requested/provided in this callback period.
func (io DeviceIO) FrameCount() uint32 { return io.frameCount }

// OutputF32 returns the output buffer as a float32 slice (sized to FrameCount * PlaybackChannels).
// Returns nil if the device has no playback stream.
func (io DeviceIO) OutputF32() []float32 {
	if io.output == nil || io.playbackChannels == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(io.output), int(io.frameCount*io.playbackChannels))
}

// OutputS16 returns the output buffer as an int16 slice (sized to FrameCount * PlaybackChannels).
// Returns nil if the device has no playback stream.
func (io DeviceIO) OutputS16() []int16 {
	if io.output == nil || io.playbackChannels == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(io.output), int(io.frameCount*io.playbackChannels))
}

// OutputBytes returns the output buffer as a raw byte slice.
// Returns nil if the device has no playback stream.
func (io DeviceIO) OutputBytes() []byte {
	if io.output == nil || io.playbackChannels == 0 {
		return nil
	}
	// bytesPerFrame derived from playback format
	...
}

// InputF32 returns the captured input buffer as a float32 slice (sized to FrameCount * CaptureChannels).
// Returns nil if the device has no capture stream.
func (io DeviceIO) InputF32() []float32 {
	if io.input == nil || io.captureChannels == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(io.input), int(io.frameCount*io.captureChannels))
}

// InputS16 returns the captured input buffer as an int16 slice.
func (io DeviceIO) InputS16() []int16 {
	if io.input == nil || io.captureChannels == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(io.input), int(io.frameCount*io.captureChannels))
}

// InputBytes returns the captured input buffer as a raw byte slice.
func (io DeviceIO) InputBytes() []byte { ... }
```

---

### 3.2 Opaque `DeviceID`

#### Current API
```go
type PlaybackDeviceConfig struct {
	DeviceID unsafe.Pointer
	...
}

func (ctx *Context) DeviceInfo(t DeviceType, deviceID unsafe.Pointer) (DeviceInfo, error)
```

#### New API
```go
// DeviceID is a type-safe opaque identifier for a hardware audio endpoint.
type DeviceID [256]byte

// IsZero reports whether the device ID is uninitialized / default.
func (id DeviceID) IsZero() bool {
	return id == DeviceID{}
}

// String returns a readable representation of the device identifier.
func (id DeviceID) String() string

type DeviceInfo struct {
	ID        DeviceID
	Name      string
	IsDefault bool
}

type PlaybackDeviceConfig struct {
	DeviceID *DeviceID // nil means system default playback device
	...
}

type CaptureDeviceConfig struct {
	DeviceID *DeviceID // nil means system default capture device
	...
}

// DeviceInfo reports basic information for a device, or for the default device when id is nil.
func (ctx *Context) DeviceInfo(t DeviceType, id *DeviceID) (DeviceInfo, error)
```

---

### 3.3 DSP Filters & Delay Line

All filter families (`Biquad`, `LowPassFilter1`, `LowPassFilter2`, `LowPassFilter`, `HighPassFilter1`, `HighPassFilter2`, `HighPassFilter`, `BandPassFilter2`, `BandPassFilter`, `NotchFilter`, `PeakFilter`, `LowShelfFilter`, `HighShelfFilter`, `Delay`) replace `ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64)` with format-typed slice methods:

```go
// Process runs audio frames through the filter for 32-bit float samples.
// out and in can point to the same slice for in-place filtering.
func (f *Filter) Process(out, in []float32) error

// ProcessS16 runs audio frames through the filter for 16-bit signed integer samples.
// Supported on all filters except Delay (which is f32 only in miniaudio).
func (f *Filter) ProcessS16(out, in []int16) error
```

#### Internal Behavior & Validation:
- If `len(in) == 0`: immediately returns `nil` (noop).
- Validates `len(in) % int(f.channels) == 0`. If not, returns `ErrInvalidSliceLength`.
- Validates `len(out) >= len(in)`. If not, returns `ErrOutputTooSmall`.
- Frame count is calculated internally: `frameCount = uint64(len(in) / int(f.channels))`.
- Obtains pointers safely: `unsafe.Pointer(&out[0])`, `unsafe.Pointer(&in[0])`.

---

### 3.4 Audio Generators & Buffers

#### `Waveform` & `Noise`
Replace `ReadPCMFrames(unsafe.Pointer, uint64) (uint64, error)` with:
```go
// Read fills out with generated 32-bit float audio frames.
// Returns the number of frames written.
func (w *Waveform) Read(out []float32) (uint64, error)

// Read fills out with generated 32-bit float audio frames.
func (n *Noise) Read(out []float32) (uint64, error)

// ReadS16 fills out with generated 16-bit integer audio frames.
func (n *Noise) ReadS16(out []int16) (uint64, error)
```
- Automatically derives requested frame count as `uint64(len(out) / int(channels))`.

#### `AudioBuffer` & `AudioBufferRef`
```go
type AudioBufferConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Data       []byte // Or DataF32 []float32
}

func (lib *Library) NewAudioBufferRef(format Format, channels uint32, data []byte) (*AudioBufferRef, error)
func (lib *Library) NewAudioBufferRefF32(channels uint32, data []float32) (*AudioBufferRef, error)

func (r *AudioBufferRef) SetData(data []byte) error
func (r *AudioBufferRef) SetDataF32(data []float32) error

func (b *AudioBuffer) Read(out []float32, loop bool) (uint64, error)
func (b *AudioBuffer) ReadS16(out []int16, loop bool) (uint64, error)
func (b *AudioBuffer) ReadBytes(out []byte, loop bool) (uint64, error)

func (b *AudioBuffer) MapF32() ([]float32, error)
func (b *AudioBuffer) MapBytes() ([]byte, error)
```

#### Ring Buffers
```go
// RingBuffer (byte-oriented)
func (r *RingBuffer) AcquireRead(sizeInBytes uint) ([]byte, error)
func (r *RingBuffer) AcquireWrite(sizeInBytes uint) ([]byte, error)

// PCMRingBuffer (frame-oriented)
func (r *PCMRingBuffer) AcquireRead(frames uint32) ([]float32, error)
func (r *PCMRingBuffer) AcquireWrite(frames uint32) ([]float32, error)
```

---

### 3.5 Resamplers & Converters

#### `Resampler` & `LinearResampler`
```go
// Process resamples 32-bit float input frames into output frames.
// Returns the number of input frames read and output frames written.
func (r *Resampler) Process(in, out []float32) (framesInRead, framesOutWritten uint64, err error)
func (r *LinearResampler) Process(in, out []float32) (framesInRead, framesOutWritten uint64, err error)
```

#### `ChannelConverter`
```go
// Process converts channel layout for 32-bit float samples.
func (c *ChannelConverter) Process(out, in []float32) error

// ProcessS16 converts channel layout for 16-bit signed integer samples.
func (c *ChannelConverter) ProcessS16(out, in []int16) error
```

#### `DataConverter` & PCM Format Conversion
```go
// Process converts audio across sample rates, channel counts, and sample formats.
func (c *DataConverter) Process(in, out []byte) (framesInRead, framesOutWritten uint64, err error)
func (c *DataConverter) ProcessF32(in, out []float32) (framesInRead, framesOutWritten uint64, err error)

// PCM Format Conversions
func (lib *Library) ConvertPCMFrames(out, in []byte, formatOut, formatIn Format, channels uint32, dither DitherMode) error
func (lib *Library) ConvertF32ToS16(out []int16, in []float32, dither DitherMode) error
func (lib *Library) ConvertS16ToF32(out []float32, in []int16) error
```

---

## 4. Error Handling & Validation Rules

New exported error sentinels:
```go
var (
	ErrInvalidSliceLength = errors.New("mago: slice length does not align with channel count")
	ErrOutputTooSmall     = errors.New("mago: output slice too small for input frames")
)
```

- **Nil & Closed Safety**: Methods on nil receivers or closed objects return clean `OpError` wrapped errors without panicking.
- **Empty Slices**: `len == 0` is treated as a safe no-op.
- **Zero Allocations**: All slice slicing from pointers (`DeviceIO.OutputF32()`, `RingBuffer.AcquireRead()`) uses `unsafe.Slice` internally without allocating heap memory.

---

## 5. Migration & Subpackage Impact

1. **`audio` package**:
   - `Engine.onDeviceData` updates to `func(device *Device, io DeviceIO)` and writes directly into `io.OutputF32()`.
2. **`speaker` package**:
   - `speakerState.onDeviceData` updates to `func(device *Device, io DeviceIO)` and writes directly into `io.OutputF32()`.
3. **Examples (`examples/*`)**:
   - `tones`, `capture`, `duplex-echo`, `buffers`, `resample`, `channel-map`, `convert-formats`, `logging`, `null-playback`, `device-report`, `audio-wav` will all be updated to use the safe slice API.
   - Zero examples will require `import "unsafe"`.

---

## 6. Testing Strategy

1. **Unit Tests**:
   - Update all existing test suites to use the slice methods (`Process`, `Read`, `DeviceIO`).
   - Add specific tests for:
     - `ErrInvalidSliceLength` when `len(slice) % channels != 0`.
     - `ErrOutputTooSmall` when `len(out) < len(in)`.
     - In-place slice aliasing (`filter.Process(s, s)`).
     - Empty slice no-ops (`len == 0`).
2. **DeviceIO Tests**:
   - Test `OutputF32`, `OutputS16`, `OutputBytes`, `InputF32`, `InputS16`, `InputBytes` with simulated buffers.
3. **Quality Gates**:
   - `mise run lint` (0 issues, 0 vulnerabilities).
   - `mise run test` (all tests pass).
   - `mise run build` (all targets build clean).
