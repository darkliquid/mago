# Phase 8: Encoding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose `ma_encoder` so mago can write WAV (and FLAC) output, and give `audio.Clip` an export path.

**Architecture:** One opaque `Encoder` allocated through `mago_alloc`, with a Go mirror of `ma_encoder_config` (48 bytes) and two purego callbacks so the encoder can write into Go memory. Two constructors: one for a file path (miniaudio does the file I/O), one that keeps the encoded bytes in Go so a clip can be exported to any `io.Writer`.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 8)
**Predecessors:** `.../phase-0.md` … `.../phase-7.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** One `mago_object_type` value plus its `mago_alloc` case.
- **Safe-slice APIs.** Public write methods take Go slices; examples must not import `unsafe`.
- **Two native rebuilds for the phase.** Task 1 adds the allocation type; Task 2 adds the
  encoder callback bridge (see below), which is a second C change.
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Mirrors are validated** by `internal/abi` + `layout_test.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

| Type | Size |
| --- | ---: |
| `ma_encoder_config` | 48 |
| `ma_encoder` | 120 |

- `ma_encoder_config` layout: `encodingFormat`(0) `format`(4) `channels`(8) `sampleRate`(12) `allocationCallbacks`(16, 32 bytes).
- `ma_encoder_config_init(encodingFormat, format, channels, sampleRate)` returns by value and otherwise zeroes.
- `ma_encoder_init_file(path, config, encoder)` writes through miniaudio's own file I/O.
- `ma_encoder_init(onWrite, onSeek, pUserData, config, encoder)` writes through callbacks:
  - `ma_encoder_write_proc(ma_encoder*, const void* pBufferIn, size_t bytesToWrite, size_t* pBytesWritten)`
  - `ma_encoder_seek_proc(ma_encoder*, ma_int64 offset, ma_seek_origin origin)`
- `ma_seek_origin` is start 0, current 1, end 2.
- The encoder needs `onSeek` because the container header is patched when the encoder is uninitialized, so a plain `io.Writer` is not enough: buffer in Go and expose the finished bytes.
- **`ma_encoder_init`'s callbacks receive the `ma_encoder`, not our user data.** Reading
  their sink from Go would mean mirroring `ma_encoder`, so the bridge adds a small
  `mago_encoder_bridge` plus two trampolines and a `mago_encoder_init` wrapper (bridge
  rule category 3). `ma_encoder` does expose `pUserData`, which the trampolines read.
- WAV and FLAC encoders are both available (`MA_HAS_FLAC`); `EncodingFormat` already exists from Phase 7.
- `ma_encoder_write_pcm_frames(encoder, in, frameCount, *pFramesWritten)` reports frames written.

---

## Task 1: Phase 8 bindings, mirror and allocation (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c` (one enum value + case)
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `MAGO_OBJECT_ENCODER = 31`.
- Produces Go: `encoderConfigNative`, `encoderHandle`.
- Produces bindings used by Task 2.

- [ ] **Step 1: Add the allocation type in C**

```c
    MAGO_OBJECT_ENCODER           = 31
```

```c
        case MAGO_OBJECT_ENCODER:           return calloc(1, sizeof(ma_encoder));
```

- [ ] **Step 2: Extend the layout probe**

```c
    printf("sizeof:ma_encoder_config %zu\n", sizeof(ma_encoder_config));
    printf("offsetof:ma_encoder_config.encodingFormat %zu\n", offsetof(ma_encoder_config, encodingFormat));
    printf("offsetof:ma_encoder_config.format %zu\n", offsetof(ma_encoder_config, format));
    printf("offsetof:ma_encoder_config.channels %zu\n", offsetof(ma_encoder_config, channels));
    printf("offsetof:ma_encoder_config.sampleRate %zu\n", offsetof(ma_encoder_config, sampleRate));
    printf("offsetof:ma_encoder_config.allocationCallbacks %zu\n", offsetof(ma_encoder_config, allocationCallbacks));
```

- [ ] **Step 3: Add the Go mirror to `types.go`**

```go
// encoderConfigNative mirrors ma_encoder_config. Validated by layout_test.go.
type encoderConfigNative struct {
	EncodingFormat      EncodingFormat
	Format              Format
	Channels            uint32
	SampleRate          uint32
	AllocationCallbacks allocationCallbacksNative
}

type encoderHandle struct{}
```

Extend the object-type constants with `magoObjectEncoder = 31`.

- [ ] **Step 4: Add the bindings**

