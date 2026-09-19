# Phase 3: Resampling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose sample-rate conversion: the linear resampler, the general resampler, and the one-call `ma_data_converter` pipeline.

**Architecture:** Three opaque objects allocated through `mago_alloc`, plus three small config structs mirrored in Go and validated by the layout probe. No config setter shims: the only C change is three allocation enum values, so the phase costs a single native rebuild.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 3)
**Predecessors:** `.../phase-0.md`, `.../phase-1.md`, `.../phase-2.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** Three `mago_object_type` values plus their `mago_alloc` cases. Nothing else.
- **One native rebuild for the phase** (Task 1).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Mirrors are validated** by `internal/abi` + `layout_test.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

| Type | Size |
| --- | ---: |
| `ma_resampler_config` | 48 |
| `ma_resampler` | 192 |
| `ma_linear_resampler_config` | 32 |
| `ma_linear_resampler` | 136 |
| `ma_data_converter_config` | 120 (nested `resampling` at offset 72) |
| `ma_data_converter` | 312 |

- `ma_resampler_config_init` sets `linear.lpfOrder = min(MA_DEFAULT_RESAMPLER_LPF_ORDER=4, MA_MAX_FILTER_ORDER=8)` = 4.
- `ma_linear_resampler_config_init` sets `lpfOrder = 4` and `lpfNyquistFactor = 1`.
- `ma_data_converter_config_init_default` sets `ditherMode = none`, `resampling.algorithm = linear`, `resampling.linear.lpfOrder = 1`, `allowDynamicSampleRate = false`.
- Resampling is only required when `allowDynamicSampleRate` is set or the sample rates differ.
- `process_pcm_frames` takes `ma_uint64* pFrameCountIn` and `ma_uint64* pFrameCountOut` (in/out).
- Custom resampling backends (`ma_resample_algorithm_custom`) need a `ma_resampling_backend_vtable`; not supported here.
- `ma_resampler`/`ma_linear_resampler`/`ma_data_converter` use the default allocator when callbacks are NULL.

---

## Task 1: Phase 3 bindings, mirrors and allocation (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c` (three enum values + cases)
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `MAGO_OBJECT_RESAMPLER = 7`, `MAGO_OBJECT_LINEAR_RESAMPLER = 8`, `MAGO_OBJECT_DATA_CONVERTER = 9`.
- Produces Go: `ResampleAlgorithm`, `resamplerConfigNative`, `linearResamplerConfigNative`, `dataConverterConfigNative`, handles `resamplerHandle`, `linearResamplerHandle`, `dataConverterHandle`.
- Produces bindings for every function listed in the tasks below.

- [ ] **Step 1: Add the allocation types in C**

```c
    MAGO_OBJECT_RESAMPLER        = 7,
    MAGO_OBJECT_LINEAR_RESAMPLER = 8,
    MAGO_OBJECT_DATA_CONVERTER   = 9
```

```c
        case MAGO_OBJECT_RESAMPLER:        return calloc(1, sizeof(ma_resampler));
        case MAGO_OBJECT_LINEAR_RESAMPLER: return calloc(1, sizeof(ma_linear_resampler));
        case MAGO_OBJECT_DATA_CONVERTER:   return calloc(1, sizeof(ma_data_converter));
```

- [ ] **Step 2: Extend the layout probe**

