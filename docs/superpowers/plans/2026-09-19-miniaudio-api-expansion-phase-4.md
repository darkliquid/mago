# Phase 4: Buffers and Ring Buffers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose in-memory audio buffers (`ma_audio_buffer`, `ma_audio_buffer_ref`) and the byte/PCM ring buffers (`ma_rb`, `ma_pcm_rb`).

**Architecture:** Four opaque objects allocated through `mago_alloc`. The one config struct (`ma_audio_buffer_config`, 64 bytes, including an embedded `ma_allocation_callbacks`) is mirrored in Go, and the ring-buffer constructors take plain integers, so the only C change is four allocation enum values. One native rebuild for the phase.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 4)
**Predecessors:** `.../phase-0.md` through `.../phase-3.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** Four `mago_object_type` values plus their `mago_alloc` cases. Nothing else.
- **One native rebuild for the phase** (Task 1).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Mirrors are validated** by `internal/abi` + `layout_test.go`.
- **Go slices passed to miniaudio must stay alive** for the object's lifetime; Go wrappers keep a reference.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

| Type | Size |
| --- | ---: |
| `ma_audio_buffer_config` | 64 (`sizeInFrames` at 16, `pData` at 24) |
| `ma_audio_buffer` | 152 |
| `ma_audio_buffer_ref` | 112 |
| `ma_rb` | 64 |
| `ma_pcm_rb` | 152 |

- `ma_audio_buffer_config` = `format`, `channels`, `sampleRate`, `sizeInFrames`, `pData`, `allocationCallbacks` (`ma_allocation_callbacks` is 4 pointers = 32 bytes).
- `ma_audio_buffer_init` uses `pData` directly; `ma_audio_buffer_init_copy` copies it; with `pData == nil` miniaudio allocates.
- `ma_audio_buffer_uninit` frees only what miniaudio itself allocated.
- `ma_audio_buffer_ref` never owns its data and has no config struct.
- `ma_rb_init` allocates when `pOptionalPreallocatedBuffer` is NULL; `ma_rb_uninit` frees. Sizes are in bytes.
- `ma_pcm_rb_*` mirrors `ma_rb_*` but in frames, and knows format/channels/sample rate.
- `ma_audio_buffer_read_pcm_frames` and `ma_audio_buffer_ref_read_pcm_frames` return the frame count directly (not a Result).
- `ma_rb_pointer_distance` is an `int32`; `ma_pcm_rb_pointer_distance` is an `int32` in frames.

---

## Task 1: Phase 4 bindings, mirrors and allocation (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c` (four enum values + cases)
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `MAGO_OBJECT_AUDIO_BUFFER = 10`, `MAGO_OBJECT_AUDIO_BUFFER_REF = 11`, `MAGO_OBJECT_RB = 12`, `MAGO_OBJECT_PCM_RB = 13`.
- Produces Go: `allocationCallbacksNative`, `audioBufferConfigNative`, and handles `audioBufferHandle`, `audioBufferRefHandle`, `ringBufferHandle`, `pcmRingBufferHandle`.
- Produces bindings for every function listed in Tasks 2-4.

- [ ] **Step 1: Add the allocation types in C**

```c
    MAGO_OBJECT_AUDIO_BUFFER      = 10,
    MAGO_OBJECT_AUDIO_BUFFER_REF  = 11,
    MAGO_OBJECT_RB                = 12,
    MAGO_OBJECT_PCM_RB            = 13
```

```c
        case MAGO_OBJECT_AUDIO_BUFFER:     return calloc(1, sizeof(ma_audio_buffer));
        case MAGO_OBJECT_AUDIO_BUFFER_REF: return calloc(1, sizeof(ma_audio_buffer_ref));
        case MAGO_OBJECT_RB:               return calloc(1, sizeof(ma_rb));
        case MAGO_OBJECT_PCM_RB:           return calloc(1, sizeof(ma_pcm_rb));
```

- [ ] **Step 2: Extend the layout probe**