```go
{FieldName: "maEncoderInitFile", Symbol: "ma_encoder_init_file", Type: "func(string, *encoderConfigNative, *encoderHandle) Result"},
{FieldName: "maEncoderInit", Symbol: "ma_encoder_init", Type: "func(uintptr, uintptr, uintptr, *encoderConfigNative, *encoderHandle) Result"},
{FieldName: "maEncoderUninit", Symbol: "ma_encoder_uninit", Type: "func(*encoderHandle)"},
{FieldName: "maEncoderWritePCMFrames", Symbol: "ma_encoder_write_pcm_frames", Type: "func(*encoderHandle, unsafe.Pointer, uint64, *uint64) Result"},
```

- [ ] **Step 5: Regenerate, rebuild, add assertions, verify**

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
go test . -run TestMirroredStructLayouts -v
```

Add `unsafe.Sizeof`/`Offsetof` assertions for `encoderConfigNative` (size plus
all five offsets) to `layout_test.go`.

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go \
  internal/abi/layout_probe.c layout_test.go zz_generated.bindings.go embed_*.go
git commit -m "refactor: add Phase 8 bindings, encoder mirror and allocation"
```

---

## Task 2: Encoder

**Files:**
- Create: `encoder.go`
- Create: `encoder_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectEncoder)`, the `maEncoder*` bindings, `purego.NewCallback`.
- Produces: `type EncoderConfig struct { EncodingFormat EncodingFormat; Format Format; Channels uint32; SampleRate uint32 }`.
- Produces: `func DefaultEncoderConfig(channels, sampleRate uint32) EncoderConfig` (WAV, f32).
- Produces: `func (lib *Library) NewEncoderFile(path string, config EncoderConfig) (*Encoder, error)`.
- Produces: `func (lib *Library) NewEncoderWriter(config EncoderConfig) (*Encoder, error)`.
- Produces: `Encoder` with `WriteF32(frames []float32) (uint64, error)`, `WriteS16(frames []int16) (uint64, error)`, `Finish() error`, `Bytes() []byte`, `Close() error`.

- [ ] **Step 1: Write the failing test**

```go
// encoder_test.go
package mago

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncoderWriterProducesWAV(t *testing.T) {
	lib := newNullLibrary(t)

	encoder, err := lib.NewEncoderWriter(DefaultEncoderConfig(1, 48_000))
	if err != nil {
		t.Fatalf("NewEncoderWriter: %v", err)
	}

	frames := make([]float32, 128)
	for i := range frames {
		frames[i] = 0.25
	}
	if written, err := encoder.WriteF32(frames); err != nil {
		t.Fatalf("WriteF32: %v", err)
	} else if written != uint64(len(frames)) {
		t.Fatalf("wrote %d frames, want %d", written, len(frames))
	}
	if err := encoder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	defer func() { _ = encoder.Close() }()

	data := encoder.Bytes()
	if len(data) < 44 {
		t.Fatalf("encoded only %d bytes, want a WAV header plus data", len(data))
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatalf("output is not a WAV stream: % x", data[:12])
	}
	if riffSize := binary.LittleEndian.Uint32(data[4:8]); int(riffSize) != len(data)-8 {
		t.Fatalf("RIFF size = %d, want %d", riffSize, len(data)-8)
	}
}

func TestEncoderFileWritesWAV(t *testing.T) {
	lib := newNullLibrary(t)

	path := t.TempDir() + "/out.wav"
	encoder, err := lib.NewEncoderFile(path, DefaultEncoderConfig(1, 48_000))
	if err != nil {
		t.Fatalf("NewEncoderFile: %v", err)
	}

	frames := make([]float32, 64)
	if _, err := encoder.WriteF32(frames); err != nil {
		t.Fatalf("WriteF32: %v", err)
	}
	if err := encoder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encoded file: %v", err)
	}
	if string(data[0:4]) != "RIFF" {
		t.Fatalf("file is not a WAV stream: % x", data[:4])
	}

	// The output must round-trip through the decoder from Phase 7.
	decoder, err := lib.NewDecoderFile(path, DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderFile: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	length, err := decoder.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != uint64(len(frames)) {
		t.Fatalf("decoded %d frames, want %d", length, len(frames))
	}
}

func TestEncoderWriterRoundTripsThroughDecoder(t *testing.T) {
	lib := newNullLibrary(t)

	encoder, err := lib.NewEncoderWriter(DefaultEncoderConfig(1, 48_000))
	if err != nil {
		t.Fatalf("NewEncoderWriter: %v", err)
	}

	frames := make([]float32, 32)
	for i := range frames {
		frames[i] = float32(i) / 32
	}
	if _, err := encoder.WriteF32(frames); err != nil {
		t.Fatalf("WriteF32: %v", err)
	}
	if err := encoder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	defer func() { _ = encoder.Close() }()

	decoder, err := lib.NewDecoderMemory(encoder.Bytes(), DefaultDecoderConfig())
	if err != nil {
		t.Fatalf("NewDecoderMemory: %v", err)
	}
	defer func() { _ = decoder.Close() }()

	out := make([]float32, len(frames))
	read, err := decoder.ReadF32(out)
	if err != nil {
		t.Fatalf("ReadF32: %v", err)
	}
	if read != uint64(len(frames)) {
		t.Fatalf("decoded %d frames, want %d", read, len(frames))
	}
	if diff := out[1] - frames[1]; diff > 0.001 || diff < -0.001 {
		t.Fatalf("round-tripped sample %v, want %v", out[1], frames[1])
	}

	var _ = bytes.MinRead
}
```

