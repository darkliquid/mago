# Phase 7: Decoding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose `ma_decoder` so mago can decode WAV, FLAC and MP3, and wire it into the `audio` package so `Engine.Load` is no longer WAV-only.

**Architecture:** One opaque `Decoder` object allocated through `mago_alloc`, plus a Go mirror of `ma_decoder_config` (144 bytes, reusing the existing resampler and allocation-callback mirrors). Reads follow the repository's safe-slice convention (`ReadF32`/`ReadS16`). The `audio` package tries its pure-Go WAV path first and falls back to the decoder for everything else.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 7)
**Predecessors:** `.../phase-0.md` … `.../phase-6.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** One `mago_object_type` value plus its `mago_alloc` case.
- **Safe-slice APIs.** Public read/process methods take Go slices, never `unsafe.Pointer` (the convention set by the safe-slice work). Examples must not import `unsafe`.
- **One native rebuild for the phase** (Task 1).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Mirrors are validated** by `internal/abi` + `layout_test.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

| Type | Size |
| --- | ---: |
| `ma_decoder_config` | 144 |
| `ma_decoder` | 552 |
| `ma_encoding_format` | unknown 0, wav 1, flac 2, mp3 3, vorbis 4 |

- `ma_decoder_config` layout: `format`(0) `channels`(4) `sampleRate`(8) `pChannelMap`(16) `channelMixMode`(24) `ditherMode`(28) `resampling`(32, 48 bytes) `allocationCallbacks`(80, 32 bytes) `encodingFormat`(112) `seekPointCount`(116) `ppCustomBackendVTables`(120) `customBackendCount`(128) `pCustomBackendUserData`(136).
- `ma_decoder_config_init(outputFormat, outputChannels, outputSampleRate)` returns by value. Passing `ma_format_unknown`, 0, 0 keeps the stream's native format.
- `ma_decoder_init_memory(const void* pData, size_t dataSize, const ma_decoder_config*, ma_decoder*)` does not copy `pData`.
- `ma_decoder_init_file(const char* pFilePath, const ma_decoder_config*, ma_decoder*)`.
- `ma_decoder_read_pcm_frames(decoder, out, frameCount, *pFramesRead)` reports frames read.
- `ma_decoder_get_data_format(decoder, *format, *channels, *sampleRate, *channelMap, channelMapCap)`.
- WAV, FLAC and MP3 decoders are enabled by default.

---

## Task 1: Phase 7 bindings, mirror and allocation (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c` (one enum value + case)
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `MAGO_OBJECT_DECODER = 14`.
- Produces Go: `EncodingFormat`, `decoderConfigNative`, `decoderHandle`.
- Produces bindings for every function used in Tasks 2-3.

- [ ] **Step 1: Add the allocation type in C**

```c
    MAGO_OBJECT_DECODER = 14
```

```c
        case MAGO_OBJECT_DECODER: return calloc(1, sizeof(ma_decoder));
```

- [ ] **Step 2: Extend the layout probe**

```c
    printf("sizeof:ma_decoder_config %zu\n", sizeof(ma_decoder_config));
    printf("offsetof:ma_decoder_config.pChannelMap %zu\n", offsetof(ma_decoder_config, pChannelMap));
    printf("offsetof:ma_decoder_config.channelMixMode %zu\n", offsetof(ma_decoder_config, channelMixMode));
    printf("offsetof:ma_decoder_config.ditherMode %zu\n", offsetof(ma_decoder_config, ditherMode));
    printf("offsetof:ma_decoder_config.resampling %zu\n", offsetof(ma_decoder_config, resampling));
    printf("offsetof:ma_decoder_config.allocationCallbacks %zu\n", offsetof(ma_decoder_config, allocationCallbacks));
    printf("offsetof:ma_decoder_config.encodingFormat %zu\n", offsetof(ma_decoder_config, encodingFormat));
    printf("offsetof:ma_decoder_config.seekPointCount %zu\n", offsetof(ma_decoder_config, seekPointCount));
    printf("offsetof:ma_decoder_config.ppCustomBackendVTables %zu\n", offsetof(ma_decoder_config, ppCustomBackendVTables));
    printf("offsetof:ma_decoder_config.customBackendCount %zu\n", offsetof(ma_decoder_config, customBackendCount));
    printf("offsetof:ma_decoder_config.pCustomBackendUserData %zu\n", offsetof(ma_decoder_config, pCustomBackendUserData));
```

