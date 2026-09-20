# Phase 9: Data Sources Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Go supply PCM to miniaudio. Add a `DataSource` abstraction, adapt existing mago objects to it, and register a custom Go data source with miniaudio through its vtable so later phases (node graph, engine) can consume it.

**Architecture:** `ma_data_source_vtable` callbacks receive the `ma_data_source`, not our user data, so the C bridge gains a small `mago_data_source` object whose base is first, a static vtable, and trampolines that forward to Go callbacks. The Go side exposes a `DataSource` interface, adapters over the existing objects, and a `CustomDataSource` that miniaudio drives through the bridge.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 9)
**Predecessors:** `.../phase-0.md` … `.../phase-8.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** The bridge additions are category 3 (callback trampolines for signatures that
  carry no user data) plus allocation of the bridge object. No config mirrors are needed.
- **Safe-slice APIs.** Public read methods take Go slices; examples must not import `unsafe`.
- **One native rebuild for the phase** (Task 1).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

- `ma_data_source_vtable` = `onRead`, `onSeek`, `onGetDataFormat`, `onGetCursor`,
  `onGetLength`, `onSetLooping`, `flags`.
- A custom data source must have `ma_data_source_base` as its **first** member and be
  initialized with `ma_data_source_init(&config{H.vtable}, &obj->base)`.
- The vtable callbacks receive `ma_data_source* pDataSource` and recover their own
  state by casting; there is no user-data parameter, so the bridge must store it.
- `ma_data_source_read_pcm_frames(pDataSource, pFramesOut, frameCount, *pFramesRead)`
  must accept `pFramesOut == NULL` to mean "seek forward".
- `ma_data_source_seek_to_pcm_frame`, `ma_data_source_get_data_format`,
  `ma_data_source_get_cursor_in_pcm_frames`,
  `ma_data_source_get_length_in_pcm_frames`, `ma_data_source_set_looping` all operate
  on any data source.
- `ma_data_source_vtable.flags` may be left 0; optional callbacks may be `NULL` and
  miniaudio treats them as unsupported.
- `ma_data_source_node` needs a node graph to do anything, so it moves to Phase 10
  (`mago-8a3.10.3` is re-parented there).

---

## Task 1: Data source bridge (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c`
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `mago_data_source_init(onRead, onSeek, onGetDataFormat, onGetCursor, onGetLength, onSetLooping, userData, void** ppDataSource) Result` and `mago_data_source_uninit(ma_data_source*)`.
- Produces Go: `dataSourceHandle`, and bindings `maDataSourceReadPCMFrames`, `maDataSourceSeekToPCMFrame`, `maDataSourceGetDataFormat`, `maDataSourceGetCursorInPCMFrames`, `maDataSourceGetLengthInPCMFrames`, `maDataSourceSetLooping`, `magoDataSourceInit`, `magoDataSourceUninit`.

- [ ] **Step 1: Add the bridge object and trampolines in C**

```c
typedef ma_result (*mago_ds_read_cb)(uintptr_t userData, void* pFramesOut, ma_uint64 frameCount, ma_uint64* pFramesRead);
typedef ma_result (*mago_ds_seek_cb)(uintptr_t userData, ma_uint64 frameIndex);
typedef ma_result (*mago_ds_format_cb)(uintptr_t userData, ma_format* pFormat, ma_uint32* pChannels, ma_uint32* pSampleRate, ma_channel* pChannelMap, size_t channelMapCap);
typedef ma_result (*mago_ds_cursor_cb)(uintptr_t userData, ma_uint64* pCursor);
typedef ma_result (*mago_ds_length_cb)(uintptr_t userData, ma_uint64* pLength);
typedef ma_result (*mago_ds_looping_cb)(uintptr_t userData, ma_bool32 isLooping);

typedef struct
{
    ma_data_source_base base; /* Must be first. */
    uintptr_t onRead;
    uintptr_t onSeek;
    uintptr_t onGetDataFormat;
    uintptr_t onGetCursor;
    uintptr_t onGetLength;
    uintptr_t onSetLooping;
    uintptr_t userData;
} mago_data_source;
```

