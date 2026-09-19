# Safe Slice API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eliminate all occurrences of `unsafe.Pointer` from public types, structs, constructors, methods, and callbacks in `mago`, replacing them with standard Go slices and opaque types with automatic length and alignment validation.

**Architecture:** Core Go wrappers derive pointers safely via `unsafe.Pointer(&slice[0])` only at the purego boundary while exposing clean typed slice APIs (`Process`, `Read`, `DeviceIO`) to consumers. Zero-allocation `unsafe.Slice` calls are used when transforming pointers delivered by native audio callbacks into Go slices.

**Tech Stack:** Go 1.24+, `purego`, `zig cc` (host library compiler).

---

## File Structure

- Create / Modify:
  - `types.go`: Error sentinels (`ErrInvalidSliceLength`, `ErrOutputTooSmall`), opaque `DeviceID` (`[256]byte`), `DeviceInfo.ID DeviceID`, `PlaybackDeviceConfig.DeviceID *DeviceID`, `CaptureDeviceConfig.DeviceID *DeviceID`.
  - `device_supported.go`: `DeviceIO` struct with `OutputF32`, `OutputS16`, `OutputBytes`, `InputF32`, `InputS16`, `InputBytes`, `FrameCount`; update `DataCallback` signature to `func(device *Device, io DeviceIO)`.
  - `context_supported.go`: Update `DeviceInfo(t DeviceType, id *DeviceID) (DeviceInfo, error)`.
  - `device_test.go` & `context_test.go`: Tests for `DeviceIO` buffer slicing and `DeviceID` matching.
  - `waveform.go`, `waveform_test.go`: `Read(out []float32) (uint64, error)`.
  - `noise.go`, `noise_test.go`: `Read(out []float32) (uint64, error)`, `ReadS16(out []int16) (uint64, error)`.
  - `buffer.go`, `buffer_test.go`: `AudioBuffer.Read`, `ReadS16`, `MapF32`, `MapBytes`; `AudioBufferRef` slice constructors; `RingBuffer.AcquireRead/Write` returning `[]byte`; `PCMRingBuffer.AcquireRead/Write` returning `[]float32`.
  - `biquad.go`, `lpf.go`, `hpf.go`, `bpf.go`, `shelf.go`, `delay.go`: `Process(out, in []float32) error`, `ProcessS16(out, in []int16) error`.
  - `channel.go`, `channel_test.go`: `Process(out, in []float32) error`, `ProcessS16(out, in []int16) error`.
  - `resample.go`, `resample_test.go`: `Resampler.Process(in, out []float32)`, `LinearResampler.Process(in, out []float32)`, `DataConverter.Process(in, out []byte)`, `DataConverter.ProcessF32(in, out []float32)`.
  - `pcm.go`, `pcm_test.go`: `ConvertPCMFrames`, `ConvertF32ToS16`, `ConvertS16ToF32`.
  - `audio/engine.go`, `speaker/speaker.go`: Update `onDeviceData` callback to use `DeviceIO`.
  - `examples/*`: Update all 11 examples to remove `import "unsafe"` and use the safe slice API.
  - `unsupported.go`: Update all platform stubs to match the new signatures.

---

## Task 1: Core Sentinels, Opaque `DeviceID`, `DeviceIO`, and `DataCallback`

**Files:**
- Modify: `types.go`
- Modify: `device_supported.go`
- Modify: `context_supported.go`
- Modify: `audio/engine.go`
- Modify: `speaker/speaker.go`
- Modify: `unsupported.go`
- Create / Modify: `device_test.go`

- [ ] **Step 1: Write failing tests for `DeviceIO` and `DeviceID`**