- [ ] **Step 3: Add the Go mirror to `types.go`**

```go
// EncodingFormat mirrors ma_encoding_format.
type EncodingFormat int32

const (
	EncodingFormatUnknown EncodingFormat = 0
	EncodingFormatWAV     EncodingFormat = 1
	EncodingFormatFLAC    EncodingFormat = 2
	EncodingFormatMP3     EncodingFormat = 3
	EncodingFormatVorbis  EncodingFormat = 4
)

// decoderConfigNative mirrors ma_decoder_config. Validated by layout_test.go.
type decoderConfigNative struct {
	Format                Format
	Channels              uint32
	SampleRate            uint32
	ChannelMap            *uint8 // *ma_channel
	ChannelMixMode        ChannelMixMode
	DitherMode            DitherMode
	Resampling            resamplerConfigNative
	AllocationCallbacks   allocationCallbacksNative
	EncodingFormat        EncodingFormat
	SeekPointCount        uint32
	CustomBackendVTables  unsafe.Pointer // **ma_decoding_backend_vtable, always nil
	CustomBackendCount    uint32
	CustomBackendUserData unsafe.Pointer
}

type decoderHandle struct{}
```

Extend the object-type constants with `magoObjectDecoder = 14`.

- [ ] **Step 4: Add the bindings**

```go
{FieldName: "maDecoderInitMemory", Symbol: "ma_decoder_init_memory", Type: "func(unsafe.Pointer, uintptr, *decoderConfigNative, *decoderHandle) Result"},
{FieldName: "maDecoderInitFile", Symbol: "ma_decoder_init_file", Type: "func(string, *decoderConfigNative, *decoderHandle) Result"},
{FieldName: "maDecoderUninit", Symbol: "ma_decoder_uninit", Type: "func(*decoderHandle)"},
{FieldName: "maDecoderReadPCMFrames", Symbol: "ma_decoder_read_pcm_frames", Type: "func(*decoderHandle, unsafe.Pointer, uint64, *uint64) Result"},
{FieldName: "maDecoderSeekToPCMFrame", Symbol: "ma_decoder_seek_to_pcm_frame", Type: "func(*decoderHandle, uint64) Result"},
{FieldName: "maDecoderGetDataFormat", Symbol: "ma_decoder_get_data_format", Type: "func(*decoderHandle, *Format, *uint32, *uint32, *uint8, uintptr) Result"},
{FieldName: "maDecoderGetCursorInPCMFrames", Symbol: "ma_decoder_get_cursor_in_pcm_frames", Type: "func(*decoderHandle, *uint64) Result"},
{FieldName: "maDecoderGetLengthInPCMFrames", Symbol: "ma_decoder_get_length_in_pcm_frames", Type: "func(*decoderHandle, *uint64) Result"},
{FieldName: "maDecoderGetAvailableFrames", Symbol: "ma_decoder_get_available_frames", Type: "func(*decoderHandle, *uint64) Result"},
```

- [ ] **Step 5: Regenerate, rebuild, add assertions, verify**

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
go test . -run TestMirroredStructLayouts -v
```

Add `unsafe.Sizeof`/`Offsetof` assertions for `decoderConfigNative` to
`layout_test.go` (size plus every offset from Step 2).

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go \
  internal/abi/layout_probe.c layout_test.go zz_generated.bindings.go embed_*.go
git commit -m "refactor: add Phase 7 bindings, decoder mirror and allocation"
```

---

## Task 2: Decoder