```c
    printf("sizeof:ma_audio_buffer_config %zu\n", sizeof(ma_audio_buffer_config));
    printf("offsetof:ma_audio_buffer_config.sizeInFrames %zu\n", offsetof(ma_audio_buffer_config, sizeInFrames));
    printf("offsetof:ma_audio_buffer_config.pData %zu\n", offsetof(ma_audio_buffer_config, pData));
    printf("offsetof:ma_audio_buffer_config.allocationCallbacks %zu\n", offsetof(ma_audio_buffer_config, allocationCallbacks));
    printf("sizeof:ma_allocation_callbacks %zu\n", sizeof(ma_allocation_callbacks));
```

- [ ] **Step 3: Add the Go mirrors to `types.go`**

```go
// allocationCallbacksNative mirrors ma_allocation_callbacks. mago always passes
// zeroed callbacks so miniaudio uses its default allocator.
type allocationCallbacksNative struct {
	UserData   unsafe.Pointer
	OnMalloc   unsafe.Pointer
	OnRealloc  unsafe.Pointer
	OnFree     unsafe.Pointer
}

// audioBufferConfigNative mirrors ma_audio_buffer_config. Validated by
// layout_test.go.
type audioBufferConfigNative struct {
	Format              Format
	Channels            uint32
	SampleRate          uint32
	SizeInFrames        uint64
	Data                unsafe.Pointer
	AllocationCallbacks allocationCallbacksNative
}

type audioBufferHandle struct{}
type audioBufferRefHandle struct{}
type ringBufferHandle struct{}
type pcmRingBufferHandle struct{}
```

Extend the object-type constants with `magoObjectAudioBuffer = 10`,
`magoObjectAudioBufferRef = 11`, `magoObjectRingBuffer = 12`,
`magoObjectPCMRingBuffer = 13`.

- [ ] **Step 4: Add the bindings**

