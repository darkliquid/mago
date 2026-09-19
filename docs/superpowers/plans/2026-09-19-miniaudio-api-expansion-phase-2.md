# Phase 2: PCM Format Conversion and Channel Mapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the F32-only limitation by exposing miniaudio's PCM sample conversion, full-frame format conversion, channel maps and channel converter.

**Architecture:** Most of this is pure functions over caller-provided buffers, bound directly through purego. The one object, `ma_channel_converter`, is allocated through `mago_alloc` like every other object. The small 48-byte config struct is mirrored in Go (validated by the Phase 0 layout probe) rather than adding a C setter shim, keeping this phase's C change to a single allocation enum entry.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 2)
**Predecessors:** `.../2026-09-19-miniaudio-api-expansion-phase-0.md`, `.../phase-1.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** The only C change in this phase is one `mago_object_type` enum value and its `mago_alloc` case. Everything else binds `ma_*` directly.
- **One native rebuild for the phase** (Task 1), then `mise run build-lib-all && mise run generate`.
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Mirrors are validated.** `ma_channel_converter_config` and `ma_channel` join the layout probe.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

- `ma_channel` is `typedef ma_uint8 ma_channel;` - channel maps are byte arrays, not int arrays.
- `sizeof(ma_channel_converter_config) = 48`, `sizeof(ma_channel_converter) = 72`.
- `ma_channel_converter_config_init` returns by value and takes a single `format` for both sides; the struct also has `calculateLFEFromSpatialChannels` and `float** ppWeights` (only for `ma_channel_mix_mode_custom_weights`).
- `ma_channel_converter_init(config, callbacks, converter)` uses the default allocator when `callbacks` is NULL, so Go only needs to allocate the 72-byte converter object.
- `ma_pcm_convert` and `ma_convert_pcm_frames_format` convert between any of u8/s16/s24/s32/f32 and apply `ma_dither_mode`.
- `ma_convert_frames` converts format + channels + sample rate in one call and returns the frames written.
- `ma_convert_frames_ex` needs `ma_data_converter_config` (120 bytes); it belongs to Phase 3, not here.
- The per-format `ma_pcm_<src>_to_<dst>` helpers are redundant with `ma_pcm_convert`; do not bind all 57 of them.

---

## Task 1: Phase 2 bindings, mirrors and allocation (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c` (one enum value + one case)
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `MAGO_OBJECT_CHANNEL_CONVERTER = 6` in `mago_object_type` and its `mago_alloc` case.
- Produces Go types: `Channel`, `ChannelMap`, `DitherMode`, `ChannelMixMode`, `StandardChannelMap`, and the constants below.
- Produces Go mirror: `channelConverterConfigNative`.
- Produces bindings: `maPCMConvert`, `maConvertPCMFramesFormat`, `maConvertFrames`, `maChannelMapInitStandard`, `maChannelMapInitBlank`, `maChannelMapCopy`, `maChannelMapCopyOrDefault`, `maChannelMapGetChannel`, `maChannelMapToString`, `maChannelConverterInit`, `maChannelConverterUninit`, `maChannelConverterProcessPCMFrames`, `maChannelConverterGetInputChannelMap`, `maChannelConverterGetOutputChannelMap`.

- [ ] **Step 1: Add the allocation type in C**

```c
    MAGO_OBJECT_CHANNEL_CONVERTER = 6
```

```c
        case MAGO_OBJECT_CHANNEL_CONVERTER: return calloc(1, sizeof(ma_channel_converter));
```

Add a one-line comment noting it is an opaque object for the channel converter.

- [ ] **Step 2: Extend the layout probe**

Append to `internal/abi/layout_probe.c`:

```c
    printf("sizeof:ma_channel %zu\n", sizeof(ma_channel));
    printf("sizeof:ma_channel_converter_config %zu\n", sizeof(ma_channel_converter_config));
    printf("offsetof:ma_channel_converter_config.pChannelMapIn %zu\n", offsetof(ma_channel_converter_config, pChannelMapIn));
    printf("offsetof:ma_channel_converter_config.pChannelMapOut %zu\n", offsetof(ma_channel_converter_config, pChannelMapOut));
    printf("offsetof:ma_channel_converter_config.mixingMode %zu\n", offsetof(ma_channel_converter_config, mixingMode));
    printf("offsetof:ma_channel_converter_config.calculateLFEFromSpatialChannels %zu\n", offsetof(ma_channel_converter_config, calculateLFEFromSpatialChannels));
    printf("offsetof:ma_channel_converter_config.ppWeights %zu\n", offsetof(ma_channel_converter_config, ppWeights));
```

- [ ] **Step 3: Add the Go types and mirror to `types.go`**

```go
// Channel mirrors ma_channel, which miniaudio typedefs to ma_uint8.
type Channel uint8

const (
	ChannelNone             Channel = 0
	ChannelMono             Channel = 1
	ChannelFrontLeft        Channel = 2
	ChannelFrontRight       Channel = 3
	ChannelFrontCenter      Channel = 4
	ChannelLFE              Channel = 5
	ChannelBackLeft         Channel = 6
	ChannelBackRight        Channel = 7
	ChannelFrontLeftCenter  Channel = 8
	ChannelFrontRightCenter Channel = 9
	ChannelBackCenter       Channel = 10
	ChannelSideLeft         Channel = 11
	ChannelSideRight        Channel = 12
	ChannelTopCenter        Channel = 13
	ChannelTopFrontLeft     Channel = 14
	ChannelTopFrontCenter   Channel = 15
	ChannelTopFrontRight    Channel = 16
	ChannelTopBackLeft      Channel = 17
	ChannelTopBackCenter    Channel = 18
	ChannelTopBackRight     Channel = 19
	ChannelAux0             Channel = 20
	// ChannelAux1 .. ChannelAux31 continue at 21..51.
	ChannelPositionCount Channel = 52
	ChannelLeft          Channel = ChannelFrontLeft
	ChannelRight         Channel = ChannelFrontRight
)
```

The AUX range is contiguous: define all of `ChannelAux0`..`ChannelAux31` (20..51)
explicitly so the mirror matches the header exactly.

```go
type DitherMode int32

const (
	DitherModeNone      DitherMode = 0
	DitherModeRectangle DitherMode = 1
	DitherModeTriangle  DitherMode = 2
)

type ChannelMixMode int32

const (
	ChannelMixModeRectangular   ChannelMixMode = 0
	ChannelMixModeSimple        ChannelMixMode = 1
	ChannelMixModeCustomWeights ChannelMixMode = 2
	ChannelMixModeDefault       ChannelMixMode = ChannelMixModeRectangular
)

type StandardChannelMap int32

const (
	StandardChannelMapMicrosoft StandardChannelMap = 0
	StandardChannelMapALSA      StandardChannelMap = 1
	StandardChannelMapRFC3551   StandardChannelMap = 2
	StandardChannelMapFLAC      StandardChannelMap = 3
	StandardChannelMapVorbis    StandardChannelMap = 4
	StandardChannelMapSound4    StandardChannelMap = 5
	StandardChannelMapSndio     StandardChannelMap = 6
	StandardChannelMapDefault   StandardChannelMap = StandardChannelMapMicrosoft
)

type channelConverterConfigNative struct {
	Format                          Format
	ChannelsIn                      uint32
	ChannelsOut                     uint32
	ChannelMapIn                    *uint8   // *ma_channel
	ChannelMapOut                   *uint8   // *ma_channel
	MixingMode                      ChannelMixMode
	CalculateLFEFromSpatialChannels uint32
	Weights                         **float32 // ppWeights, only used by custom weights
}

const magoObjectChannelConverter int32 = 6
```

Verify the full `ma_channel` enum against the header and extend the constants to
cover every named channel before finishing this step.

- [ ] **Step 4: Extend the bindings generator**