```c
    printf("sizeof:ma_resampler_config %zu\n", sizeof(ma_resampler_config));
    printf("offsetof:ma_resampler_config.algorithm %zu\n", offsetof(ma_resampler_config, algorithm));
    printf("offsetof:ma_resampler_config.linear.lpfOrder %zu\n", offsetof(ma_resampler_config, linear.lpfOrder));
    printf("sizeof:ma_linear_resampler_config %zu\n", sizeof(ma_linear_resampler_config));
    printf("offsetof:ma_linear_resampler_config.lpfOrder %zu\n", offsetof(ma_linear_resampler_config, lpfOrder));
    printf("offsetof:ma_linear_resampler_config.lpfNyquistFactor %zu\n", offsetof(ma_linear_resampler_config, lpfNyquistFactor));
    printf("sizeof:ma_data_converter_config %zu\n", sizeof(ma_data_converter_config));
    printf("offsetof:ma_data_converter_config.pChannelMapIn %zu\n", offsetof(ma_data_converter_config, pChannelMapIn));
    printf("offsetof:ma_data_converter_config.ditherMode %zu\n", offsetof(ma_data_converter_config, ditherMode));
    printf("offsetof:ma_data_converter_config.channelMixMode %zu\n", offsetof(ma_data_converter_config, channelMixMode));
    printf("offsetof:ma_data_converter_config.calculateLFEFromSpatialChannels %zu\n", offsetof(ma_data_converter_config, calculateLFEFromSpatialChannels));
    printf("offsetof:ma_data_converter_config.ppChannelWeights %zu\n", offsetof(ma_data_converter_config, ppChannelWeights));
    printf("offsetof:ma_data_converter_config.allowDynamicSampleRate %zu\n", offsetof(ma_data_converter_config, allowDynamicSampleRate));
    printf("offsetof:ma_data_converter_config.resampling %zu\n", offsetof(ma_data_converter_config, resampling));
```

- [ ] **Step 3: Add the Go mirrors to `types.go`**

```go
type ResampleAlgorithm int32

const (
	ResampleAlgorithmLinear ResampleAlgorithm = 0
	ResampleAlgorithmCustom ResampleAlgorithm = 1
)

type resamplerConfigNative struct {
	Format          Format
	Channels        uint32
	SampleRateIn    uint32
	SampleRateOut   uint32
	Algorithm       ResampleAlgorithm
	BackendVTable   unsafe.Pointer
	BackendUserData unsafe.Pointer
	Linear          struct{ LPFOrder uint32 }
}

type linearResamplerConfigNative struct {
	Format           Format
	Channels         uint32
	SampleRateIn     uint32
	SampleRateOut    uint32
	LPFOrder         uint32
	LPFNyquistFactor float64
}

type dataConverterConfigNative struct {
	FormatIn                        Format
	FormatOut                       Format
	ChannelsIn                      uint32
	ChannelsOut                     uint32
	SampleRateIn                    uint32
	SampleRateOut                   uint32
	ChannelMapIn                    *uint8
	ChannelMapOut                   *uint8
	DitherMode                      DitherMode
	ChannelMixMode                  ChannelMixMode
	CalculateLFEFromSpatialChannels uint32
	ChannelWeights                  **float32
	AllowDynamicSampleRate          uint32
	Resampling                      resamplerConfigNative
}

type resamplerHandle struct{}
type linearResamplerHandle struct{}
type dataConverterHandle struct{}
```

Extend the object-type constant block with `magoObjectResampler = 7`,
`magoObjectLinearResampler = 8`, `magoObjectDataConverter = 9`.

- [ ] **Step 4: Add the bindings**