Create/update `device_test.go`:
```go
package mago

import (
	"testing"
	"unsafe"
)

func TestDeviceIOSlicing(t *testing.T) {
	const frames = 128
	const channels = 2
	totalSamples := frames * channels

	outBuf := make([]float32, totalSamples)
	inBuf := make([]float32, totalSamples)

	io := DeviceIO{
		output:           unsafe.Pointer(&outBuf[0]),
		input:            unsafe.Pointer(&inBuf[0]),
		frameCount:       frames,
		playbackChannels: channels,
		captureChannels:  channels,
	}

	if got := io.FrameCount(); got != frames {
		t.Fatalf("FrameCount: got %d, want %d", got, frames)
	}

	outF32 := io.OutputF32()
	if len(outF32) != totalSamples {
		t.Fatalf("OutputF32 length: got %d, want %d", len(outF32), totalSamples)
	}

	inF32 := io.InputF32()
	if len(inF32) != totalSamples {
		t.Fatalf("InputF32 length: got %d, want %d", len(inF32), totalSamples)
	}

	// Capture-only device: output is nil
	ioCap := DeviceIO{
		input:           unsafe.Pointer(&inBuf[0]),
		frameCount:      frames,
		captureChannels: channels,
	}
	if ioCap.OutputF32() != nil {
		t.Errorf("expected nil OutputF32 for capture-only IO")
	}
}

func TestDeviceIDIsZero(t *testing.T) {
	var id DeviceID
	if !id.IsZero() {
		t.Errorf("expected empty DeviceID to be zero")
	}
	id[0] = 1
	if id.IsZero() {
		t.Errorf("expected non-empty DeviceID to not be zero")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestDeviceIO|TestDeviceID" ./...`
Expected: FAIL due to undefined types and methods.

- [ ] **Step 3: Implement `DeviceID`, `DeviceIO`, and updated `DataCallback`**

In `types.go`:
```go
var (
	ErrInvalidSliceLength = errors.New("mago: slice length does not align with channel count")
	ErrOutputTooSmall     = errors.New("mago: output slice too small for input frames")
)

type DeviceID [256]byte

func (id DeviceID) IsZero() bool {
	return id == DeviceID{}
}

func (id DeviceID) String() string {
	for i, b := range id {
		if b == 0 {
			return string(id[:i])
		}
	}
	return string(id[:])
}

type DeviceInfo struct {
	ID        DeviceID
	Name      string
	IsDefault bool
}

type PlaybackDeviceConfig struct {
	DeviceID                  *DeviceID
	...
}

type CaptureDeviceConfig struct {
	DeviceID                  *DeviceID
	...
}
```

In `device_supported.go`:
```go
type DataCallback func(device *Device, io DeviceIO)

type DeviceIO struct {
	output           unsafe.Pointer
	input            unsafe.Pointer
	frameCount       uint32
	playbackChannels uint32
	captureChannels  uint32
}

func (io DeviceIO) FrameCount() uint32 { return io.frameCount }

func (io DeviceIO) OutputF32() []float32 {
	if io.output == nil || io.playbackChannels == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(io.output), int(io.frameCount*io.playbackChannels))
}

func (io DeviceIO) OutputS16() []int16 {
	if io.output == nil || io.playbackChannels == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(io.output), int(io.frameCount*io.playbackChannels))
}

func (io DeviceIO) OutputBytes() []byte {
	if io.output == nil || io.playbackChannels == 0 {
		return nil
	}
	// For raw bytes, we determine bytes per sample from the device or default to 4 bytes per sample (f32)
	return unsafe.Slice((*byte)(io.output), int(io.frameCount*io.playbackChannels*4))
}

func (io DeviceIO) InputF32() []float32 {
	if io.input == nil || io.captureChannels == 0 {
		return nil
	}
	return unsafe.Slice((*float32)(io.input), int(io.frameCount*io.captureChannels))
}

func (io DeviceIO) InputS16() []int16 {
	if io.input == nil || io.captureChannels == 0 {
		return nil
	}
	return unsafe.Slice((*int16)(io.input), int(io.frameCount*io.captureChannels))
}

func (io DeviceIO) InputBytes() []byte {
	if io.input == nil || io.captureChannels == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(io.input), int(io.frameCount*io.captureChannels*4))
}
```
Update `onDataCallback` dispatcher in `device_supported.go` to construct `DeviceIO` and pass it to `state.onData(state.device, io)`.
Update `Context.DeviceInfo(t DeviceType, id *DeviceID) (DeviceInfo, error)` in `context_supported.go`.
Update `copyDeviceInfo` to copy `native.ID` directly to `info.ID`.
Update `audio/engine.go` and `speaker/speaker.go` callback handlers to accept `io DeviceIO` and write directly to `io.OutputF32()`.
Update `unsupported.go` stubs.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestDeviceIO|TestDeviceID" ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add types.go device_supported.go context_supported.go audio/engine.go speaker/speaker.go unsupported.go device_test.go
git commit -m "feat: introduce DeviceIO, opaque DeviceID, and safe DataCallback"
```

---

## Task 2: Audio Generators and Buffers Refactor

**Files:**
- Modify: `waveform.go`, `waveform_test.go`
- Modify: `noise.go`, `noise_test.go`
- Modify: `buffer.go`, `buffer_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing tests for Generator & Buffer slice methods**

