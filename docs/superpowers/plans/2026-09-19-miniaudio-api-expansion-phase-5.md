# Phase 5: Waveform and Noise Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose signal generation data sources: `Waveform` (`ma_waveform` - sine, square, triangle, sawtooth) and `Noise` (`ma_noise` - white, pink, brownian), and migrate `examples/tones` to use `ma_waveform`.

**Architecture:** Two opaque objects allocated through `mago_alloc`. Config structs (`ma_waveform_config`, `ma_noise_config`, both 32 bytes) are mirrored in Go and validated by the `internal/abi` layout probe. All waveform and noise functions are bound directly against exported `ma_*` symbols with purego. Single native library rebuild for the phase.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, `dockercross/osxcross`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 5)
**Predecessors:** `.../phase-0.md` through `.../phase-4.md`

---

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass at all times.
- **Minimal C.** Two `mago_object_type` values (`MAGO_OBJECT_WAVEFORM = 14`, `MAGO_OBJECT_NOISE = 15`) plus their `mago_alloc` cases. No wrapper functions.
- **One native rebuild for the phase** in Task 1 (`mise run build-lib-all && mise run generate`).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Mirrors are validated** by `internal/abi` + `layout_test.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

| Type | Size | Notes |
| --- | ---: | --- |
| `ma_waveform_config` | 32 | format (0), channels (4), sampleRate (8), type (12), amplitude (16, f64), frequency (24, f64) |
| `ma_noise_config` | 32 | format (0), channels (4), type (8), seed (12), amplitude (16, f64), duplicateChannels (24, u32) |
| `ma_waveform` | 120 | Allocated via `mago_alloc(magoObjectWaveform)` |
| `ma_noise` | 152 | Allocated via `mago_alloc(magoObjectNoise)` |

- `ma_waveform_init(pConfig, pWaveform)` returns `ma_result`. `ma_waveform_uninit(pWaveform)` returns `void`.
- `ma_waveform_read_pcm_frames(pWaveform, pFramesOut, frameCount, pFramesRead)` returns `ma_result` and writes frames actually read to `*pFramesRead`.
- `ma_noise_init(pConfig, pAllocationCallbacks, pNoise)` returns `ma_result`. Passing `nil` uses default allocator.
- `ma_noise_uninit(pNoise, pAllocationCallbacks)` returns `void`.
- `ma_noise_read_pcm_frames(pNoise, pFramesOut, frameCount, pFramesRead)` returns `ma_result` and writes frames read to `*pFramesRead`.
- `ma_noise_set_type` always returns `MA_INVALID_OPERATION` in miniaudio 0.11.25 (changing noise type dynamically is explicitly unsupported by miniaudio because heap allocations differ).
- Default seed when `config.seed == 0` is `MA_DEFAULT_LCG_SEED = 4321`.

---

## Task 1: Bindings, mirrors, probe, and single native rebuild

**Files:**
- Modify: `native/miniaudio_bridge.c`
- Modify: `internal/abi/layout_probe.c`
- Modify: `types.go`
- Modify: `internal/gen/bindings/main.go`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `MAGO_OBJECT_WAVEFORM = 14`, `MAGO_OBJECT_NOISE = 15`.
- Produces Go: `WaveformType`, `NoiseType`, `waveformConfigNative`, `noiseConfigNative`, `waveformHandle`, `noiseHandle`.
- Produces bindings for all `ma_waveform_*` and `ma_noise_*` symbols.

- [ ] **Step 1: Add the allocation types in `native/miniaudio_bridge.c`**

In `enum mago_object_type`:
```c
    MAGO_OBJECT_PCM_RB            = 13,
    MAGO_OBJECT_WAVEFORM          = 14,
    MAGO_OBJECT_NOISE             = 15
```

In `mago_alloc`:
```c
        case MAGO_OBJECT_PCM_RB:           return calloc(1, sizeof(ma_pcm_rb));
        case MAGO_OBJECT_WAVEFORM:         return calloc(1, sizeof(ma_waveform));
        case MAGO_OBJECT_NOISE:            return calloc(1, sizeof(ma_noise));