```go
{FieldName: "maAudioBufferInit", Symbol: "ma_audio_buffer_init", Type: "func(*audioBufferConfigNative, *audioBufferHandle) Result"},
{FieldName: "maAudioBufferInitCopy", Symbol: "ma_audio_buffer_init_copy", Type: "func(*audioBufferConfigNative, *audioBufferHandle) Result"},
{FieldName: "maAudioBufferUninit", Symbol: "ma_audio_buffer_uninit", Type: "func(*audioBufferHandle)"},
{FieldName: "maAudioBufferReadPCMFrames", Symbol: "ma_audio_buffer_read_pcm_frames", Type: "func(*audioBufferHandle, unsafe.Pointer, uint64, uint32) uint64"},
{FieldName: "maAudioBufferSeekToPCMFrame", Symbol: "ma_audio_buffer_seek_to_pcm_frame", Type: "func(*audioBufferHandle, uint64) Result"},
{FieldName: "maAudioBufferMap", Symbol: "ma_audio_buffer_map", Type: "func(*audioBufferHandle, *unsafe.Pointer, *uint64) Result"},
{FieldName: "maAudioBufferUnmap", Symbol: "ma_audio_buffer_unmap", Type: "func(*audioBufferHandle, uint64) Result"},
{FieldName: "maAudioBufferGetCursorInPCMFrames", Symbol: "ma_audio_buffer_get_cursor_in_pcm_frames", Type: "func(*audioBufferHandle, *uint64) Result"},
{FieldName: "maAudioBufferGetLengthInPCMFrames", Symbol: "ma_audio_buffer_get_length_in_pcm_frames", Type: "func(*audioBufferHandle, *uint64) Result"},
{FieldName: "maAudioBufferGetAvailableFrames", Symbol: "ma_audio_buffer_get_available_frames", Type: "func(*audioBufferHandle, *uint64) Result"},
{FieldName: "maAudioBufferRefInit", Symbol: "ma_audio_buffer_ref_init", Type: "func(Format, uint32, unsafe.Pointer, uint64, *audioBufferRefHandle) Result"},
{FieldName: "maAudioBufferRefUninit", Symbol: "ma_audio_buffer_ref_uninit", Type: "func(*audioBufferRefHandle)"},
{FieldName: "maAudioBufferRefSetData", Symbol: "ma_audio_buffer_ref_set_data", Type: "func(*audioBufferRefHandle, unsafe.Pointer, uint64) Result"},
{FieldName: "maAudioBufferRefReadPCMFrames", Symbol: "ma_audio_buffer_ref_read_pcm_frames", Type: "func(*audioBufferRefHandle, unsafe.Pointer, uint64, uint32) uint64"},
{FieldName: "maAudioBufferRefSeekToPCMFrame", Symbol: "ma_audio_buffer_ref_seek_to_pcm_frame", Type: "func(*audioBufferRefHandle, uint64) Result"},
{FieldName: "maAudioBufferRefMap", Symbol: "ma_audio_buffer_ref_map", Type: "func(*audioBufferRefHandle, *unsafe.Pointer, *uint64) Result"},
{FieldName: "maAudioBufferRefUnmap", Symbol: "ma_audio_buffer_ref_unmap", Type: "func(*audioBufferRefHandle, uint64) Result"},
{FieldName: "maAudioBufferRefAtEnd", Symbol: "ma_audio_buffer_ref_at_end", Type: "func(*audioBufferRefHandle) uint32"},
{FieldName: "maAudioBufferRefGetCursorInPCMFrames", Symbol: "ma_audio_buffer_ref_get_cursor_in_pcm_frames", Type: "func(*audioBufferRefHandle, *uint64) Result"},
{FieldName: "maAudioBufferRefGetLengthInPCMFrames", Symbol: "ma_audio_buffer_ref_get_length_in_pcm_frames", Type: "func(*audioBufferRefHandle, *uint64) Result"},
{FieldName: "maAudioBufferRefGetAvailableFrames", Symbol: "ma_audio_buffer_ref_get_available_frames", Type: "func(*audioBufferRefHandle, *uint64) Result"},
{FieldName: "maRBInit", Symbol: "ma_rb_init", Type: "func(uintptr, unsafe.Pointer, unsafe.Pointer, *ringBufferHandle) Result"},
{FieldName: "maRBInitEx", Symbol: "ma_rb_init_ex", Type: "func(uintptr, uintptr, uintptr, unsafe.Pointer, unsafe.Pointer, *ringBufferHandle) Result"},
{FieldName: "maRBUninit", Symbol: "ma_rb_uninit", Type: "func(*ringBufferHandle)"},
{FieldName: "maRBReset", Symbol: "ma_rb_reset", Type: "func(*ringBufferHandle)"},
{FieldName: "maRBAcquireRead", Symbol: "ma_rb_acquire_read", Type: "func(*ringBufferHandle, *uintptr, *unsafe.Pointer) Result"},
{FieldName: "maRBCommitRead", Symbol: "ma_rb_commit_read", Type: "func(*ringBufferHandle, uintptr) Result"},
{FieldName: "maRBAcquireWrite", Symbol: "ma_rb_acquire_write", Type: "func(*ringBufferHandle, *uintptr, *unsafe.Pointer) Result"},
{FieldName: "maRBCommitWrite", Symbol: "ma_rb_commit_write", Type: "func(*ringBufferHandle, uintptr) Result"},
{FieldName: "maRBSeekRead", Symbol: "ma_rb_seek_read", Type: "func(*ringBufferHandle, uintptr) Result"},
{FieldName: "maRBSeekWrite", Symbol: "ma_rb_seek_write", Type: "func(*ringBufferHandle, uintptr) Result"},
{FieldName: "maRBPointerDistance", Symbol: "ma_rb_pointer_distance", Type: "func(*ringBufferHandle) int32"},
{FieldName: "maRBAvailableRead", Symbol: "ma_rb_available_read", Type: "func(*ringBufferHandle) uint32"},
{FieldName: "maRBAvailableWrite", Symbol: "ma_rb_available_write", Type: "func(*ringBufferHandle) uint32"},
{FieldName: "maRBGetSubbufferSize", Symbol: "ma_rb_get_subbuffer_size", Type: "func(*ringBufferHandle) uintptr"},
{FieldName: "maRBGetSubbufferStride", Symbol: "ma_rb_get_subbuffer_stride", Type: "func(*ringBufferHandle) uintptr"},
{FieldName: "maRBGetSubbufferOffset", Symbol: "ma_rb_get_subbuffer_offset", Type: "func(*ringBufferHandle, uintptr) uintptr"},
{FieldName: "maRBGetSubbufferPtr", Symbol: "ma_rb_get_subbuffer_ptr", Type: "func(*ringBufferHandle, uintptr, unsafe.Pointer) unsafe.Pointer"},
{FieldName: "maPCMRBInit", Symbol: "ma_pcm_rb_init", Type: "func(Format, uint32, uint32, unsafe.Pointer, unsafe.Pointer, *pcmRingBufferHandle) Result"},
{FieldName: "maPCMRBInitEx", Symbol: "ma_pcm_rb_init_ex", Type: "func(Format, uint32, uint32, uint32, uint32, unsafe.Pointer, unsafe.Pointer, *pcmRingBufferHandle) Result"},
{FieldName: "maPCMRBUninit", Symbol: "ma_pcm_rb_uninit", Type: "func(*pcmRingBufferHandle)"},
{FieldName: "maPCMRBReset", Symbol: "ma_pcm_rb_reset", Type: "func(*pcmRingBufferHandle)"},
{FieldName: "maPCMRBAcquireRead", Symbol: "ma_pcm_rb_acquire_read", Type: "func(*pcmRingBufferHandle, *uint32, *unsafe.Pointer) Result"},
{FieldName: "maPCMRBCommitRead", Symbol: "ma_pcm_rb_commit_read", Type: "func(*pcmRingBufferHandle, uint32) Result"},
{FieldName: "maPCMRBAcquireWrite", Symbol: "ma_pcm_rb_acquire_write", Type: "func(*pcmRingBufferHandle, *uint32, *unsafe.Pointer) Result"},
{FieldName: "maPCMRBCommitWrite", Symbol: "ma_pcm_rb_commit_write", Type: "func(*pcmRingBufferHandle, uint32) Result"},
{FieldName: "maPCMRBSeekRead", Symbol: "ma_pcm_rb_seek_read", Type: "func(*pcmRingBufferHandle, uint32) Result"},
{FieldName: "maPCMRBSeekWrite", Symbol: "ma_pcm_rb_seek_write", Type: "func(*pcmRingBufferHandle, uint32) Result"},
{FieldName: "maPCMRBPointerDistance", Symbol: "ma_pcm_rb_pointer_distance", Type: "func(*pcmRingBufferHandle) int32"},
{FieldName: "maPCMRBAvailableRead", Symbol: "ma_pcm_rb_available_read", Type: "func(*pcmRingBufferHandle) uint32"},
{FieldName: "maPCMRBAvailableWrite", Symbol: "ma_pcm_rb_available_write", Type: "func(*pcmRingBufferHandle) uint32"},
{FieldName: "maPCMRBGetFormat", Symbol: "ma_pcm_rb_get_format", Type: "func(*pcmRingBufferHandle) Format"},
{FieldName: "maPCMRBGetChannels", Symbol: "ma_pcm_rb_get_channels", Type: "func(*pcmRingBufferHandle) uint32"},
{FieldName: "maPCMRBGetSampleRate", Symbol: "ma_pcm_rb_get_sample_rate", Type: "func(*pcmRingBufferHandle) uint32"},
{FieldName: "maPCMRBGetSubbufferSize", Symbol: "ma_pcm_rb_get_subbuffer_size", Type: "func(*pcmRingBufferHandle) uint32"},
{FieldName: "maPCMRBGetSubbufferStride", Symbol: "ma_pcm_rb_get_subbuffer_stride", Type: "func(*pcmRingBufferHandle) uint32"},
{FieldName: "maPCMRBGetSubbufferOffset", Symbol: "ma_pcm_rb_get_subbuffer_offset", Type: "func(*pcmRingBufferHandle, uint32) uint32"},
{FieldName: "maPCMRBGetSubbufferPtr", Symbol: "ma_pcm_rb_get_subbuffer_ptr", Type: "func(*pcmRingBufferHandle, uint32, unsafe.Pointer) unsafe.Pointer"},
```