```go
{FieldName: "maPCMConvert", Symbol: "ma_pcm_convert", Type: "func(unsafe.Pointer, Format, unsafe.Pointer, Format, uint64, DitherMode)"},
{FieldName: "maConvertPCMFramesFormat", Symbol: "ma_convert_pcm_frames_format", Type: "func(unsafe.Pointer, Format, unsafe.Pointer, Format, uint64, uint32, DitherMode)"},
{FieldName: "maConvertFrames", Symbol: "ma_convert_frames", Type: "func(unsafe.Pointer, uint64, Format, uint32, uint32, unsafe.Pointer, uint64, Format, uint32, uint32) uint64"},
{FieldName: "maChannelMapInitStandard", Symbol: "ma_channel_map_init_standard", Type: "func(StandardChannelMap, *uint8, uintptr, uint32)"},
{FieldName: "maChannelMapInitBlank", Symbol: "ma_channel_map_init_blank", Type: "func(*uint8, uint32)"},
{FieldName: "maChannelMapCopy", Symbol: "ma_channel_map_copy", Type: "func(*uint8, *uint8, uint32)"},
{FieldName: "maChannelMapCopyOrDefault", Symbol: "ma_channel_map_copy_or_default", Type: "func(*uint8, uintptr, *uint8, uint32)"},
{FieldName: "maChannelMapGetChannel", Symbol: "ma_channel_map_get_channel", Type: "func(*uint8, uint32, uint32) Channel"},
{FieldName: "maChannelMapToString", Symbol: "ma_channel_map_to_string", Type: "func(*uint8, uint32, *byte, uintptr) uintptr"},
{FieldName: "maChannelConverterInit", Symbol: "ma_channel_converter_init", Type: "func(*channelConverterConfigNative, unsafe.Pointer, *channelConverterHandle) Result"},
{FieldName: "maChannelConverterUninit", Symbol: "ma_channel_converter_uninit", Type: "func(*channelConverterHandle, unsafe.Pointer)"},
{FieldName: "maChannelConverterProcessPCMFrames", Symbol: "ma_channel_converter_process_pcm_frames", Type: "func(*channelConverterHandle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
{FieldName: "maChannelConverterGetInputChannelMap", Symbol: "ma_channel_converter_get_input_channel_map", Type: "func(*channelConverterHandle, *uint8, uintptr) Result"},
{FieldName: "maChannelConverterGetOutputChannelMap", Symbol: "ma_channel_converter_get_output_channel_map", Type: "func(*channelConverterHandle, *uint8, uintptr) Result"},
```

Add `type channelConverterHandle struct{}` to `types.go`.

- [ ] **Step 5: Regenerate, build, rebuild libraries, verify**

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
git status --short
```

Expected: only the bridge, generator, `types.go`, generated files, probe and
`layout_test.go` change; no diff in unrelated files.

- [ ] **Step 6: Add the layout assertions**

In `layout_test.go`, assert the Go mirror against the probe:

```go
	if got, want := uint64(unsafe.Sizeof(Channel(0))), probe["sizeof:ma_channel"]; got != want {
		t.Errorf("sizeof ma_channel: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(channelConverterConfigNative{})), probe["sizeof:ma_channel_converter_config"]; got != want {
		t.Errorf("sizeof channelConverterConfigNative: mirror %d, header %d", got, want)
	}

	mirror := channelConverterConfigNative{}
	offsets := map[string]struct {
		got  uintptr
		key  string
	}{
		"pChannelMapIn":                   {unsafe.Offsetof(mirror.ChannelMapIn), "offsetof:ma_channel_converter_config.pChannelMapIn"},
		"pChannelMapOut":                  {unsafe.Offsetof(mirror.ChannelMapOut), "offsetof:ma_channel_converter_config.pChannelMapOut"},
		"mixingMode":                      {unsafe.Offsetof(mirror.MixingMode), "offsetof:ma_channel_converter_config.mixingMode"},
		"calculateLFEFromSpatialChannels": {unsafe.Offsetof(mirror.CalculateLFEFromSpatialChannels), "offsetof:ma_channel_converter_config.calculateLFEFromSpatialChannels"},
		"ppWeights":                       {unsafe.Offsetof(mirror.Weights), "offsetof:ma_channel_converter_config.ppWeights"},
	}
	for name, check := range offsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_channel_converter_config.%s: mirror %d, header %d", name, got, want)
		}
	}