```

- [ ] **Step 2: Extend `internal/abi/layout_probe.c`**

Add probes for waveform and noise configs:
```c
    printf("sizeof:ma_waveform_config %zu\n", sizeof(ma_waveform_config));
    printf("offsetof:ma_waveform_config.format %zu\n", offsetof(ma_waveform_config, format));
    printf("offsetof:ma_waveform_config.channels %zu\n", offsetof(ma_waveform_config, channels));
    printf("offsetof:ma_waveform_config.sampleRate %zu\n", offsetof(ma_waveform_config, sampleRate));
    printf("offsetof:ma_waveform_config.type %zu\n", offsetof(ma_waveform_config, type));
    printf("offsetof:ma_waveform_config.amplitude %zu\n", offsetof(ma_waveform_config, amplitude));
    printf("offsetof:ma_waveform_config.frequency %zu\n", offsetof(ma_waveform_config, frequency));
    printf("sizeof:ma_noise_config %zu\n", sizeof(ma_noise_config));
    printf("offsetof:ma_noise_config.format %zu\n", offsetof(ma_noise_config, format));
    printf("offsetof:ma_noise_config.channels %zu\n", offsetof(ma_noise_config, channels));
    printf("offsetof:ma_noise_config.type %zu\n", offsetof(ma_noise_config, type));
    printf("offsetof:ma_noise_config.seed %zu\n", offsetof(ma_noise_config, seed));
    printf("offsetof:ma_noise_config.amplitude %zu\n", offsetof(ma_noise_config, amplitude));
    printf("offsetof:ma_noise_config.duplicateChannels %zu\n", offsetof(ma_noise_config, duplicateChannels));
    printf("sizeof:ma_waveform %zu\n", sizeof(ma_waveform));
    printf("sizeof:ma_noise %zu\n", sizeof(ma_noise));
```

- [ ] **Step 3: Add the Go types and mirrors to `types.go`**

```go
// WaveformType identifies the periodic shape of a waveform.
type WaveformType int32

// NoiseType identifies the spectral distribution of generated noise.
type NoiseType int32

// waveformConfigNative mirrors ma_waveform_config. Validated by layout_test.go.
type waveformConfigNative struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Type       WaveformType
	Amplitude  float64
	Frequency  float64
}

// noiseConfigNative mirrors ma_noise_config. Validated by layout_test.go.
type noiseConfigNative struct {
	Format            Format
	Channels          uint32
	Type              NoiseType
	Seed              int32
	Amplitude         float64
	DuplicateChannels uint32
}

type waveformHandle struct{}
type noiseHandle struct{}
```

In the `mago_object_type` constant block in `types.go`:
```go
	magoObjectPCMRingBuffer    int32 = 13
	magoObjectWaveform         int32 = 14
	magoObjectNoise            int32 = 15
```

- [ ] **Step 4: Add constants and bindings to `internal/gen/bindings/main.go`**

In `baseConstants`:
```go
	{Name: "WaveformTypeSine", Type: "WaveformType", Value: "0"},
	{Name: "WaveformTypeSquare", Type: "WaveformType", Value: "1"},
	{Name: "WaveformTypeTriangle", Type: "WaveformType", Value: "2"},
	{Name: "WaveformTypeSawtooth", Type: "WaveformType", Value: "3"},
	{Name: "NoiseTypeWhite", Type: "NoiseType", Value: "0"},
	{Name: "NoiseTypePink", Type: "NoiseType", Value: "1"},
	{Name: "NoiseTypeBrownian", Type: "NoiseType", Value: "2"},