```go
{FieldName: "maResamplerInit", Symbol: "ma_resampler_init", Type: "func(*resamplerConfigNative, unsafe.Pointer, *resamplerHandle) Result"},
{FieldName: "maResamplerUninit", Symbol: "ma_resampler_uninit", Type: "func(*resamplerHandle, unsafe.Pointer)"},
{FieldName: "maResamplerProcessPCMFrames", Symbol: "ma_resampler_process_pcm_frames", Type: "func(*resamplerHandle, unsafe.Pointer, *uint64, unsafe.Pointer, *uint64) Result"},
{FieldName: "maResamplerSetRate", Symbol: "ma_resampler_set_rate", Type: "func(*resamplerHandle, uint32, uint32) Result"},
{FieldName: "maResamplerSetRateRatio", Symbol: "ma_resampler_set_rate_ratio", Type: "func(*resamplerHandle, float32) Result"},
{FieldName: "maResamplerReset", Symbol: "ma_resampler_reset", Type: "func(*resamplerHandle) Result"},
{FieldName: "maResamplerGetRequiredInputFrameCount", Symbol: "ma_resampler_get_required_input_frame_count", Type: "func(*resamplerHandle, uint64, *uint64) Result"},
{FieldName: "maResamplerGetExpectedOutputFrameCount", Symbol: "ma_resampler_get_expected_output_frame_count", Type: "func(*resamplerHandle, uint64, *uint64) Result"},
{FieldName: "maLinearResamplerInit", Symbol: "ma_linear_resampler_init", Type: "func(*linearResamplerConfigNative, unsafe.Pointer, *linearResamplerHandle) Result"},
{FieldName: "maLinearResamplerUninit", Symbol: "ma_linear_resampler_uninit", Type: "func(*linearResamplerHandle, unsafe.Pointer)"},
{FieldName: "maLinearResamplerProcessPCMFrames", Symbol: "ma_linear_resampler_process_pcm_frames", Type: "func(*linearResamplerHandle, unsafe.Pointer, *uint64, unsafe.Pointer, *uint64) Result"},
{FieldName: "maLinearResamplerSetRate", Symbol: "ma_linear_resampler_set_rate", Type: "func(*linearResamplerHandle, uint32, uint32) Result"},
{FieldName: "maLinearResamplerSetRateRatio", Symbol: "ma_linear_resampler_set_rate_ratio", Type: "func(*linearResamplerHandle, float32) Result"},
{FieldName: "maLinearResamplerReset", Symbol: "ma_linear_resampler_reset", Type: "func(*linearResamplerHandle) Result"},
{FieldName: "maLinearResamplerGetRequiredInputFrameCount", Symbol: "ma_linear_resampler_get_required_input_frame_count", Type: "func(*linearResamplerHandle, uint64, *uint64) Result"},
{FieldName: "maLinearResamplerGetExpectedOutputFrameCount", Symbol: "ma_linear_resampler_get_expected_output_frame_count", Type: "func(*linearResamplerHandle, uint64, *uint64) Result"},
{FieldName: "maDataConverterInit", Symbol: "ma_data_converter_init", Type: "func(*dataConverterConfigNative, unsafe.Pointer, *dataConverterHandle) Result"},
{FieldName: "maDataConverterUninit", Symbol: "ma_data_converter_uninit", Type: "func(*dataConverterHandle, unsafe.Pointer)"},
{FieldName: "maDataConverterProcessPCMFrames", Symbol: "ma_data_converter_process_pcm_frames", Type: "func(*dataConverterHandle, unsafe.Pointer, *uint64, unsafe.Pointer, *uint64) Result"},
{FieldName: "maDataConverterSetRate", Symbol: "ma_data_converter_set_rate", Type: "func(*dataConverterHandle, uint32, uint32) Result"},
{FieldName: "maDataConverterSetRateRatio", Symbol: "ma_data_converter_set_rate_ratio", Type: "func(*dataConverterHandle, float32) Result"},
{FieldName: "maDataConverterReset", Symbol: "ma_data_converter_reset", Type: "func(*dataConverterHandle) Result"},
{FieldName: "maDataConverterGetRequiredInputFrameCount", Symbol: "ma_data_converter_get_required_input_frame_count", Type: "func(*dataConverterHandle, uint64, *uint64) Result"},
{FieldName: "maDataConverterGetExpectedOutputFrameCount", Symbol: "ma_data_converter_get_expected_output_frame_count", Type: "func(*dataConverterHandle, uint64, *uint64) Result"},
{FieldName: "maDataConverterGetInputChannelMap", Symbol: "ma_data_converter_get_input_channel_map", Type: "func(*dataConverterHandle, *uint8, uintptr) Result"},
{FieldName: "maDataConverterGetOutputChannelMap", Symbol: "ma_data_converter_get_output_channel_map", Type: "func(*dataConverterHandle, *uint8, uintptr) Result"},
```