**Files:**
- Create: `decoder.go`
- Create: `decoder_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectDecoder)`, the `maDecoder*` bindings.
- Produces: `type DecoderConfig struct { Format Format; Channels uint32; SampleRate uint32; ChannelMixMode ChannelMixMode; DitherMode DitherMode; SeekPointCount uint32 }`.
- Produces: `func DefaultDecoderConfig() DecoderConfig` (f32 output, rectangular mixing, no dither).
- Produces: `func (lib *Library) NewDecoderFile(path string, config DecoderConfig) (*Decoder, error)`.
- Produces: `func (lib *Library) NewDecoderMemory(data []byte, config DecoderConfig) (*Decoder, error)` (retains `data`).
- Produces: `Decoder` with `ReadF32(out []float32) (uint64, error)`, `ReadS16(out []int16) (uint64, error)`, `SeekToPCMFrame`, `CursorInPCMFrames`, `LengthInPCMFrames`, `AvailableFrames`, `DataFormat() (Format, uint32, uint32, error)`, `Close`.

- [ ] **Step 1: Write the failing test**

```go
// decoder_test.go
package mago

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestDecoderReadsWAVFromMemory(t *testing.T) {
	lib := newNullLibrary(t)

	samples := make([]float32, 64)
	for i := range samples {
		samples[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/48_000))
	}
	data := testWAV(t, samples, 1, 48_000)

	decoder, err := lib.NewDecoderMemory(data, DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderMemory: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	length, err := decoder.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != uint64(len(samples)) {
		t.Fatalf("length = %d, want %d", length, len(samples))
	}

	out := make([]float32, len(samples))
	read, err := decoder.ReadF32(out)
	if err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if read != uint64(len(samples)) {
		t.Fatalf("read %d frames, want %d", read, len(samples))
	}
	if math.Abs(float64(out[1]-samples[1])) > 0.001 {
		t.Fatalf("decoded sample %v, want %v", out[1], samples[1])
	}
}

func testWAV(t *testing.T, samples []float32, channels int, sampleRate int) []byte {
	t.Helper()

	var pcm bytes.Buffer
	for _, sample := range samples {
		value := int16(math.Max(-32768, math.Min(32767, float64(sample)*32767)))
		for ch := 0; ch < channels; ch++ {
			if err := binary.Write(&pcm, binary.LittleEndian, value); err != nil {
				t.Fatalf("write pcm: %v", err)
			}
		}
	}

	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+pcm.Len()))
	wav.WriteString("WAVE")
	wav.WriteString("fmt ")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(16))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(1))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*channels*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(channels*2))
	_ = binary.Write(&wav, binary.LittleEndian, uint16(16))
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(pcm.Len()))
	wav.Write(pcm.Bytes())
	return wav.Bytes()
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestDecoderReadsWAVFromMemory -v`
Expected: FAIL - `undefined: DefaultDecoderConfig`.

- [ ] **Step 3: Implement**

`NewDecoderMemory` keeps `data` on the wrapper so the Go slice cannot be
collected while the decoder references it. Both constructors allocate the handle
through `magoAlloc(magoObjectDecoder)` and free it if init fails.

`ReadF32`/`ReadS16` compute the frame count from the slice length and the
decoder's configured channels (stored at construction), pass a pointer to the
first element, and return `pFramesRead`. Empty slices return `(0, nil)`.