```

In `functions`:
```go
	{FieldName: "maWaveformInit", Symbol: "ma_waveform_init", Type: "func(*waveformConfigNative, *waveformHandle) Result"},
	{FieldName: "maWaveformUninit", Symbol: "ma_waveform_uninit", Type: "func(*waveformHandle)"},
	{FieldName: "maWaveformReadPCMFrames", Symbol: "ma_waveform_read_pcm_frames", Type: "func(*waveformHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "maWaveformSeekToPCMFrame", Symbol: "ma_waveform_seek_to_pcm_frame", Type: "func(*waveformHandle, uint64) Result"},
	{FieldName: "maWaveformSetAmplitude", Symbol: "ma_waveform_set_amplitude", Type: "func(*waveformHandle, float64) Result"},
	{FieldName: "maWaveformSetFrequency", Symbol: "ma_waveform_set_frequency", Type: "func(*waveformHandle, float64) Result"},
	{FieldName: "maWaveformSetType", Symbol: "ma_waveform_set_type", Type: "func(*waveformHandle, WaveformType) Result"},
	{FieldName: "maWaveformSetSampleRate", Symbol: "ma_waveform_set_sample_rate", Type: "func(*waveformHandle, uint32) Result"},
	{FieldName: "maNoiseInit", Symbol: "ma_noise_init", Type: "func(*noiseConfigNative, unsafe.Pointer, *noiseHandle) Result"},
	{FieldName: "maNoiseUninit", Symbol: "ma_noise_uninit", Type: "func(*noiseHandle, unsafe.Pointer)"},
	{FieldName: "maNoiseReadPCMFrames", Symbol: "ma_noise_read_pcm_frames", Type: "func(*noiseHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "maNoiseSetAmplitude", Symbol: "ma_noise_set_amplitude", Type: "func(*noiseHandle, float64) Result"},
	{FieldName: "maNoiseSetSeed", Symbol: "ma_noise_set_seed", Type: "func(*noiseHandle, int32) Result"},
	{FieldName: "maNoiseSetType", Symbol: "ma_noise_set_type", Type: "func(*noiseHandle, NoiseType) Result"},
```

- [ ] **Step 5: Extend `layout_test.go`**

Add assertions:
```go
	if got, want := uint64(unsafe.Sizeof(waveformConfigNative{})), probe["sizeof:ma_waveform_config"]; got != want {
		t.Errorf("sizeof waveformConfigNative: mirror %d, header %d", got, want)
	}
	waveformCfg := waveformConfigNative{}
	waveformOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":     {unsafe.Offsetof(waveformCfg.Format), "offsetof:ma_waveform_config.format"},
		"channels":   {unsafe.Offsetof(waveformCfg.Channels), "offsetof:ma_waveform_config.channels"},
		"sampleRate": {unsafe.Offsetof(waveformCfg.SampleRate), "offsetof:ma_waveform_config.sampleRate"},
		"type":       {unsafe.Offsetof(waveformCfg.Type), "offsetof:ma_waveform_config.type"},
		"amplitude":  {unsafe.Offsetof(waveformCfg.Amplitude), "offsetof:ma_waveform_config.amplitude"},
		"frequency":  {unsafe.Offsetof(waveformCfg.Frequency), "offsetof:ma_waveform_config.frequency"},
	}
	for name, check := range waveformOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_waveform_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(noiseConfigNative{})), probe["sizeof:ma_noise_config"]; got != want {
		t.Errorf("sizeof noiseConfigNative: mirror %d, header %d", got, want)
	}
	noiseCfg := noiseConfigNative{}
	noiseOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"format":            {unsafe.Offsetof(noiseCfg.Format), "offsetof:ma_noise_config.format"},
		"channels":          {unsafe.Offsetof(noiseCfg.Channels), "offsetof:ma_noise_config.channels"},
		"type":              {unsafe.Offsetof(noiseCfg.Type), "offsetof:ma_noise_config.type"},
		"seed":              {unsafe.Offsetof(noiseCfg.Seed), "offsetof:ma_noise_config.seed"},
		"amplitude":         {unsafe.Offsetof(noiseCfg.Amplitude), "offsetof:ma_noise_config.amplitude"},
		"duplicateChannels": {unsafe.Offsetof(noiseCfg.DuplicateChannels), "offsetof:ma_noise_config.duplicateChannels"},
	}
	for name, check := range noiseOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_noise_config.%s: mirror %d, header %d", name, got, want)
		}
	}
```

- [ ] **Step 6: Rebuild native libraries and regenerate Go bindings**

```bash
mise run build-lib-all
mise run generate
go test -run TestLayoutMatchesHeader ./
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add native/miniaudio_bridge.c internal/abi/layout_probe.c types.go internal/gen/bindings/main.go layout_test.go zz_generated.bindings.go embed_*.go
git commit -m "feat: add waveform and noise allocation, bindings and mirrors"
```

---

## Task 2: Implement Waveform (`waveform.go`, `waveform_test.go`, `unsupported.go`)

**Files:**
- Create: `waveform.go`
- Create: `waveform_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write the failing tests in `waveform_test.go`**