- [ ] **Step 5: Regenerate, rebuild, verify**

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
go test . -run TestMirroredStructLayouts -v
```

Add `unsafe.Offsetof` assertions for each new probe key to `layout_test.go`
(`Algorithm`, `Linear.LPFOrder`, `LPFOrder`, `LPFNyquistFactor`,
`ChannelMapIn`, `DitherMode`, `ChannelMixMode`,
`CalculateLFEFromSpatialChannels`, `ChannelWeights`, `AllowDynamicSampleRate`,
`Resampling`) and assert `unsafe.Sizeof` for all three config mirrors.

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go \
  internal/abi/layout_probe.c layout_test.go zz_generated.bindings.go embed_*.go
git commit -m "refactor: add Phase 3 bindings, mirrors and resampler allocation"
```

---

## Task 2: General resampler

**Files:**
- Create: `resample.go`
- Create: `resample_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectResampler)`, `maResampler*` bindings.
- Produces: `type ResamplerConfig struct { Format Format; Channels, SampleRateIn, SampleRateOut uint32; Algorithm ResampleAlgorithm; LinearLPFOrder uint32 }`.
- Produces: `func DefaultResamplerConfig(format Format, channels, sampleRateIn, sampleRateOut uint32) ResamplerConfig` (algorithm linear, `LinearLPFOrder` 4).
- Produces: `func (lib *Library) NewResampler(config ResamplerConfig) (*Resampler, error)` (rejects `ResampleAlgorithmCustom`).
- Produces: `Resampler` with `ProcessPCMFrames`, `SetRate`, `SetRateRatio`, `Reset`, `RequiredInputFrameCount`, `ExpectedOutputFrameCount`, `Close`.
- Produces: `ProcessPCMFrames(in unsafe.Pointer, framesIn uint64, out unsafe.Pointer, framesOut uint64) (usedIn, usedOut uint64, err error)`.

- [ ] **Step 1: Write the failing test**

```go
// resample_test.go
package mago

import (
	"testing"
	"unsafe"
)

func TestResamplerUpsamplesF32(t *testing.T) {
	lib := newNullLibrary(t)

	resampler, err := lib.NewResampler(DefaultResamplerConfig(FormatF32, 1, 24_000, 48_000))
	if err != nil {
		t.Fatalf("NewResampler: %v", err)
	}
	defer func() { _ = resampler.Close() }()

	in := make([]float32, 240)
	for i := range in {
		in[i] = float32(i) / 240
	}
	out := make([]float32, 960)

	usedIn, usedOut, err := resampler.ProcessPCMFrames(
		unsafe.Pointer(&in[0]), uint64(len(in)),
		unsafe.Pointer(&out[0]), uint64(len(out)),
	)
	if err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
	}
	if usedIn == 0 || usedOut == 0 {
		t.Fatalf("expected frames in and out, got %d and %d", usedIn, usedOut)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestResamplerUpsamplesF32 -v`
Expected: FAIL - `undefined: DefaultResamplerConfig`.

- [ ] **Step 3: Implement**