`DataFormat` calls `maDecoderGetDataFormat` with a nil channel map.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestDecoderReadsWAVFromMemory -v`
Expected: PASS.

- [ ] **Step 5: Add a seek/cursor test**

Read a few frames, `SeekToPCMFrame(0)`, and assert the cursor is 0 and a re-read
returns the same first sample.

Run: `go test . -run TestDecoder -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add decoder.go decoder_test.go
git commit -m "feat: expose the miniaudio decoder"
```

---

## Task 3: Audio integration and the custom-backend decision

**Files:**
- Modify: `audio/engine.go`
- Modify: `audio/wav.go` (only if a shared helper is needed)
- Modify: `audio/integration_test.go`

**Interfaces:**
- Consumes: `mago.NewDecoderMemory`, `mago.DefaultDecoderConfig`.
- Produces: `Engine.Load`/`LoadReadSeeker` fall back to `ma_decoder` when the
  pure-Go WAV path fails, so FLAC and MP3 clips load.

- [ ] **Step 1: Write the failing test**

Add to `audio/integration_test.go`:

```go
func TestEngineLoadsWAVViaDecoderFallback(t *testing.T) {
	engine := newTestEngine(t)
	defer func() { _ = engine.Close() }()

	// A WAV with an unusual chunk layout still round-trips through the decoder
	// fallback because the pure-Go path rejects anything it cannot parse.
	clip, err := engine.Load(bytes.NewReader(mustTestWAV(t, make([]float32, 128), 1, 48_000)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if clip.Duration() == 0 {
		t.Fatal("expected a non-zero clip duration")
	}
}
```

Adapt the helper names to the existing test helpers in that file; note that this
test also passes on the current pure-Go path, so pair it with a unit test that
forces the fallback by feeding a WAV whose `data` chunk the pure-Go decoder
rejects (for example, a non-PCM format code).

- [ ] **Step 2: Implement the fallback**

In `audio/engine.go`, change `Load` to read all bytes, try `decodeWAV`, and on
failure call a new `decodeWithDecoder(engine, data)` that:

1. builds `mago.NewDecoderMemory(data, mago.DefaultDecoderConfig())`,
2. reads frames in a loop into a growing `[]float32`,
3. obtains channels and sample rate from `decoder.DataFormat()`,
4. builds a `*Clip` the same way `decodeWAV` does.

Document that the pure-Go WAV path stays first so the common case needs no
decoder allocation.

- [ ] **Step 3: Run the audio tests**

Run: `go test ./audio/ -v`
Expected: PASS.

- [ ] **Step 4: Record the custom-decoding-backend decision**

Custom decoders (`ma_decoding_backend_vtable`) need a five-entry C vtable; do not
expose them. Record the rationale on `mago-8a3.8.3` and close it:

```bash
bd comment mago-8a3.8.3 "Decision: custom decoding backends are not exposed. They require a ma_decoding_backend_vtable (onInit/onInitFile/onInitFileW/onInitMemory/onUninit) that Go cannot build without more bridge C, and the built-in WAV/FLAC/MP3 decoders cover the practical cases. Revisit only for a concrete format such as Opus."
bd close mago-8a3.8.3 --reason "Decided: custom decoding backends not exposed; built-in decoders cover WAV/FLAC/MP3."
```

- [ ] **Step 5: Full test and commit**

Run: `mise run test && mise run lint`
Expected: PASS, 0 lint issues.

```bash
git add audio/engine.go audio/integration_test.go
git commit -m "feat(audio): decode FLAC and MP3 through the mago decoder"
```

---

## Task 4: Example and README

**Files:**
- Create: `examples/decode/main.go`
- Modify: `README.md`

**Interfaces:** none exported; uses `mago.NewDecoderMemory`/`NewDecoderFile`.
The example must not import `unsafe`.

- [ ] **Step 1: Write the example**

`examples/decode` should:
1. synthesise a short WAV in memory,
2. decode it with `NewDecoderMemory`, reporting length, data format and peak,
3. write it to a temporary file and decode it again with `NewDecoderFile`,
4. seek back to the start and re-read a few frames.

- [ ] **Step 2: Run it headless**

Run: `env -i PATH=/usr/bin:/bin HOME="$HOME" go run ./examples/decode`
Expected: prints the decoded length, format and peak without error.

- [ ] **Step 3: Document it**

Add a `go run ./examples/decode` block to the README demos section and list it
among the device-free examples.

- [ ] **Step 4: Verify and commit**

Run: `go build -trimpath ./examples/... && go test ./... && mise run lint`
Expected: PASS.

```bash
git add examples/decode README.md
git commit -m "docs: add a decoding example"
```

---

## Phase 7 exit criteria

- `Decoder` decodes WAV from memory and from a file, with read, seek, length,
  cursor and data-format accessors.
- `audio.Engine.Load` decodes non-WAV input through the decoder; the pure-Go WAV
  path is unchanged.
- Custom decoding backends are explicitly declined and recorded.
- `mise run test`, `mise run lint`, `mise run check-cgo` are green.
- `mise run build-lib-all && mise run generate` leaves no diff.
- `bd ready` surfaces the next phases once Phase 7 closes.