```go
package mago

import (
	"math"
	"testing"
	"unsafe"
)

func TestWaveformSineKnownSamples(t *testing.T) {
	lib := newNullLibrary(t)

	// 48 kHz, 1 channel, 1000 Hz sine, amplitude 1.0.
	// One full cycle is 48 samples.
	waveform, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  1.0,
		Frequency:  1000.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = waveform.Close() }()

	samples := make([]float32, 48)
	read, err := waveform.ReadPCMFrames(unsafe.Pointer(&samples[0]), uint64(len(samples)))
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != uint64(len(samples)) {
		t.Fatalf("read %d frames, want %d", read, len(samples))
	}

	// Sample 0: sin(0) = 0.0
	if math.Abs(float64(samples[0])) > 1e-4 {
		t.Errorf("sample[0] = %f, want ~0.0", samples[0])
	}
	// Sample 12 (quarter cycle): sin(pi/2) = 1.0
	if math.Abs(float64(samples[12])-1.0) > 1e-3 {
		t.Errorf("sample[12] = %f, want ~1.0", samples[12])
	}
	// Sample 24 (half cycle): sin(pi) = 0.0
	if math.Abs(float64(samples[24])) > 1e-3 {
		t.Errorf("sample[24] = %f, want ~0.0", samples[24])
	}
	// Sample 36 (three quarter cycle): sin(3pi/2) = -1.0
	if math.Abs(float64(samples[36])+1.0) > 1e-3 {
		t.Errorf("sample[36] = %f, want ~-1.0", samples[36])
	}
}

func TestWaveformShapes(t *testing.T) {
	lib := newNullLibrary(t)

	shapes := []WaveformType{
		WaveformTypeSine,
		WaveformTypeSquare,
		WaveformTypeTriangle,
		WaveformTypeSawtooth,
	}

	for _, shape := range shapes {
		wf, err := lib.NewWaveform(WaveformConfig{
			Format:     FormatF32,
			Channels:   1,
			SampleRate: 48000,
			Type:       shape,
			Amplitude:  0.8,
			Frequency:  440.0,
		})
		if err != nil {
			t.Fatalf("NewWaveform(%v): %v", shape, err)
		}

		buf := make([]float32, 100)
		read, err := wf.ReadPCMFrames(unsafe.Pointer(&buf[0]), uint64(len(buf)))
		if err != nil {
			_ = wf.Close()
			t.Fatalf("ReadPCMFrames(%v): %v", shape, err)
		}
		if read != uint64(len(buf)) {
			_ = wf.Close()
			t.Fatalf("read %d frames for shape %v, want %d", read, shape, len(buf))
		}

		for i, v := range buf {
			if float64(v) > 0.81 || float64(v) < -0.81 {
				_ = wf.Close()
				t.Fatalf("shape %v sample[%d] = %f out of amplitude bounds [-0.8, 0.8]", shape, i, v)
			}
		}

		if err := wf.Close(); err != nil {
			t.Fatalf("Close(%v): %v", shape, err)
		}
	}
}

func TestWaveformControls(t *testing.T) {
	lib := newNullLibrary(t)

	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = wf.Close() }()

	if err := wf.SetAmplitude(0.25); err != nil {
		t.Errorf("SetAmplitude: %v", err)
	}
	if err := wf.SetFrequency(880.0); err != nil {
		t.Errorf("SetFrequency: %v", err)
	}
	if err := wf.SetType(WaveformTypeSquare); err != nil {
		t.Errorf("SetType: %v", err)
	}
	if err := wf.SetSampleRate(44100); err != nil {
		t.Errorf("SetSampleRate: %v", err)
	}
	if err := wf.SeekToPCMFrame(0); err != nil {
		t.Errorf("SeekToPCMFrame: %v", err)
	}

	buf := make([]float32, 10)
	read, err := wf.ReadPCMFrames(unsafe.Pointer(&buf[0]), 10)
	if err != nil || read != 10 {
		t.Fatalf("read %d frames (%v), want 10", read, err)
	}
}

func TestWaveformLifecycle(t *testing.T) {
	lib := newNullLibrary(t)

	wf, err := lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   1,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  0.5,
		Frequency:  440.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}

	if err := wf.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := wf.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var buf [4]float32
	if _, err := wf.ReadPCMFrames(unsafe.Pointer(&buf[0]), 4); err == nil {
		t.Fatal("read on closed waveform should return error")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test -run TestWaveform ./
```
Expected: FAIL (compile errors: undefined `WaveformConfig`, `NewWaveform`, etc.).

- [ ] **Step 3: Implement `waveform.go` and update `unsupported.go`**