Allocate via `magoAlloc(magoObjectResampler)`, build a `resamplerConfigNative`,
call `maResamplerInit(&native, nil, handle)`, free on failure. `ProcessPCMFrames`
passes local `uint64` copies through pointers and returns the updated values.
`DefaultResamplerConfig` mirrors `ma_resampler_config_init` defaults.
`SetRateRatio` takes a `float64` and converts to `float32`.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestResamplerUpsamplesF32 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add resample.go resample_test.go
git commit -m "feat: expose the general resampler"
```

---

## Task 3: Linear resampler

**Files:**
- Modify: `resample.go`
- Modify: `resample_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectLinearResampler)`, `maLinearResampler*` bindings.
- Produces: `type LinearResamplerConfig struct { Format Format; Channels, SampleRateIn, SampleRateOut, LPFOrder uint32; LPFNyquistFactor float64 }`.
- Produces: `func DefaultLinearResamplerConfig(format Format, channels, sampleRateIn, sampleRateOut uint32) LinearResamplerConfig` (LPFOrder 4, factor 1).
- Produces: `func (lib *Library) NewLinearResampler(config LinearResamplerConfig) (*LinearResampler, error)`.
- Produces: `LinearResampler` with the same method set as `Resampler`.

- [ ] **Step 1: Write the failing test**

```go
func TestLinearResamplerPreservesToneEnergy(t *testing.T) {
	lib := newNullLibrary(t)

	resampler, err := lib.NewLinearResampler(DefaultLinearResamplerConfig(FormatF32, 1, 48_000, 24_000))
	if err != nil {
		t.Fatalf("NewLinearResampler: %v", err)
	}
	defer func() { _ = resampler.Close() }()

	in := make([]float32, 480)
	for i := range in {
		in[i] = 0.5 * float32(math.Sin(2*math.Pi*440*float64(i)/48_000))
	}
	out := make([]float32, 480)

	_, usedOut, err := resampler.ProcessPCMFrames(
		unsafe.Pointer(&in[0]), uint64(len(in)),
		unsafe.Pointer(&out[0]), uint64(len(out)),
	)
	if err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
	}

	var peak float32
	for i := 0; i < int(usedOut); i++ {
		if out[i] > peak {
			peak = out[i]
		}
	}
	if peak < 0.3 || peak > 0.6 {
		t.Fatalf("down-sampled peak %v, want roughly 0.5", peak)
	}
}
```

Add `"math"` to the test imports.

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestLinearResamplerPreservesToneEnergy -v`
Expected: FAIL - `undefined: DefaultLinearResamplerConfig`.

- [ ] **Step 3: Implement**