- [ ] **Step 5: Regenerate, rebuild, add assertions, verify**

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
```

Add `unsafe.Sizeof`/`Offsetof` assertions for `audioBufferConfigNative`
(`SizeInFrames`, `Data`, `AllocationCallbacks`) and `allocationCallbacksNative`
to `layout_test.go`.

Run: `go test . -run TestMirroredStructLayouts -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go \
  internal/abi/layout_probe.c layout_test.go zz_generated.bindings.go embed_*.go
git commit -m "refactor: add Phase 4 bindings, mirrors and buffer allocation"
```

---

## Task 2: Audio buffer and buffer reference

**Files:**
- Create: `buffer.go`
- Create: `buffer_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectAudioBuffer)`, `magoAlloc(magoObjectAudioBufferRef)`, the `maAudioBuffer*` bindings.
- Produces: `type AudioBufferConfig struct { Format Format; Channels uint32; SampleRate uint32; SizeInFrames uint64; Data unsafe.Pointer }`.
- Produces: `func (lib *Library) NewAudioBuffer(config AudioBufferConfig) (*AudioBuffer, error)` (references `Data`) and `NewAudioBufferCopy(config)` (copies).
- Produces: `AudioBuffer` with `ReadPCMFrames(out unsafe.Pointer, frameCount uint64, loop bool) (uint64, error)`, `SeekToPCMFrame`, `Map() (unsafe.Pointer, uint64, error)`, `Unmap(frameCount uint64)`, `CursorInPCMFrames`, `LengthInPCMFrames`, `AvailableFrames`, `Close`.
- Produces: `func (lib *Library) NewAudioBufferRef(format Format, channels uint32, data unsafe.Pointer, sizeInFrames uint64) (*AudioBufferRef, error)` and the same method set plus `SetData`, `AtEnd`.

- [ ] **Step 1: Write the failing test**

```go
// buffer_test.go
package mago