Add the missing `os` import and drop the trailing `bytes` placeholder when
writing the real test.

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestEncoder -v`
Expected: FAIL - `undefined: DefaultEncoderConfig`.

- [ ] **Step 3: Implement the file encoder**

```go
// NewEncoderFile writes encoded audio to path using miniaudio's file I/O.
func (lib *Library) NewEncoderFile(path string, config EncoderConfig) (*Encoder, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	native := config.native()
	handle := (*encoderHandle)(lib.bindings.magoAlloc(magoObjectEncoder))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate encoder: out of memory")
	}
	if result := lib.bindings.maEncoderInitFile(path, &native, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_encoder_init_file", result)
	}
	return &Encoder{lib: lib, handle: handle}, nil
}
```

- [ ] **Step 4: Implement the writer encoder and callbacks**

The writer variant registers a `*encoderSink` in a `sync.Map` keyed by a token,
and passes two `purego.NewCallback` trampolines to `ma_encoder_init`:

```go
type encoderSink struct {
	data []byte
	pos  int64
}

func (s *encoderSink) write(p []byte) int {
	if s.pos < 0 {
		return 0
	}
	if end := s.pos + int64(len(p)); end > int64(len(s.data)) {
		s.data = append(s.data, make([]byte, end-int64(len(s.data)))...)
	}
	copy(s.data[s.pos:], p)
	s.pos += int64(len(p))
	return len(p)
}

func (s *encoderSink) seek(offset int64, origin uint32) {
	switch origin {
	case 0: // start
		s.pos = offset
	case 1: // current
		s.pos += offset
	case 2: // end
		s.pos = int64(len(s.data)) + offset
	}
	if s.pos < 0 {
		s.pos = 0
	}
}
```

```go
var (
	encoderSeq    atomic.Uint64
	encoderSinks  sync.Map // token -> *encoderSink
	encoderWritePtr = purego.NewCallback(func(token uintptr, _ uintptr, buffer uintptr, bytesToWrite uintptr, bytesWritten uintptr) uintptr {
		value, ok := encoderSinks.Load(token)
		if !ok || buffer == 0 {
			return uintptr(Error)
		}
		sink := value.(*encoderSink)
		written := sink.write(unsafe.Slice((*byte)(unsafe.Pointer(buffer)), int(bytesToWrite)))
		if bytesWritten != 0 {
			*(*uintptr)(unsafe.Pointer(bytesWritten)) = uintptr(written)
		}
		return uintptr(Success)
	})
	encoderSeekPtr = purego.NewCallback(func(token uintptr, _ uintptr, offset int64, origin uint32) uintptr {
		value, ok := encoderSinks.Load(token)
		if !ok {
			return uintptr(Error)
		}
		value.(*encoderSink).seek(offset, origin)
		return uintptr(Success)
	})
)
```

`NewEncoderWriter` allocates and frees the handle like the file variant but
passes the callbacks and token as `pUserData`.

- [ ] **Step 5: Implement `Finish`, `Bytes`, `Close`, and the write methods**

- `WriteF32`/`WriteS16` validate the configured format, compute frames from the
  slice length and channels, call `maEncoderWritePCMFrames`, and return the
  written frame count.
- `Finish` calls `maEncoderUninit` (which patches the container header), frees
  the handle, removes the sink token, and marks the encoder finished. Calling it
  twice returns nil.
- `Bytes` returns a copy of the sink's data (nil for the file variant).
- `Close` finalizes if needed and frees anything still outstanding; it must be
  safe to call after `Finish`.

- [ ] **Step 6: Run the tests**

Run: `go test . -run TestEncoder -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add encoder.go encoder_test.go
git commit -m "feat: expose the miniaudio encoder"
```

---

## Task 3: Clip export

**Files:**
- Modify: `audio/stream.go` (or `audio/types.go`; whichever already owns `Clip`)
- Modify: `audio/integration_test.go`

**Interfaces:**
- Consumes: `mago.NewEncoderWriter`, `mago.DefaultEncoderConfig`.
- Produces: `func (c *Clip) WriteWAV(w io.Writer) error` and `func (c *Clip) Encode() ([]byte, error)`.

- [ ] **Step 1: Write the failing test**

```go
func TestClipEncodesToWAV(t *testing.T) {
	engine := newNullEngine(t)
	defer func() { _ = engine.Close() }()

	samples := make([]float32, 256)
	for i := range samples {
		samples[i] = 0.25
	}
	clip, err := engine.Load(bytes.NewReader(mustTestWAV(t, samples, 1, 48000)))
	if err != nil {
		t.Fatalf("load clip: %v", err)
	}

	encoded, err := clip.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded) < 44 || string(encoded[0:4]) != "RIFF" {
		t.Fatalf("encoded output is not a WAV stream: % x", encoded[:4])
	}

	var buf bytes.Buffer
	if err := clip.WriteWAV(&buf); err != nil {
		t.Fatalf("WriteWAV: %v", err)
	}
	if buf.Len() != len(encoded) {
		t.Fatalf("WriteWAV wrote %d bytes, Encode produced %d", buf.Len(), len(encoded))
	}
}
```

Use the engine helper that already exists in the audio tests.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./audio/ -run TestClipEncodesToWAV -v`
Expected: FAIL - `clip.Encode undefined`.