```

Run: `go test . -run TestMirroredStructLayouts -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go \
  internal/abi/layout_probe.c layout_test.go zz_generated.bindings.go embed_*.go
git commit -m "refactor: add Phase 2 bindings, mirrors and converter allocation"
```

---

## Task 2: PCM sample conversion

**Files:**
- Create: `pcm.go`
- Create: `pcm_test.go`

**Interfaces:**
- Consumes: `maPCMConvert`, `maConvertPCMFramesFormat`.
- Produces: `func (lib *Library) ConvertPCMSamples(out unsafe.Pointer, formatOut Format, in unsafe.Pointer, formatIn Format, sampleCount uint64, dither DitherMode)`.
- Produces: `func (lib *Library) ConvertPCMFramesFormat(out unsafe.Pointer, formatOut Format, in unsafe.Pointer, formatIn Format, frameCount uint64, channels uint32, dither DitherMode)`.
- Produces: `func BytesPerSample(format Format) uint32`.

- [ ] **Step 1: Write the failing test**

```go
// pcm_test.go
package mago

import (
	"math"
	"testing"
	"unsafe"
)

func TestConvertPCMSamplesF32ToS16(t *testing.T) {
	lib := newNullLibrary(t)

	in := []float32{0, 0.5, -0.5, 1, -1}
	out := make([]int16, len(in))

	lib.ConvertPCMSamples(
		unsafe.Pointer(&out[0]), FormatS16,
		unsafe.Pointer(&in[0]), FormatF32,
		uint64(len(in)), DitherModeNone,
	)

	want := []int16{0, 16384, -16384, 32767, -32768}
	for i := range want {
		if diff := int(out[i]) - int(want[i]); diff > 1 || diff < -1 {
			t.Errorf("sample %d = %d, want ~%d", i, out[i], want[i])
		}
	}
}