import (
	"testing"
	"unsafe"
)

func TestAudioBufferReadAndSeek(t *testing.T) {
	lib := newNullLibrary(t)

	samples := []float32{0, 1, 2, 3, 4, 5, 6, 7}
	buffer, err := lib.NewAudioBuffer(AudioBufferConfig{
		Format:       FormatF32,
		Channels:     1,
		SampleRate:   48_000,
		SizeInFrames: uint64(len(samples)),
		Data:         unsafe.Pointer(&samples[0]),
	})
	if err != nil {
		t.Fatalf("NewAudioBuffer: %v", err)
	}
	defer func() { _ = buffer.Close() }()

	if length, err := buffer.LengthInPCMFrames(); err != nil || length != uint64(len(samples)) {
		t.Fatalf("length = %d (%v), want %d", length, err, len(samples))
	}

	out := make([]float32, 4)
	read, err := buffer.ReadPCMFrames(unsafe.Pointer(&out[0]), uint64(len(out)), false)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != uint64(len(out)) {
		t.Fatalf("read %d frames, want %d", read, len(out))
	}
	if out[0] != samples[0] || out[3] != samples[3] {
		t.Fatalf("read %v, want the first four source samples", out)
	}

	if err := buffer.SeekToPCMFrame(0); err != nil {
		t.Fatalf("SeekToPCMFrame: %v", err)
	}
	if cursor, err := buffer.CursorInPCMFrames(); err != nil || cursor != 0 {
		t.Fatalf("cursor = %d (%v), want 0", cursor, err)
	}
}