Each trampoline casts `pDataSource` back to `mago_data_source*` and forwards to the
corresponding Go callback with `pDataSource->userData`. A `NULL` callback returns
`MA_NOT_IMPLEMENTED` where miniaudio allows it and `MA_INVALID_ARGS` for `onRead`/`onSeek`.

```c
static const ma_data_source_vtable g_mago_data_source_vtable =
{
    mago_data_source_on_read,
    mago_data_source_on_seek,
    mago_data_source_on_get_data_format,
    mago_data_source_on_get_cursor,
    mago_data_source_on_get_length,
    mago_data_source_on_set_looping,
    0
};

MAGO_API ma_result mago_data_source_init(
    uintptr_t onRead, uintptr_t onSeek, uintptr_t onGetDataFormat,
    uintptr_t onGetCursor, uintptr_t onGetLength, uintptr_t onSetLooping,
    uintptr_t userData, ma_data_source** ppDataSource)
```

`mago_data_source_init` allocates the struct, fills the fields, calls
`ma_data_source_init` with a config pointing at the static vtable, and returns the
object as `ma_data_source*` (the base is first, so the pointer is the same).
`mago_data_source_uninit` calls `ma_data_source_uninit` and frees.

- [ ] **Step 2: Add the Go handle and bindings**

```go
type dataSourceHandle struct{}
```

```go
{FieldName: "magoDataSourceInit", Symbol: "mago_data_source_init", Type: "func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, **dataSourceHandle) Result"},
{FieldName: "magoDataSourceUninit", Symbol: "mago_data_source_uninit", Type: "func(*dataSourceHandle)"},
{FieldName: "maDataSourceReadPCMFrames", Symbol: "ma_data_source_read_pcm_frames", Type: "func(*dataSourceHandle, unsafe.Pointer, uint64, *uint64) Result"},
{FieldName: "maDataSourceSeekToPCMFrame", Symbol: "ma_data_source_seek_to_pcm_frame", Type: "func(*dataSourceHandle, uint64) Result"},
{FieldName: "maDataSourceGetDataFormat", Symbol: "ma_data_source_get_data_format", Type: "func(*dataSourceHandle, *Format, *uint32, *uint32, *uint8, uintptr) Result"},
{FieldName: "maDataSourceGetCursorInPCMFrames", Symbol: "ma_data_source_get_cursor_in_pcm_frames", Type: "func(*dataSourceHandle, *uint64) Result"},
{FieldName: "maDataSourceGetLengthInPCMFrames", Symbol: "ma_data_source_get_length_in_pcm_frames", Type: "func(*dataSourceHandle, *uint64) Result"},
{FieldName: "maDataSourceSetLooping", Symbol: "ma_data_source_set_looping", Type: "func(*dataSourceHandle, uint32) Result"},
```

- [ ] **Step 3: Regenerate, rebuild, verify**

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
go test ./...
```

Expected: PASS, and `git status` shows only the bridge, generator, `types.go` and the
generated files changed.

- [ ] **Step 4: Commit**

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go \
  zz_generated.bindings.go embed_*.go
git commit -m "refactor: add the data source callback bridge"
```

---

## Task 2: DataSource interface and custom data source

**Files:**
- Create: `datasource.go`
- Create: `datasource_test.go`

**Interfaces:**
- Consumes: `magoDataSourceInit`, `magoDataSourceUninit`, and the `maDataSource*` bindings.
- Produces: `type DataSource interface { ReadPCMFrames(out []byte) (uint64, error); SeekToPCMFrame(uint64) error; DataFormat() (Format, uint32, uint32, error); CursorInPCMFrames() (uint64, error); LengthInPCMFrames() (uint64, error); SetLooping(bool) error }`.
- Produces: `func (lib *Library) NewCustomDataSource(source DataSource) (*CustomDataSource, error)`.
- Produces: `CustomDataSource` with `ReadPCMFrames(out []byte) (uint64, error)`, `SeekToPCMFrame`, `DataFormat`, `CursorInPCMFrames`, `LengthInPCMFrames`, `SetLooping`, `Close`.