Create `waveform.go`:
```go
//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// WaveformConfig describes how a waveform generator should be initialized.
type WaveformConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Type       WaveformType
	Amplitude  float64
	Frequency  float64
}

// Waveform generates periodic audio waveforms (sine, square, triangle, sawtooth).
type Waveform struct {
	lib    *Library
	handle *waveformHandle
}

// NewWaveform creates and initializes a new Waveform generator.
func (lib *Library) NewWaveform(config WaveformConfig) (*Waveform, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	native := waveformConfigNative{
		Format:     config.Format,
		Channels:   config.Channels,
		SampleRate: config.SampleRate,
		Type:       config.Type,
		Amplitude:  config.Amplitude,
		Frequency:  config.Frequency,
	}

	handle := (*waveformHandle)(lib.bindings.magoAlloc(magoObjectWaveform))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate waveform: out of memory")
	}

	result := lib.bindings.maWaveformInit(&native, handle)
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_waveform_init", result)
	}

	return &Waveform{lib: lib, handle: handle}, nil
}

// ReadPCMFrames writes up to frameCount frames into out, returning how many were written.
func (w *Waveform) ReadPCMFrames(out unsafe.Pointer, frameCount uint64) (uint64, error) {
	if w == nil || w.handle == nil {
		return 0, fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var framesRead uint64
	result := w.lib.bindings.maWaveformReadPCMFrames(w.handle, out, frameCount, &framesRead)
	if result != Success {
		return framesRead, w.lib.resultError("ma_waveform_read_pcm_frames", result)
	}
	return framesRead, nil
}

// SeekToPCMFrame moves the waveform internal cursor to frameIndex.
func (w *Waveform) SeekToPCMFrame(frameIndex uint64) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_seek_to_pcm_frame", w.lib.bindings.maWaveformSeekToPCMFrame(w.handle, frameIndex))
}

// SetAmplitude updates the waveform amplitude.
func (w *Waveform) SetAmplitude(amplitude float64) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_amplitude", w.lib.bindings.maWaveformSetAmplitude(w.handle, amplitude))
}

// SetFrequency updates the waveform frequency in Hz.
func (w *Waveform) SetFrequency(frequency float64) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_frequency", w.lib.bindings.maWaveformSetFrequency(w.handle, frequency))
}

// SetType updates the waveform shape (sine, square, triangle, sawtooth).
func (w *Waveform) SetType(waveformType WaveformType) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_type", w.lib.bindings.maWaveformSetType(w.handle, waveformType))
}

// SetSampleRate updates the waveform sample rate in Hz.
func (w *Waveform) SetSampleRate(sampleRate uint32) error {
	if w == nil || w.handle == nil {
		return fmt.Errorf("mago: nil waveform")
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	return w.lib.resultError("ma_waveform_set_sample_rate", w.lib.bindings.maWaveformSetSampleRate(w.handle, sampleRate))
}

// Close uninitializes the waveform and frees its native memory.
func (w *Waveform) Close() error {
	if w == nil || w.handle == nil {
		return nil
	}
	if err := w.lib.ensureOpen(); err != nil {
		return err
	}
	w.lib.bindings.maWaveformUninit(w.handle)
	w.lib.bindings.magoFree(unsafe.Pointer(w.handle))
	w.handle = nil
	return nil
}
```

In `unsupported.go`:
```go
type WaveformConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Type       WaveformType
	Amplitude  float64
	Frequency  float64
}
type Waveform struct{}

func (*Library) NewWaveform(WaveformConfig) (*Waveform, error) { return nil, errUnsupportedPlatform }
func (*Waveform) ReadPCMFrames(unsafe.Pointer, uint64) (uint64, error) { return 0, errUnsupportedPlatform }
func (*Waveform) SeekToPCMFrame(uint64) error { return errUnsupportedPlatform }
func (*Waveform) SetAmplitude(float64) error { return errUnsupportedPlatform }
func (*Waveform) SetFrequency(float64) error { return errUnsupportedPlatform }
func (*Waveform) SetType(WaveformType) error { return errUnsupportedPlatform }
func (*Waveform) SetSampleRate(uint32) error { return errUnsupportedPlatform }
func (*Waveform) Close() error { return errUnsupportedPlatform }
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run TestWaveform ./
```
Expected: PASS.

- [ ] **Step 5: Commit and close task `mago-8a3.6.1`**

```bash
git add waveform.go waveform_test.go unsupported.go
git commit -m "feat: expose ma_waveform data source"
bd close mago-8a3.6.1 --reason="Implemented Waveform data source wrapper with full controls and tests"
```

---

## Task 3: Implement Noise (`noise.go`, `noise_test.go`, `unsupported.go`)

**Files:**
- Create: `noise.go`
- Create: `noise_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write the failing tests in `noise_test.go`**

```go
package mago

import (
	"bytes"
	"math"
	"testing"
	"unsafe"
)