In `waveform_test.go`:
```go
func TestWaveformReadSlice(t *testing.T) {
	lib := newNullLibrary(t)
	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   2,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = wf.Close() }()

	buf := make([]float32, 256) // 128 frames for stereo
	read, err := wf.Read(buf)
	if err != nil {
		t.Fatalf("wf.Read: %v", err)
	}
	if read != 128 {
		t.Errorf("read: got %d frames, want 128", read)
	}

	// Misaligned buffer should fail
	oddBuf := make([]float32, 255)
	if _, err := wf.Read(oddBuf); err != ErrInvalidSliceLength {
		t.Errorf("expected ErrInvalidSliceLength, got %v", err)
	}
}
```

Add similar tests in `noise_test.go` (`n.Read(buf)`, `n.ReadS16(bufS16)`) and `buffer_test.go` (`buf.Read`, `buf.MapF32`, `ring.AcquireRead([]byte)`).

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -run "TestWaveformReadSlice|TestNoiseReadSlice" ./...`
Expected: FAIL due to missing `Read` methods on `Waveform` and `Noise`.

- [ ] **Step 3: Implement safe slice methods on Generators & Buffers**

In `waveform.go`:
```go
func (w *Waveform) Read(out []float32) (uint64, error) {
	if w == nil || w.handle == nil {
		return 0, fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if len(out)%int(w.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	frameCount := uint64(len(out) / int(w.channels))
	return w.lib.bindings.maWaveformReadPCMFrames(w.handle, unsafe.Pointer(&out[0]), frameCount)
}
```

In `noise.go`:
Implement `Read(out []float32) (uint64, error)` and `ReadS16(out []int16) (uint64, error)`.

In `buffer.go`:
- `AudioBuffer.Read(out []float32, loop bool) (uint64, error)`
- `AudioBuffer.ReadS16(out []int16, loop bool) (uint64, error)`
- `AudioBuffer.MapF32() ([]float32, error)`
- `AudioBuffer.MapBytes() ([]byte, error)`
- `NewAudioBufferRefF32(channels uint32, data []float32) (*AudioBufferRef, error)`
- `NewAudioBufferRef(format Format, channels uint32, data []byte) (*AudioBufferRef, error)`
- `RingBuffer.AcquireRead(sizeInBytes uint) ([]byte, error)`
- `RingBuffer.AcquireWrite(sizeInBytes uint) ([]byte, error)`
- `PCMRingBuffer.AcquireRead(frames uint32) ([]float32, error)`
- `PCMRingBuffer.AcquireWrite(frames uint32) ([]float32, error)`
Update `unsupported.go` stubs.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestWaveform|TestNoise|TestBuffer|TestRingBuffer" ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add waveform.go waveform_test.go noise.go noise_test.go buffer.go buffer_test.go unsupported.go
git commit -m "feat: implement safe slice APIs for waveforms, noise, and buffers"
```

---

## Task 3: DSP Filter Family Refactor

**Files:**
- Modify: `biquad.go`, `biquad_test.go`
- Modify: `lpf.go`, `lpf_test.go`
- Modify: `hpf.go`, `hpf_test.go`
- Modify: `bpf.go`, `bpf_test.go`
- Modify: `shelf.go`, `shelf_test.go`
- Modify: `delay.go`, `delay_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing tests for Filter slice `Process` and `ProcessS16`**

Update `biquad_test.go`:
```go
func TestBiquadProcessSlice(t *testing.T) {
	lib := newNullLibrary(t)
	bq, err := lib.NewBiquad(BiquadConfig{
		Format:   FormatF32,
		Channels: 1,
		B0: 1, B1: 0, B2: 0,
		A0: 1, A1: 0, A2: 0,
	})
	if err != nil {
		t.Fatalf("NewBiquad: %v", err)
	}
	defer func() { _ = bq.Close() }()

	in := []float32{0.1, 0.2, 0.3, 0.4}
	out := make([]float32, len(in))

	if err := bq.Process(out, in); err != nil {
		t.Fatalf("Process: %v", err)
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("sample %d: got %f, want %f", i, out[i], in[i])
		}
	}

	// In-place processing
	if err := bq.Process(in, in); err != nil {
		t.Fatalf("in-place Process: %v", err)
	}

	// Validation
	shortOut := make([]float32, 2)
	if err := bq.Process(shortOut, in); err != ErrOutputTooSmall {
		t.Errorf("expected ErrOutputTooSmall, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestBiquadProcessSlice" ./...`
Expected: FAIL due to missing `Process` method.

- [ ] **Step 3: Implement `Process` and `ProcessS16` across all 14 filter types**

In `types.go` or a private helper in `biquad.go`:
```go
func validateFilterSlices[T float32 | int16](channels uint32, out, in []T) (uint64, error) {
	if len(in) == 0 {
		return 0, nil
	}
	if channels == 0 || len(in)%int(channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	if len(out) < len(in) {
		return 0, ErrOutputTooSmall
	}
	return uint64(len(in) / int(channels)), nil
}
```

Implement `Process(out, in []float32) error` and `ProcessS16(out, in []int16) error` on:
- `Biquad`
- `LowPassFilter1`, `LowPassFilter2`, `LowPassFilter`
- `HighPassFilter1`, `HighPassFilter2`, `HighPassFilter`
- `BandPassFilter2`, `BandPassFilter`
- `NotchFilter`, `PeakFilter`, `LowShelfFilter`, `HighShelfFilter`
- `Delay` (implements `Process(out, in []float32) error`; miniaudio delay does not support `s16`)

Remove the old `ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64)` methods.
Update all existing filter tests (`biquad_test.go`, `lpf_test.go`, `hpf_test.go`, `bpf_test.go`, `shelf_test.go`, `delay_test.go`) to use `.Process(out, in)`.
Update `unsupported.go` stubs.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestBiquad|TestLPF|TestHPF|TestBPF|TestShelf|TestDelay" ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add biquad.go biquad_test.go lpf.go lpf_test.go hpf.go hpf_test.go bpf.go bpf_test.go shelf.go shelf_test.go delay.go delay_test.go unsupported.go
git commit -m "feat: implement safe Process slice APIs for all filter families and delay"
```

---

## Task 4: Converters and Resamplers Refactor

**Files:**
- Modify: `channel.go`, `channel_test.go`
- Modify: `resample.go`, `resample_test.go`
- Modify: `pcm.go`, `pcm_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing tests for converter & resampler slice methods**

In `resample_test.go`:
```go
func TestResamplerProcessSlice(t *testing.T) {
	lib := newNullLibrary(t)
	r, err := lib.NewResampler(ResamplerConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRateIn: 44100,
		SampleRateOut: 48000,
		Algorithm:  ResampleAlgorithmLinear,
	})
	if err != nil {
		t.Fatalf("NewResampler: %v", err)
	}
	defer func() { _ = r.Close() }()

	in := make([]float32, 441)
	out := make([]float32, 480)
	inRead, outWritten, err := r.Process(in, out)
	if err != nil {
		t.Fatalf("r.Process: %v", err)
	}
	if inRead == 0 || outWritten == 0 {
		t.Errorf("expected frames processed, got in=%d, out=%d", inRead, outWritten)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestResamplerProcessSlice" ./...`
Expected: FAIL due to missing `Process` method.

- [ ] **Step 3: Implement safe slice methods on Converters and Resamplers**

In `channel.go`:
- `ChannelConverter.Process(out, in []float32) error`
- `ChannelConverter.ProcessS16(out, in []int16) error`
- Remove `ProcessPCMFrames`.

In `resample.go`:
- `Resampler.Process(in, out []float32) (framesInRead, framesOutWritten uint64, err error)`
- `LinearResampler.Process(in, out []float32) (framesInRead, framesOutWritten uint64, err error)`
- `DataConverter.Process(in, out []byte) (framesInRead, framesOutWritten uint64, err error)`
- `DataConverter.ProcessF32(in, out []float32) (framesInRead, framesOutWritten uint64, err error)`
- Remove `ProcessPCMFrames`.

In `pcm.go`:
- `ConvertPCMFrames(out, in []byte, formatOut, formatIn Format, channels uint32, dither DitherMode) error`
- `ConvertF32ToS16(out []int16, in []float32, dither DitherMode) error`
- `ConvertS16ToF32(out []float32, in []int16) error`
- Remove `ConvertPCMSamples` and `ConvertPCMFramesFormat` with `unsafe.Pointer`.

Update tests in `channel_test.go`, `resample_test.go`, `pcm_test.go`.
Update `unsupported.go` stubs.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestChannel|TestResampler|TestPCM" ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add channel.go channel_test.go resample.go resample_test.go pcm.go pcm_test.go unsupported.go
git commit -m "feat: implement safe slice APIs for resamplers and channel/data converters"
```

---

## Task 5: Examples Modernization, Full Audit, Linting, and CI Verification

**Files:**
- Modify: `examples/tones/main.go`
- Modify: `examples/null-playback/main.go`
- Modify: `examples/capture/main.go`
- Modify: `examples/duplex-echo/main.go`
- Modify: `examples/buffers/main.go`
- Modify: `examples/channel-map/main.go`
- Modify: `examples/convert-formats/main.go`
- Modify: `examples/device-report/main.go`
- Modify: `examples/logging/main.go`
- Modify: `examples/resample/main.go`
- Modify: `examples/audio-wav/main.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Audit all public files for remaining `unsafe.Pointer`**

Run: `git grep -n "unsafe.Pointer" -- '*.go' ':!internal/**' ':!*_test.go' ':!zz_generated*' ':!embed_*' ':!unsupported.go'`
Verify: The only remaining `unsafe.Pointer` instances are internal private fields/calls (e.g. `magoFree(unsafe.Pointer(handle))`). Zero public functions, methods, callbacks, or struct fields require or accept `unsafe.Pointer`.

- [ ] **Step 2: Update all 11 examples**

Update all examples to use `io DeviceIO`, `filter.Process`, `resampler.Process`, etc.
Verify that none of the examples import `"unsafe"`.

- [ ] **Step 3: Run linter and formatting**

```bash
mise run fmt
mise run lint
```
Expected: 0 lint errors, no vulnerabilities.

- [ ] **Step 4: Run full test suite and build**

```bash
mise run test
mise run build
```
Expected: All tests pass across all packages, clean build.

- [ ] **Step 5: Commit and close Beads issue**

```bash
git add examples/ unsupported.go
git commit -m "refactor(examples): modernize all examples to safe slice APIs without unsafe imports"
bd close mago-8a3.14
```