- [ ] **Step 1: Write the failing test**

```go
// datasource_test.go
package mago

import "testing"

// rampSource is a tiny Go data source: a fixed number of mono f32 frames.
type rampSource struct {
	frames []float32
	pos    uint64
}

func (s *rampSource) DataFormat() (Format, uint32, uint32, error) { return FormatF32, 1, 48_000, nil }
func (s *rampSource) LengthInPCMFrames() (uint64, error)           { return uint64(len(s.frames)), nil }
func (s *rampSource) CursorInPCMFrames() (uint64, error)           { return s.pos, nil }
func (s *rampSource) SetLooping(bool) error                        { return nil }

func (s *rampSource) SeekToPCMFrame(frameIndex uint64) error {
	if frameIndex > uint64(len(s.frames)) {
		return fmt.Errorf("seek out of range")
	}
	s.pos = frameIndex
	return nil
}

func (s *rampSource) ReadPCMFrames(out []byte) (uint64, error) {
	available := (uint64(len(s.frames)) - s.pos) * 4
	if uint64(len(out)) > available {
		out = out[:available]
	}
	frames := uint64(len(out)) / 4
	for i := uint64(0); i < frames; i++ {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(s.frames[s.pos+i]))
	}
	s.pos += frames
	return frames, nil
}

func TestCustomDataSourceRoundTrip(t *testing.T) {
	lib := newNullLibrary(t)

	ramp := &rampSource{frames: []float32{0, 1, 2, 3, 4, 5, 6, 7}}
	source, err := lib.NewCustomDataSource(ramp)
	if err != nil {
		t.Fatalf("NewCustomDataSource: %v", err)
	}
	defer func() { _ = source.Close() }()

	format, channels, sampleRate, err := source.DataFormat()
	if err != nil {
		t.Fatalf("DataFormat: %v", err)
	}
	if format != FormatF32 || channels != 1 || sampleRate != 48_000 {
		t.Fatalf("format = %v channels = %d rate = %d", format, channels, sampleRate)
	}

	length, err := source.LengthInPCMFrames()
	if err != nil {
		t.Fatalf("LengthInPCMFrames: %v", err)
	}
	if length != uint64(len(ramp.frames)) {
		t.Fatalf("length = %d, want %d", length, len(ramp.frames))
	}

	buffer := make([]byte, 4*4)
	read, err := source.ReadPCMFrames(buffer)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != 4 {
		t.Fatalf("read %d frames, want 4", read)
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(buffer[4:8])); got != 1 {
		t.Fatalf("second frame = %v, want 1", got)
	}

	if err := source.SeekToPCMFrame(0); err != nil {
		t.Fatalf("SeekToPCMFrame: %v", err)
	}
	if cursor, err := source.CursorInPCMFrames(); err != nil || cursor != 0 {
		t.Fatalf("cursor = %d (%v), want 0", cursor, err)
	}
}
```

Add the `encoding/binary`, `fmt` and `math` imports when writing the real file.

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestCustomDataSource -v`
Expected: FAIL - `undefined: CustomDataSource`.

- [ ] **Step 3: Implement the Go callbacks and wrapper**

Keep a `sync.Map` from token to the `DataSource` plus the format information, and
register six `purego.NewCallback` trampolines whose argument shapes mirror the C
callback typedefs. Store nothing in the map beyond what the callbacks need.

```go
type dataSourceState struct {
	source   DataSource
	format   Format
	channels uint32
	rate     uint32
}

var (
	dataSourceSeq   atomic.Uint64
	dataSources     sync.Map // token -> *dataSourceState
	dataSourceReadPtr = purego.NewCallback(func(token uintptr, framesOut unsafe.Pointer, frameCount uint64, framesRead unsafe.Pointer) uintptr { ... })
	// five more, one per vtable entry
)
```

`NewCustomDataSource` reads the source's format once, allocates through
`magoDataSourceInit`, and returns a `CustomDataSource` holding the handle and token.
`Close` calls `magoDataSourceUninit` and deletes the token.

The read callback must honour `framesOut == NULL` (seek forward): advance the cursor
by `frameCount` and report the frames skipped without touching Go memory.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestCustomDataSource -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add datasource.go datasource_test.go
git commit -m "feat: let Go provide PCM through a custom data source"
```