func TestNoiseSeededReproducibility(t *testing.T) {
	lib := newNullLibrary(t)

	newNoiseWithSeed := func(seed int32) *Noise {
		n, err := lib.NewNoise(NoiseConfig{
			Format:    FormatF32,
			Channels:  1,
			Type:      NoiseTypeWhite,
			Seed:      seed,
			Amplitude: 0.5,
		})
		if err != nil {
			t.Fatalf("NewNoise: %v", err)
		}
		return n
	}

	n1 := newNoiseWithSeed(12345)
	defer func() { _ = n1.Close() }()
	n2 := newNoiseWithSeed(12345)
	defer func() { _ = n2.Close() }()
	n3 := newNoiseWithSeed(54321)
	defer func() { _ = n3.Close() }()

	const frameCount = 128
	buf1 := make([]float32, frameCount)
	buf2 := make([]float32, frameCount)
	buf3 := make([]float32, frameCount)

	read1, err1 := n1.ReadPCMFrames(unsafe.Pointer(&buf1[0]), frameCount)
	read2, err2 := n2.ReadPCMFrames(unsafe.Pointer(&buf2[0]), frameCount)
	read3, err3 := n3.ReadPCMFrames(unsafe.Pointer(&buf3[0]), frameCount)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("ReadPCMFrames failed: %v, %v, %v", err1, err2, err3)
	}
	if read1 != frameCount || read2 != frameCount || read3 != frameCount {
		t.Fatalf("read counts mismatch: %d, %d, %d", read1, read2, read3)
	}

	b1 := unsafe.Slice((*byte)(unsafe.Pointer(&buf1[0])), frameCount*4)
	b2 := unsafe.Slice((*byte)(unsafe.Pointer(&buf2[0])), frameCount*4)
	b3 := unsafe.Slice((*byte)(unsafe.Pointer(&buf3[0])), frameCount*4)

	if !bytes.Equal(b1, b2) {
		t.Fatal("identical seeds produced different noise output")
	}
	if bytes.Equal(b1, b3) {
		t.Fatal("different seeds produced identical noise output")
	}
}

func TestNoiseTypes(t *testing.T) {
	lib := newNullLibrary(t)

	types := []NoiseType{
		NoiseTypeWhite,
		NoiseTypePink,
		NoiseTypeBrownian,
	}

	for _, ntype := range types {
		n, err := lib.NewNoise(NoiseConfig{
			Format:    FormatF32,
			Channels:  2,
			Type:      ntype,
			Seed:      42,
			Amplitude: 0.6,
		})
		if err != nil {
			t.Fatalf("NewNoise(%v): %v", ntype, err)
		}

		buf := make([]float32, 200) // 100 frames * 2 channels
		read, err := n.ReadPCMFrames(unsafe.Pointer(&buf[0]), 100)
		if err != nil {
			_ = n.Close()
			t.Fatalf("ReadPCMFrames(%v): %v", ntype, err)
		}
		if read != 100 {
			_ = n.Close()
			t.Fatalf("read %d frames for %v, want 100", read, ntype)
		}

		hasNonZero := false
		for _, v := range buf {
			if v != 0 {
				hasNonZero = true
			}
			if math.Abs(float64(v)) > 1.0 { // should be reasonably bounded
				_ = n.Close()
				t.Fatalf("sample %f exceeds reasonable bounds for %v", v, ntype)
			}
		}
		if !hasNonZero {
			_ = n.Close()
			t.Fatalf("noise %v generated all zeroes", ntype)
		}

		if err := n.Close(); err != nil {
			t.Fatalf("Close(%v): %v", ntype, err)
		}
	}
}

func TestNoiseControls(t *testing.T) {
	lib := newNullLibrary(t)

	n, err := lib.NewNoise(NoiseConfig{
		Format:    FormatF32,
		Channels:  1,
		Type:      NoiseTypeWhite,
		Seed:      100,
		Amplitude: 0.5,
	})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}
	defer func() { _ = n.Close() }()

	if err := n.SetAmplitude(0.2); err != nil {
		t.Errorf("SetAmplitude: %v", err)
	}

	// Reseeding to the same initial seed should repeat the pseudo-random sequence
	if err := n.SetSeed(100); err != nil {
		t.Errorf("SetSeed: %v", err)
	}

	// miniaudio rejects dynamic SetType with MA_INVALID_OPERATION
	if err := n.SetType(NoiseTypePink); err == nil {
		t.Error("SetType expected error (unsupported by miniaudio), got nil")
	}
}