func TestAudioBufferRefDoesNotOwnData(t *testing.T) {
	lib := newNullLibrary(t)

	samples := []float32{1, 2, 3, 4}
	ref, err := lib.NewAudioBufferRef(FormatF32, 1, unsafe.Pointer(&samples[0]), uint64(len(samples)))
	if err != nil {
		t.Fatalf("NewAudioBufferRef: %v", err)
	}
	defer func() { _ = ref.Close() }()

	if ref.AtEnd() {
		t.Fatal("a fresh reference should not report end-of-buffer")
	}

	out := make([]float32, 2)
	read, err := ref.ReadPCMFrames(unsafe.Pointer(&out[0]), 2, false)
	if err != nil {
		t.Fatalf("ReadPCMFrames: %v", err)
	}
	if read != 2 || out[0] != 1 {
		t.Fatalf("read %d frames %v, want 2 starting at 1", read, out)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run 'TestAudioBuffer' -v`
Expected: FAIL - `undefined: AudioBufferConfig`.

- [ ] **Step 3: Implement**

`NewAudioBuffer` builds an `audioBufferConfigNative`, allocates via
`magoAlloc(magoObjectAudioBuffer)`, calls `maAudioBufferInit`, and stores the
`Data` reference on the wrapper so the Go slice cannot be collected while
miniaudio points at it. `NewAudioBufferCopy` calls `maAudioBufferInitCopy`.
`ReadPCMFrames` converts `loop bool` to `uint32` and returns the frame count.

`NewAudioBufferRef` calls `maAudioBufferRefInit` (no config struct) and stores
the data reference too.

- [ ] **Step 4: Run the tests**

Run: `go test . -run 'TestAudioBuffer' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add buffer.go buffer_test.go
git commit -m "feat: expose in-memory audio buffers and buffer references"
```

---

## Task 3: Byte ring buffer

**Files:**
- Modify: `buffer.go`
- Modify: `buffer_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectRingBuffer)`, the `maRB*` bindings.
- Produces: `func (lib *Library) NewRingBuffer(bufferSizeInBytes uint) (*RingBuffer, error)` and `NewRingBufferEx(subbufferSizeInBytes, subbufferCount, subbufferStrideInBytes uint)`.
- Produces: `RingBuffer` with `AcquireRead() (unsafe.Pointer, uint, error)`, `CommitRead(uint) error`, `AcquireWrite() (unsafe.Pointer, uint, error)`, `CommitWrite(uint) error`, `SeekRead`, `SeekWrite`, `Reset`, `PointerDistance() int32`, `AvailableRead() uint32`, `AvailableWrite() uint32`, `Close`.

- [ ] **Step 1: Write the failing test**

```go
func TestRingBufferWriteThenRead(t *testing.T) {
	lib := newNullLibrary(t)

	ring, err := lib.NewRingBuffer(256)
	if err != nil {
		t.Fatalf("NewRingBuffer: %v", err)
	}
	defer func() { _ = ring.Close() }()

	if ring.AvailableWrite() == 0 {
		t.Fatal("expected space to write into a fresh ring buffer")
	}

	ptr, size, err := ring.AcquireWrite()
	if err != nil {
		t.Fatalf("AcquireWrite: %v", err)
	}
	if ptr == nil || size == 0 {
		t.Fatal("expected a writable region")
	}
	region := unsafe.Slice((*byte)(ptr), size)
	for i := range region {
		region[i] = byte(i)
	}
	if err := ring.CommitWrite(size); err != nil {
		t.Fatalf("CommitWrite: %v", err)
	}

	if got := ring.AvailableRead(); got < uint32(size) {
		t.Fatalf("available read = %d, want at least %d", got, size)
	}

	readPtr, readSize, err := ring.AcquireRead()
	if err != nil {
		t.Fatalf("AcquireRead: %v", err)
	}
	read := unsafe.Slice((*byte)(readPtr), readSize)
	if read[0] != 0 || read[1] != 1 {
		t.Fatalf("read %v, want the written bytes", read[:4])
	}
	if err := ring.CommitRead(readSize); err != nil {
		t.Fatalf("CommitRead: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestRingBufferWriteThenRead -v`
Expected: FAIL - `lib.NewRingBuffer undefined`.

- [ ] **Step 3: Implement**

Allocate via `magoAlloc(magoObjectRingBuffer)` and call `maRBInit` / `maRBInitEx`
with nil preallocated buffers and callbacks. `AcquireRead`/`AcquireWrite` return
a pointer plus the number of bytes available in that region.
`PointerDistance` is documented as bytes that can be read before catching the
write pointer.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestRingBufferWriteThenRead -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add buffer.go buffer_test.go
git commit -m "feat: expose the byte ring buffer"
```

---

## Task 4: PCM ring buffer

**Files:**
- Modify: `buffer.go`
- Modify: `buffer_test.go`

**Interfaces:**
- Consumes: `magoAlloc(magoObjectPCMRingBuffer)`, the `maPCMRB*` bindings.
- Produces: `func (lib *Library) NewPCMRingBuffer(format Format, channels, bufferSizeInFrames uint32) (*PCMRingBuffer, error)` and `NewPCMRingBufferEx(format Format, channels, subbufferSizeInFrames, subbufferCount, subbufferStrideInFrames uint32)`.
- Produces: `PCMRingBuffer` with frame-based `AcquireRead() (unsafe.Pointer, uint32, error)`, `CommitRead(uint32) error`, `AcquireWrite`, `CommitWrite`, `SeekRead`, `SeekWrite`, `Reset`, `PointerDistance() int32`, `AvailableRead()/AvailableWrite() uint32`, `Format() Format`, `Channels() uint32`, `SampleRate() uint32`, `Close`.

- [ ] **Step 1: Write the failing test**

```go
func TestPCMRingBufferRoundTrip(t *testing.T) {
	lib := newNullLibrary(t)

	ring, err := lib.NewPCMRingBuffer(FormatF32, 1, 64)
	if err != nil {
		t.Fatalf("NewPCMRingBuffer: %v", err)
	}
	defer func() { _ = ring.Close() }()

	if ring.Format() != FormatF32 || ring.Channels() != 1 {
		t.Fatalf("ring reports format %v channels %d", ring.Format(), ring.Channels())
	}

	ptr, frames, err := ring.AcquireWrite()
	if err != nil {
		t.Fatalf("AcquireWrite: %v", err)
	}
	if frames == 0 {
		t.Fatal("expected writable frames")
	}
	region := unsafe.Slice((*float32)(ptr), frames)
	for i := range region {
		region[i] = float32(i)
	}
	if err := ring.CommitWrite(frames); err != nil {
		t.Fatalf("CommitWrite: %v", err)
	}

	readPtr, readFrames, err := ring.AcquireRead()
	if err != nil {
		t.Fatalf("AcquireRead: %v", err)
	}
	read := unsafe.Slice((*float32)(readPtr), readFrames)
	if read[0] != 0 || read[1] != 1 {
		t.Fatalf("read %v, want the written frames", read[:4])
	}
	if err := ring.CommitRead(readFrames); err != nil {
		t.Fatalf("CommitRead: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestPCMRingBufferRoundTrip -v`
Expected: FAIL - `lib.NewPCMRingBuffer undefined`.

- [ ] **Step 3: Implement**

Same shape as `RingBuffer` but frame-based; `NewPCMRingBuffer` delegates to
`maPCMRBInit` and records nothing extra because miniaudio can report format,
channels and sample rate.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestPCMRingBufferRoundTrip -v`
Expected: PASS.

- [ ] **Step 5: Add unsupported-platform stubs**

In `unsupported.go`, add `AudioBufferConfig`, `AudioBuffer`, `AudioBufferRef`,
`RingBuffer`, `PCMRingBuffer`, their constructors returning
`errUnsupportedPlatform`, and their methods, so non-supported GOOS still
type-checks.

- [ ] **Step 6: Full test, lint, and commit**

Run: `mise run test && mise run lint && mise run check-cgo`
Expected: PASS, 0 lint issues.

```bash
git add buffer.go buffer_test.go unsupported.go
git commit -m "feat: expose the PCM ring buffer"
```

---

## Task 5: Example and README

**Files:**
- Create: `examples/buffers/main.go`
- Modify: `README.md`

**Interfaces:** none exported; uses the public API from Tasks 2-4.

- [ ] **Step 1: Write the example**

A single `examples/buffers` command that:
1. wraps a slice in an `AudioBuffer` and reads it back, reporting length/cursor,
2. wraps the same data in an `AudioBufferRef` and shows `AtEnd` after a full read,
3. pushes bytes through a `RingBuffer` and reads them back,
4. pushes float frames through a `PCMRingBuffer` and reads them back.

It must use the shared `examples/internal/example` helper and need no audio
device.

- [ ] **Step 2: Run it headless**

Run: `env -i PATH=/usr/bin:/bin HOME="$HOME" go run ./examples/buffers`
Expected: prints the buffer and ring-buffer round trips without error.

- [ ] **Step 3: Document it**

Add a `go run ./examples/buffers` block to the README demos section and add
`buffers` to the list of device-free examples.

- [ ] **Step 4: Verify and commit**

Run: `go build -trimpath ./examples/... && go test ./... && mise run lint`
Expected: PASS.

```bash
git add examples/buffers README.md
git commit -m "docs: add a buffers and ring buffers example"
```

---

## Phase 4 exit criteria

- `AudioBuffer` (referencing and copying) and `AudioBufferRef` read, seek and
  report cursor/length/available frames.
- `RingBuffer` and `PCMRingBuffer` acquire/commit both directions correctly.
- Go data referenced by miniaudio is kept alive for the object's lifetime.
- `mise run test`, `mise run lint`, `mise run check-cgo` are green.
- `mise run build-lib-all && mise run generate` leaves no diff.
- `bd ready` surfaces the next phases once Phase 4 closes.