---

## Task 3: Adapters for existing sources

**Files:**
- Modify: `datasource.go`
- Modify: `datasource_test.go`

**Interfaces:**
- Produces: `func (d *Decoder) AsDataSource() *DecoderDataSource`, `func (b *AudioBuffer) AsDataSource() *AudioBufferDataSource`, `func (w *Waveform) AsDataSource() *WaveformDataSource`, `func (n *Noise) AsDataSource() *NoiseDataSource`, each satisfying `DataSource`.

- [ ] **Step 1: Write the failing test**

```go
func TestWaveformImplementsDataSource(t *testing.T) {
	lib := newNullLibrary(t)

	waveform, err := lib.NewWaveform(WaveformConfig{Format: FormatF32, Channels: 1, SampleRate: 48_000, Type: WaveformTypeSine, Amplitude: 0.5, Frequency: 440})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = waveform.Close() }()

	var source DataSource = waveform.AsDataSource()

	if _, err := source.DataFormat(); err != nil {
		t.Fatalf("DataFormat: %v", err)
	}
	buffer := make([]byte, 4*8)
	read, err := source.ReadPCMFrames(buffer)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read == 0 {
		t.Fatal("expected the waveform to produce frames")
	}
}
```

Mirror this for `AudioBuffer` and `Decoder` where a source is easy to build.

- [ ] **Step 2: Implement**

Adapters are thin Go wrappers: they render the underlying typed read into the
interface's `[]byte` form using `encoding/binary`, and delegate the rest. For
`Decoder` and `AudioBuffer`, prefer their existing `ReadF32`/`ReadS16`; convert the
bytes to the right Go type with `unsafe`-free helpers.

- [ ] **Step 3: Run the tests**

Run: `go test . -run 'TestWaveformImplementsDataSource|TestCustomDataSource' -v`
Expected: PASS.

- [ ] **Step 4: Full test and commit**

Run: `mise run test && mise run lint && mise run check-cgo`
Expected: PASS, 0 lint issues.

```bash
git add datasource.go datasource_test.go
git commit -m "feat: adapt existing sources to the DataSource interface"
```

---

## Task 4: Example, README and bead housekeeping

**Files:**
- Create: `examples/datasource/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write the example**

`examples/datasource` implements a small Go data source (a sine generator), registers
it with `NewCustomDataSource`, then reads frames through miniaudio and reports the
format, length, cursor and peak. It must not import `unsafe` and must need no audio
device.

- [ ] **Step 2: Run it headless**

Run: `env -i PATH=/usr/bin:/bin HOME="$HOME" go run ./examples/datasource`
Expected: prints the format, length and peak without error.

- [ ] **Step 3: Document it and re-parent the node bead**

Add a `go run ./examples/datasource` block to the README demos section and list it
among the device-free examples.

`ma_data_source_node` needs a node graph, so move it to Phase 10:

```bash
bd update mago-8a3.10.3 --parent mago-8a3.11
bd comment mago-8a3.10.3 "Moved to Phase 10: ma_data_source_node requires a node graph, which Phase 9 does not expose."
```

- [ ] **Step 4: Verify and commit**

Run: `go build -trimpath ./examples/... && go test ./... && mise run lint`
Expected: PASS.

```bash
git add examples/datasource README.md
git commit -m "docs: add a custom data source example"
```

---

## Phase 9 exit criteria

- A Go type satisfying `DataSource` is driven by miniaudio through the vtable bridge.
- `Decoder`, `AudioBuffer`, `Waveform` and `Noise` can be used through the same interface.
- The bridge handles `pFramesOut == NULL` as a forward seek.
- `mise run test`, `mise run lint`, `mise run check-cgo` are green.
- `mise run build-lib-all && mise run generate` leaves no diff.
- `ma_data_source_node` is re-parented to Phase 10 and `bd ready` reflects that.