func TestNoiseLifecycle(t *testing.T) {
	lib := newNullLibrary(t)

	n, err := lib.NewNoise(NoiseConfig{
		Format:    FormatF32,
		Channels:  1,
		Type:      NoiseTypeWhite,
		Seed:      1,
		Amplitude: 0.5,
	})
	if err != nil {
		t.Fatalf("NewNoise: %v", err)
	}

	if err := n.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := n.Close(); err != nil {
		t.Fatalf("double Close should succeed: %v", err)
	}

	var buf [4]float32
	if _, err := n.ReadPCMFrames(unsafe.Pointer(&buf[0]), 4); err == nil {
		t.Fatal("read on closed noise should return error")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
go test -run TestNoise ./
```
Expected: FAIL (compile errors: undefined `NoiseConfig`, `NewNoise`, etc.).

- [ ] **Step 3: Implement `noise.go` and update `unsupported.go`**

Create `noise.go`:
```go
//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// NoiseConfig describes how a noise generator should be initialized.
type NoiseConfig struct {
	Format            Format
	Channels          uint32
	Type              NoiseType
	Seed              int32
	Amplitude         float64
	DuplicateChannels bool
}

// Noise generates white, pink, or brownian noise.
type Noise struct {
	lib    *Library
	handle *noiseHandle
}

// NewNoise creates and initializes a new Noise generator.
func (lib *Library) NewNoise(config NoiseConfig) (*Noise, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	native := noiseConfigNative{
		Format:            config.Format,
		Channels:          config.Channels,
		Type:              config.Type,
		Seed:              config.Seed,
		Amplitude:         config.Amplitude,
		DuplicateChannels: boolToBool32(config.DuplicateChannels),
	}

	handle := (*noiseHandle)(lib.bindings.magoAlloc(magoObjectNoise))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate noise: out of memory")
	}

	result := lib.bindings.maNoiseInit(&native, nil, handle)
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_noise_init", result)
	}

	return &Noise{lib: lib, handle: handle}, nil
}