- [ ] **Step 3: Implement**

`Clip` needs access to a `*mago.Library` to build the encoder; it does not hold
one today. Pass the engine's library through: add an unexported `lib
*mago.Library` field to `Clip` and set it wherever clips are constructed
(`decodeWAV` callers take the engine, so thread the library through, or add a
helper `newClip(lib, samples, channels, sampleRate)` used by both decode paths).
When `lib` is nil, `Encode` fails with a clear error.

`Encode` builds `mago.NewEncoderWriter(mago.DefaultEncoderConfig(uint32(c.channels), uint32(c.sampleRate)))`,
writes the clip's samples with `WriteF32`, calls `Finish`, and returns `Bytes`.
`WriteWAV` calls `Encode` and writes the result to `w`.

- [ ] **Step 4: Run the tests**

Run: `go test ./audio/ -v`
Expected: PASS.

- [ ] **Step 5: Full test and commit**

Run: `mise run test && mise run lint && mise run check-cgo`
Expected: PASS, 0 lint issues.

```bash
git add audio/stream.go audio/types.go audio/engine.go audio/wav.go audio/integration_test.go
git commit -m "feat(audio): export clips to WAV"
```

---

## Task 4: Example and README

**Files:**
- Create: `examples/encode/main.go`
- Modify: `README.md`

**Interfaces:** uses `mago.NewEncoderWriter`/`NewEncoderFile`; must not import
`unsafe`.

- [ ] **Step 1: Write the example**

`examples/encode` should:
1. synthesise a short tone as `[]float32`,
2. encode it with `NewEncoderWriter`, reporting the byte count and RIFF size,
3. write it to a temporary file with `NewEncoderFile`,
4. decode both results back with the Phase 7 decoder and report frame counts.

- [ ] **Step 2: Run it headless**

Run: `env -i PATH=/usr/bin:/bin HOME="$HOME" go run ./examples/encode`
Expected: prints the encoded sizes and decoded frame counts without error.

- [ ] **Step 3: Document it**

Add a `go run ./examples/encode` block to the README demos section and list it
among the device-free examples.

- [ ] **Step 4: Verify and commit**

Run: `go build -trimpath ./examples/... && go test ./... && mise run lint`
Expected: PASS.

```bash
git add examples/encode README.md
git commit -m "docs: add an encoding example"
```

---

## Phase 8 exit criteria

- `Encoder` writes WAV to a file and to an in-memory buffer, and the output
  round-trips through the Phase 7 decoder.
- `audio.Clip` can encode itself to WAV bytes or write them to an `io.Writer`.
- `mise run test`, `mise run lint`, `mise run check-cgo` are green.
- `mise run build-lib-all && mise run generate` leaves no diff.
- `bd ready` surfaces the next phases once Phase 8 closes.