Same shape as `Resampler`, using `linearResamplerConfigNative` and the
`maLinearResampler*` bindings.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestLinearResamplerPreservesToneEnergy -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add resample.go resample_test.go
git commit -m "feat: expose the linear resampler"
```

---

## Task 4: Data converter

**Files:**
- Modify: `resample.go`
- Modify: `resample_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectDataConverter)`, `maDataConverter*` bindings.
- Produces: `type DataConverterConfig struct { FormatIn, FormatOut Format; ChannelsIn, ChannelsOut, SampleRateIn, SampleRateOut uint32; ChannelMapIn, ChannelMapOut ChannelMap; DitherMode DitherMode; ChannelMixMode ChannelMixMode; CalculateLFEFromSpatialChannels bool; AllowDynamicSampleRate bool; LinearLPFOrder uint32 }`.
- Produces: `func DefaultDataConverterConfig(formatIn, formatOut Format, channelsIn, channelsOut, sampleRateIn, sampleRateOut uint32) DataConverterConfig` (LPF order 1).
- Produces: `func (lib *Library) NewDataConverter(config DataConverterConfig) (*DataConverter, error)` (rejects `ChannelMixModeCustomWeights`).
- Produces: `DataConverter` with `ProcessPCMFrames`, `SetRate`, `SetRateRatio`, `Reset`, `RequiredInputFrameCount`, `ExpectedOutputFrameCount`, `InputChannelMap`, `OutputChannelMap`, `Close`.

- [ ] **Step 1: Write the failing test**

```go
func TestDataConverterChangesFormatChannelsAndRate(t *testing.T) {
	lib := newNullLibrary(t)

	converter, err := lib.NewDataConverter(DefaultDataConverterConfig(
		FormatF32, FormatS16, 2, 1, 48_000, 24_000,
	))
	if err != nil {
		t.Fatalf("NewDataConverter: %v", err)
	}
	defer func() { _ = converter.Close() }()

	in := make([]float32, 480)
	for i := range in {
		in[i] = 0.5
	}
	out := make([]int16, 240)

	_, usedOut, err := converter.ProcessPCMFrames(
		unsafe.Pointer(&in[0]), uint64(len(in)/2),
		unsafe.Pointer(&out[0]), uint64(len(out)),
	)
	if err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
	}
	if usedOut == 0 {
		t.Fatal("expected output frames")
	}
	if out[0] < 1000 {
		t.Fatalf("first output sample = %d, want a scaled-down 0.5", out[0])
	}

	if _, err := converter.InputChannelMap(); err != nil {
		t.Fatalf("InputChannelMap: %v", err)
	}
	if _, err := converter.OutputChannelMap(); err != nil {
		t.Fatalf("OutputChannelMap: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestDataConverterChangesFormatChannelsAndRate -v`
Expected: FAIL - `undefined: DefaultDataConverterConfig`.

- [ ] **Step 3: Implement**

Build a `dataConverterConfigNative` (including the nested resampler config with
`Algorithm` linear, `Linear.LPFOrder` from the config, and `BackendVTable` nil),
allocate the converter, call `maDataConverterInit`. Channel maps are passed as
`*uint8` when non-empty. `InputChannelMap`/`OutputChannelMap` allocate a
`ChannelMap` using `ChannelsIn`/`ChannelsOut`.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestDataConverterChangesFormatChannelsAndRate -v`
Expected: PASS.

- [ ] **Step 5: Add unsupported-platform stubs**

In `unsupported.go`, add `ResampleAlgorithm`, the three config structs, the three
object types, `NewResampler`/`NewLinearResampler`/`NewDataConverter` returning
`errUnsupportedPlatform`, and their methods, so non-supported GOOS still
type-checks.

- [ ] **Step 6: Full test, lint, and commit**

Run: `mise run test && mise run lint && mise run check-cgo`
Expected: PASS, 0 lint issues.

```bash
git add resample.go resample_test.go unsupported.go
git commit -m "feat: expose the data converter pipeline"
```

---

## Task 5: Examples and README

**Files:**
- Create: `examples/resample/main.go`
- Modify: `README.md`

**Interfaces:** none exported; uses the public API from Tasks 2-4.

- [ ] **Step 1: Write the example**

A single `examples/resample` command that:
1. builds a short 440 Hz tone,
2. up-samples 24 kHz -> 48 kHz with `Resampler`, reporting frames in/out,
3. down-samples 48 kHz -> 24 kHz with `LinearResampler`,
4. runs `DataConverter` from stereo f32 48 kHz to mono s16 24 kHz in one call,
5. prints `RequiredInputFrameCount`/`ExpectedOutputFrameCount` for a target.

It must load the embedded library via the shared `examples/internal/example`
helper and need no audio device.

- [ ] **Step 2: Run it headless**

Run: `env -i PATH=/usr/bin:/bin HOME="$HOME" go run ./examples/resample`
Expected: prints the frame counts and conversions without error.

- [ ] **Step 3: Document it**

Add a `go run ./examples/resample` block to the README demos section and note it
needs no audio device.

- [ ] **Step 4: Verify and commit**

Run: `go build -trimpath ./examples/... && go test ./... && mise run lint`
Expected: PASS.

```bash
git add examples/resample README.md
git commit -m "docs: add a resampling example"
```

---

## Phase 3 exit criteria

- General, linear and data-converter resampling work and return frame counts.
- `Default*Config` helpers reproduce miniaudio's documented defaults.
- Custom resampling backends and custom channel weights are rejected clearly.
- `mise run test`, `mise run lint`, `mise run check-cgo` are green.
- `mise run build-lib-all && mise run generate` leaves no diff.
- `bd ready` surfaces the next phases once Phase 3 closes.