// ReadPCMFrames writes up to frameCount frames into out, returning how many were written.
func (n *Noise) ReadPCMFrames(out unsafe.Pointer, frameCount uint64) (uint64, error) {
	if n == nil || n.handle == nil {
		return 0, fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var framesRead uint64
	result := n.lib.bindings.maNoiseReadPCMFrames(n.handle, out, frameCount, &framesRead)
	if result != Success {
		return framesRead, n.lib.resultError("ma_noise_read_pcm_frames", result)
	}
	return framesRead, nil
}

// SetAmplitude updates the noise amplitude.
func (n *Noise) SetAmplitude(amplitude float64) error {
	if n == nil || n.handle == nil {
		return fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	return n.lib.resultError("ma_noise_set_amplitude", n.lib.bindings.maNoiseSetAmplitude(n.handle, amplitude))
}

// SetSeed updates the random number generator seed.
func (n *Noise) SetSeed(seed int32) error {
	if n == nil || n.handle == nil {
		return fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	return n.lib.resultError("ma_noise_set_seed", n.lib.bindings.maNoiseSetSeed(n.handle, seed))
}

// SetType attempts to change the noise type dynamically.
// Note: miniaudio 0.11.25 does not support dynamic noise type changes and will return ResultInvalidOperation.
func (n *Noise) SetType(noiseType NoiseType) error {
	if n == nil || n.handle == nil {
		return fmt.Errorf("mago: nil noise")
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	return n.lib.resultError("ma_noise_set_type", n.lib.bindings.maNoiseSetType(n.handle, noiseType))
}

// Close uninitializes the noise generator and frees its native memory.
func (n *Noise) Close() error {
	if n == nil || n.handle == nil {
		return nil
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}
	n.lib.bindings.maNoiseUninit(n.handle, nil)
	n.lib.bindings.magoFree(unsafe.Pointer(n.handle))
	n.handle = nil
	return nil
}
```

In `unsupported.go`:
```go
type NoiseConfig struct {
	Format            Format
	Channels          uint32
	Type              NoiseType
	Seed              int32
	Amplitude         float64
	DuplicateChannels bool
}
type Noise struct{}

func (*Library) NewNoise(NoiseConfig) (*Noise, error) { return nil, errUnsupportedPlatform }
func (*Noise) ReadPCMFrames(unsafe.Pointer, uint64) (uint64, error) { return 0, errUnsupportedPlatform }
func (*Noise) SetAmplitude(float64) error { return errUnsupportedPlatform }
func (*Noise) SetSeed(int32) error { return errUnsupportedPlatform }
func (*Noise) SetType(NoiseType) error { return errUnsupportedPlatform }
func (*Noise) Close() error { return errUnsupportedPlatform }
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run TestNoise ./
```
Expected: PASS.

- [ ] **Step 5: Commit and close task `mago-8a3.6.2`**

```bash
bd update mago-8a3.6.2 --claim
git add noise.go noise_test.go unsupported.go
git commit -m "feat: expose ma_noise data source"
bd close mago-8a3.6.2 --reason="Implemented Noise data source wrapper with seed, amplitude, types and tests"
```

---

## Task 4: Migrate `examples/tones` and add tests

**Files:**
- Modify: `examples/tones/main.go`
- Create: `examples/tones/main_test.go`

- [ ] **Step 1: Update `examples/tones/main.go` to use `ma_waveform`**

Replace the manual phase accumulation and math.Sin loop with `mago.Waveform`:
- Add flags `--waveform` (default `"sine"`) and `--freq` (default `440.0`).
- Map the waveform flag to `mago.WaveformType`:
  - `"sine"` -> `mago.WaveformTypeSine`
  - `"square"` -> `mago.WaveformTypeSquare`
  - `"triangle"` -> `mago.WaveformTypeTriangle`
  - `"sawtooth"` -> `mago.WaveformTypeSawtooth`
- Initialize `waveform, err := lib.NewWaveform(mago.WaveformConfig{...})`.
- In `config.DataCallback`:
  ```go
  config.DataCallback = func(_ *mago.Device, output unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
      _, _ = waveform.ReadPCMFrames(output, uint64(frameCount))
  }
  ```
- Make sure `defer waveform.Close()` is called.

- [ ] **Step 2: Add `examples/tones/main_test.go`**

```go
package main

import (
	"testing"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
)

func TestTonesWithNullBackend(t *testing.T) {
	lib, err := mago.Open()
	if err != nil {
		t.Skipf("mago.Open: %v (embedded library not available on this platform)", err)
	}
	defer func() { _ = lib.Close() }()

	ctx, err := lib.NewContext(mago.BackendNull)
	if err != nil {
		t.Fatalf("NewContext(BackendNull): %v", err)
	}
	defer func() { _ = ctx.Close() }()

	waveform, err := lib.NewWaveform(mago.WaveformConfig{
		Format:     mago.FormatF32,
		Channels:   2,
		SampleRate: 48000,
		Type:       mago.WaveformTypeSine,
		Amplitude:  0.2,
		Frequency:  440.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = waveform.Close() }()

	config := mago.DefaultPlaybackDeviceConfig()
	config.Channels = 2
	config.SampleRate = 48000
	config.PeriodSizeInFrames = 256
	config.DataCallback = func(_ *mago.Device, output unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
		_, _ = waveform.ReadPCMFrames(output, uint64(frameCount))
	}

	device, err := ctx.NewPlaybackDevice(config)
	if err != nil {
		t.Fatalf("NewPlaybackDevice: %v", err)
	}
	defer func() { _ = device.Close() }()

	if err := device.Start(); err != nil {
		t.Fatalf("device.Start: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := device.Stop(); err != nil {
		t.Fatalf("device.Stop: %v", err)
	}
}
```

- [ ] **Step 3: Run tests and build examples**

```bash
go test ./examples/tones/...
go build ./examples/...
```
Expected: PASS.

- [ ] **Step 4: Commit and close task `mago-8a3.6.3`**

```bash
bd update mago-8a3.6.3 --claim
git add examples/tones/main.go examples/tones/main_test.go
git commit -m "refactor(examples): migrate examples/tones to ma_waveform"
bd close mago-8a3.6.3 --reason="Migrated tones example to ma_waveform and added null-backend test"
```

---

## Task 5: Phase 5 Verification and Session Close

- [ ] **Step 1: Run CGO guard**

```bash
mise run check-cgo
```
Expected: PASS (0 cgo usages found).

- [ ] **Step 2: Run linter**

```bash
mise run lint
```
Expected: PASS (0 issues found).

- [ ] **Step 3: Run full test suite**

```bash
go test ./...
```
Expected: PASS across all packages.

- [ ] **Step 4: Check git status for drift**

```bash
git status
```
Expected: Clean working tree (except untracked scratch/transcripts if any).

- [ ] **Step 5: Close Phase 5 Epic in Beads**

```bash
bd close mago-8a3.6 --reason="Phase 5 complete: Waveform and Noise data sources exposed and tested, examples/tones migrated"
```