func TestBytesPerSample(t *testing.T) {
	cases := map[Format]uint32{
		FormatU8: 1, FormatS16: 2, FormatS24: 3, FormatS32: 4, FormatF32: 4,
	}
	for format, want := range cases {
		if got := BytesPerSample(format); got != want {
			t.Errorf("BytesPerSample(%v) = %d, want %d", format, got, want)
		}
	}
	_ = math.MaxInt16
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run 'TestConvertPCMSamplesF32ToS16|TestBytesPerSample' -v`
Expected: FAIL - `lib.ConvertPCMSamples undefined`.

- [ ] **Step 3: Implement**

`ConvertPCMSamples` and `ConvertPCMFramesFormat` guard on `ensureOpen()` then
call the bindings; both take `unsafe.Pointer` because the buffers may be any
PCM format. `BytesPerSample` maps `Format` to 1/2/3/4/4 and returns 0 for
unknown.

- [ ] **Step 4: Run the tests**

Run: `go test . -run 'TestConvertPCMSamplesF32ToS16|TestBytesPerSample' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pcm.go pcm_test.go
git commit -m "feat: expose PCM sample and frame format conversion"
```

---

## Task 3: Full-frame format conversion

**Files:**
- Modify: `pcm.go`
- Modify: `pcm_test.go`

**Interfaces:**
- Consumes: `maConvertFrames`.
- Produces: `func (lib *Library) ConvertFrames(out unsafe.Pointer, frameCountOut uint64, formatOut Format, channelsOut uint32, sampleRateOut uint32, in unsafe.Pointer, frameCountIn uint64, formatIn Format, channelsIn uint32, sampleRateIn uint32) uint64`.

- [ ] **Step 1: Write the failing test**

```go
func TestConvertFramesStereoF32ToMonoS16(t *testing.T) {
	lib := newNullLibrary(t)

	const framesIn = 4
	in := []float32{0.5, -0.5, 0.25, -0.25, 0, 0, 1, -1}
	out := make([]int16, framesIn)

	written := lib.ConvertFrames(
		unsafe.Pointer(&out[0]), framesIn, FormatS16, 1, 48_000,
		unsafe.Pointer(&in[0]), framesIn, FormatF32, 2, 48_000,
	)
	if written != framesIn {
		t.Fatalf("wrote %d frames, want %d", written, framesIn)
	}
	if out[0] == 0 && out[1] == 0 {
		t.Fatal("expected non-silent output from a down-mix of non-zero input")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestConvertFramesStereoF32ToMonoS16 -v`
Expected: FAIL - `lib.ConvertFrames undefined`.

- [ ] **Step 3: Implement `ConvertFrames`**

Guard on `ensureOpen()`, call `maConvertFrames`, and return the frame count
miniaudio reports. `ma_convert_frames` returns 0 with no output when the buffer
is undersized, so document that the caller must size the output using the
returned value only after the call.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestConvertFramesStereoF32ToMonoS16 -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pcm.go pcm_test.go
git commit -m "feat: add one-call frame format, channel and rate conversion"
```

---

## Task 4: Channel maps

**Files:**
- Create: `channel.go`
- Create: `channel_test.go`

**Interfaces:**
- Consumes: `maChannelMapInitStandard`, `maChannelMapInitBlank`, `maChannelMapCopy`, `maChannelMapCopyOrDefault`, `maChannelMapGetChannel`, `maChannelMapToString`.
- Produces: `func NewStandardChannelMap(std StandardChannelMap, channels uint32) ChannelMap`.
- Produces: `func NewBlankChannelMap(channels uint32) ChannelMap`.
- Produces: `func (m ChannelMap) Clone() ChannelMap`, `func (m ChannelMap) Get(i int) Channel`, `func (m ChannelMap) String() string`.
- Produces: `type ChannelMap []Channel`.

- [ ] **Step 1: Write the failing test**

```go
// channel_test.go
package mago

import "testing"

func TestStandardStereoChannelMap(t *testing.T) {
	m := NewStandardChannelMap(StandardChannelMapMicrosoft, 2)
	if len(m) != 2 {
		t.Fatalf("length %d, want 2", len(m))
	}
	if m.Get(0) != ChannelFrontLeft || m.Get(1) != ChannelFrontRight {
		t.Fatalf("stereo map = %v, want front-left, front-right", m)
	}
	if m.String() == "" {
		t.Fatal("expected a non-empty channel-map string")
	}
}

func TestBlankChannelMap(t *testing.T) {
	m := NewBlankChannelMap(4)
	for i := range m {
		if m.Get(i) != ChannelNone {
			t.Fatalf("blank map[%d] = %v, want none", i, m.Get(i))
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run 'TestStandardStereoChannelMap|TestBlankChannelMap' -v`
Expected: FAIL - `undefined: NewStandardChannelMap`.

- [ ] **Step 3: Implement**

`NewStandardChannelMap` allocates a `[]Channel` and calls
`maChannelMapInitStandard`. `NewBlankChannelMap` does the same with
`maChannelMapInitBlank`. `Clone` calls `maChannelMapCopy` into a fresh slice.
`Get` calls `maChannelMapGetChannel`. `String` calls `maChannelMapToString`
into a `[512]byte` buffer.

`strings`/`unsafe` conversions: `ChannelMap` is `[]Channel` and `Channel` is
`uint8`, so `&m[0]` is the `*uint8` the bindings expect.

- [ ] **Step 4: Run the tests**

Run: `go test . -run 'TestStandardStereoChannelMap|TestBlankChannelMap' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add channel.go channel_test.go
git commit -m "feat: expose channel maps and standard layouts"
```

---

## Task 5: Channel converter

**Files:**
- Modify: `channel.go`
- Modify: `channel_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectChannelConverter)`, `magoFree`, `maChannelConverterInit`, `maChannelConverterUninit`, `maChannelConverterProcessPCMFrames`, `maChannelConverterGetInputChannelMap`, `maChannelConverterGetOutputChannelMap`.
- Produces: `type ChannelConverterConfig struct { Format Format; ChannelsIn, ChannelsOut uint32; ChannelMapIn, ChannelMapOut ChannelMap; MixingMode ChannelMixMode; CalculateLFEFromSpatialChannels bool }`.
- Produces: `func (lib *Library) NewChannelConverter(config ChannelConverterConfig) (*ChannelConverter, error)`.
- Produces: methods `ProcessPCMFrames(out, in unsafe.Pointer, frameCount uint64) error`, `InputChannelMap() (ChannelMap, error)`, `OutputChannelMap() (ChannelMap, error)`, `Close() error`.

- [ ] **Step 1: Write the failing test**

```go
func TestChannelConverterStereoToMono(t *testing.T) {
	lib := newNullLibrary(t)

	converter, err := lib.NewChannelConverter(ChannelConverterConfig{
		Format:      FormatF32,
		ChannelsIn:  2,
		ChannelsOut: 1,
		MixingMode:  ChannelMixModeRectangular,
	})
	if err != nil {
		t.Fatalf("NewChannelConverter: %v", err)
	}
	defer func() { _ = converter.Close() }()

	in := []float32{0.5, 0.5, -0.5, -0.5}
	out := make([]float32, 2)

	if err := converter.ProcessPCMFrames(
		unsafe.Pointer(&out[0]), unsafe.Pointer(&in[0]), 2,
	); err != nil {
		t.Fatalf("ProcessPCMFrames: %v", err)
	}
	if out[0] == 0 {
		t.Fatal("expected a non-zero down-mix")
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

Run: `go test . -run TestChannelConverterStereoToMono -v`
Expected: FAIL - `lib.NewChannelConverter undefined`.

- [ ] **Step 3: Implement**

`NewChannelConverter` allocates the converter through
`magoAlloc(magoObjectChannelConverter)`, builds a `channelConverterConfigNative`
from the public config (converting `ChannelMapIn`/`ChannelMapOut` to `*uint8`,
or nil when empty), and calls `maChannelConverterInit(&config, nil, handle)`.
On failure it frees the handle. `Close` calls `maChannelConverterUninit` then
`magoFree`. Custom weights are not supported yet: reject
`ChannelMixModeCustomWeights` with a clear error.

`InputChannelMap`/`OutputChannelMap` allocate a `[]Channel` sized by
`ChannelsIn`/`ChannelsOut` and call the getter into it.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestChannelConverterStereoToMono -v`
Expected: PASS.

- [ ] **Step 5: Add unsupported-platform stubs**

In `unsupported.go`, add `ChannelMap`, `ChannelConverter`, its config, and
`func (*Library) NewChannelConverter(...)` returning `errUnsupportedPlatform`.
Add the free functions (`NewStandardChannelMap`, `NewBlankChannelMap`,
`BytesPerSample`, and the `Library` conversion methods) where they are exported
on supported platforms so non-supported GOOS still type-checks.

- [ ] **Step 6: Full test, lint, and commit**

Run: `mise run test && mise run lint && mise run check-cgo`
Expected: PASS, 0 lint issues.

```bash
git add channel.go channel_test.go unsupported.go
git commit -m "feat: expose the channel converter"
```

---

## Phase 2 exit criteria

- `ConvertPCMSamples`, `ConvertPCMFramesFormat` and `ConvertFrames` convert
  between every format pair the tests cover, including a channel-count change.
- `ChannelMap` constructors, `Clone`, `Get` and `String` work.
- `ChannelConverter` down/up-mixes and reports its input/output maps.
- `mise run test`, `mise run lint` and `mise run check-cgo` are green.
- `mise run build-lib-all && mise run generate` leaves `git diff --exit-code` clean.
- `bd ready` surfaces Phase 3 (`mago-8a3.4`) once Phase 2 closes.
