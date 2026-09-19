# Phase 1: Low-Level Device and Context Completeness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Round out the low-level device and context API: capture, duplex and loopback device types, device state/name/info/log accessors, master volume, and context log/config access.

**Architecture:** Generalize the single `mago_device_init_playback` C shim into one `mago_device_init` that builds any `ma_device_type` config (still category 2: by-value, large, backend-specific struct). Everything else binds exported `ma_*` symbols directly through purego. One batched native rebuild for the whole phase.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, osxcross (Docker), mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (section 5, Phase 1)
**Predecessor plan:** `docs/superpowers/plans/2026-09-19-miniaudio-api-expansion-phase-0.md`

## Global Constraints

- **ZERO CGO.** No `import "C"`, no cgo files. `mise run check-cgo` must pass.
- **Minimal C.** Only the three allowed categories: allocation, by-value/large config, callbacks with no `pUserData`. Do not add wrappers for functions that already have a public `ma_*` symbol.
- **One native rebuild for the phase.** All C changes land in Task 1; Tasks 2-5 are Go-only. After Task 1: `mise run build-lib-all && mise run generate`, then commit the regenerated `embed_*.go`.
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Tests never touch real hardware.** Use `mago.BackendNull` and the Phase 0 helpers in `lifecycle_test.go`.
- **Mirrors stay validated.** Any new Go struct that mirrors miniaudio layout is asserted in `layout_test.go` via `internal/abi`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

- `ma_device_get_name(ma_device*, ma_device_type, char* pName, size_t nameCap, size_t* pLength)` - caller supplies the buffer.
- `ma_device_get_info(ma_device*, ma_device_type, ma_device_info*)` and `ma_context_get_device_info(ma_context*, ma_device_type, const ma_device_id*, ma_device_info*)` write a full `ma_device_info` (1544 bytes), so Go allocates it through `mago_alloc` and reads only the prefix mirror.
- `ma_device_get_log(ma_device*)` and `ma_context_get_log(ma_context*)` return a borrowed `ma_log*`.
- `ma_device_get_context(ma_device*)` returns a borrowed `ma_context*`.
- `ma_device_get_state(const ma_device*)` returns `ma_device_state` (`uninitialized=0, stopped=1, started=2, starting=3, stopping=4`).
- Master volume: `ma_device_set_master_volume(ma_device*, float)`, `ma_device_get_master_volume(ma_device*, float*)`, `ma_device_set_master_volume_db(ma_device*, float)`, `ma_device_get_master_volume_db(ma_device*, float*)`.
- `ma_context_config_init()` returns `ma_context_config` (240 bytes) by value; its first field is `ma_log* pLog`, and it also carries `ma_allocation_callbacks` and per-backend sub-structs.
- `sizeof(ma_device) = 3776`, `sizeof(ma_device_info) = 1544`, `sizeof(ma_context_config) = 240`.

---

## Task 1: Phase 1 bridge revision (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c`
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go` (native mirrors and object constants)
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces C: `void* mago_alloc(int type)` gains `MAGO_OBJECT_DEVICE_INFO = 4` and `MAGO_OBJECT_CONTEXT_CONFIG = 5`.
- Produces C: `ma_result mago_device_init(ma_context*, const mago_device_config*, ma_device**)` replacing `mago_device_init_playback`.
- Produces C: `void mago_context_config_init(void* pOut)` and `void mago_context_config_set_log(void* pConfig, ma_log* pLog)`.
- Produces Go bindings (exact field names): `maDeviceGetState`, `maDeviceGetName`, `maDeviceGetInfo`, `maDeviceGetLog`, `maDeviceGetContext`, `maContextGetLog`, `maContextGetDeviceInfo`, `maDeviceSetMasterVolume`, `maDeviceGetMasterVolume`, `maDeviceSetMasterVolumeDB`, `maDeviceGetMasterVolumeDB`, `magoContextConfigInit`, `magoContextConfigSetLog`.

- [ ] **Step 1: Generalize the device config in C**

In `native/miniaudio_bridge.c`, replace `mago_playback_device_config` and
`mago_device_init_playback` with:

```c
typedef struct
{
    const ma_device_id* pDeviceID;
    ma_int32 deviceIndex;
    ma_format format;
    ma_uint32 channels;
    ma_uint32 sampleRate;
    ma_uint32 periodSizeInFrames;
    ma_uint32 periodSizeInMilliseconds;
    ma_uint32 periods;
    ma_performance_profile performanceProfile;
    ma_share_mode shareMode;
    ma_bool32 noPreSilencedOutputBuffer;
    ma_bool32 noClip;
    ma_bool32 noDisableDenormals;
    ma_bool32 noFixedSizedCallback;
} mago_stream_config;

typedef struct
{
    ma_uint32 deviceType;
    const mago_stream_config* pPlayback;
    const mago_stream_config* pCapture;
    uintptr_t dataCallback;
    uintptr_t notificationCallback;
    uintptr_t userData;
} mago_device_config;

static const ma_device_id* mago_resolve_device_id(ma_context* pContext, ma_device_type type, const mago_stream_config* pStream)
{
    ma_device_info* pInfos;
    ma_uint32 count;

    if (pStream->deviceIndex < 0 || pContext == NULL)
    {
        return pStream->pDeviceID;
    }

    if (type == ma_device_type_capture)
    {
        if (ma_context_get_devices(pContext, NULL, NULL, &pInfos, &count) != MA_SUCCESS)
        {
            return NULL;
        }
    }
    else
    {
        if (ma_context_get_devices(pContext, &pInfos, &count, NULL, NULL) != MA_SUCCESS)
        {
            return NULL;
        }
    }

    if ((ma_uint32)pStream->deviceIndex >= count)
    {
        return NULL;
    }
    return &pInfos[pStream->deviceIndex].id;
}

static void mago_apply_stream_config(ma_device_config* pConfig, const mago_stream_config* pStream, ma_device_type type, const ma_device_id* pDeviceID)
{
    if (type == ma_device_type_capture || type == ma_device_type_loopback)
    {
        pConfig->capture.pDeviceID = pDeviceID;
        pConfig->capture.format = pStream->format;
        pConfig->capture.channels = pStream->channels;
        pConfig->capture.shareMode = pStream->shareMode;
    }
    else
    {
        pConfig->playback.pDeviceID = pDeviceID;
        pConfig->playback.format = pStream->format;
        pConfig->playback.channels = pStream->channels;
        pConfig->playback.shareMode = pStream->shareMode;
    }
}

MAGO_API ma_result mago_device_init(
    ma_context* pContext,
    const mago_device_config* pMagoConfig,
    ma_device** ppDevice)
{
    ma_device_config config;
    ma_device* pDevice;
    mago_device_bridge* pBridge;
    ma_device_type type;
    ma_result result;

    if (pMagoConfig == NULL || ppDevice == NULL)
    {
        return MA_INVALID_ARGS;
    }

    *ppDevice = NULL;
    type = (ma_device_type)pMagoConfig->deviceType;

    pDevice = (ma_device*)calloc(1, sizeof(ma_device));
    if (pDevice == NULL)
    {
        return MA_OUT_OF_MEMORY;
    }

    pBridge = (mago_device_bridge*)calloc(1, sizeof(mago_device_bridge));
    if (pBridge == NULL)
    {
        free(pDevice);
        return MA_OUT_OF_MEMORY;
    }

    pBridge->dataCallback = (mago_data_callback)pMagoConfig->dataCallback;
    pBridge->notificationCallback = (mago_notification_callback)pMagoConfig->notificationCallback;
    pBridge->userData = pMagoConfig->userData;

    config = ma_device_config_init(type);

    if (type == ma_device_type_playback || type == ma_device_type_duplex)
    {
        if (pMagoConfig->pPlayback == NULL)
        {
            free(pBridge);
            free(pDevice);
            return MA_INVALID_ARGS;
        }
        mago_apply_stream_config(&config, pMagoConfig->pPlayback, ma_device_type_playback,
            mago_resolve_device_id(pContext, ma_device_type_playback, pMagoConfig->pPlayback));
        config.sampleRate = pMagoConfig->pPlayback->sampleRate;
        config.periodSizeInFrames = pMagoConfig->pPlayback->periodSizeInFrames;
        config.periodSizeInMilliseconds = pMagoConfig->pPlayback->periodSizeInMilliseconds;
        config.periods = pMagoConfig->pPlayback->periods;
        config.performanceProfile = pMagoConfig->pPlayback->performanceProfile;
        config.noPreSilencedOutputBuffer = (ma_bool8)pMagoConfig->pPlayback->noPreSilencedOutputBuffer;
        config.noClip = (ma_bool8)pMagoConfig->pPlayback->noClip;
        config.noDisableDenormals = (ma_bool8)pMagoConfig->pPlayback->noDisableDenormals;
        config.noFixedSizedCallback = (ma_bool8)pMagoConfig->pPlayback->noFixedSizedCallback;
    }

    if (type == ma_device_type_capture || type == ma_device_type_duplex || type == ma_device_type_loopback)
    {
        if (pMagoConfig->pCapture == NULL)
        {
            free(pBridge);
            free(pDevice);
            return MA_INVALID_ARGS;
        }
        mago_apply_stream_config(&config, pMagoConfig->pCapture, type,
            mago_resolve_device_id(pContext, type, pMagoConfig->pCapture));
        if (type != ma_device_type_duplex)
        {
            config.sampleRate = pMagoConfig->pCapture->sampleRate;
            config.periodSizeInFrames = pMagoConfig->pCapture->periodSizeInFrames;
            config.periodSizeInMilliseconds = pMagoConfig->pCapture->periodSizeInMilliseconds;
            config.periods = pMagoConfig->pCapture->periods;
            config.performanceProfile = pMagoConfig->pCapture->performanceProfile;
            config.noFixedSizedCallback = (ma_bool8)pMagoConfig->pCapture->noFixedSizedCallback;
        }
    }

    config.dataCallback = pBridge->dataCallback != NULL ? mago_on_device_data : NULL;
    config.notificationCallback = pBridge->notificationCallback != NULL ? mago_on_device_notification : NULL;
    config.pUserData = pBridge;

    result = ma_device_init(pContext, &config, pDevice);
    if (result != MA_SUCCESS)
    {
        free(pBridge);
        free(pDevice);
        return result;
    }

    *ppDevice = pDevice;
    return MA_SUCCESS;
}
```

Update the file's header comment to list the new understanding (device type
generalization) under category 2. Do not change `mago_device_uninit_free`,
`mago_alloc`/`mago_free`, or the trampolines.

- [ ] **Step 2: Extend `mago_alloc` and add context-config shims**

Add two enum values and cases, plus the context-config helpers:

```c
enum mago_object_type
{
    MAGO_OBJECT_CONTEXT = 1,
    MAGO_OBJECT_DEVICE  = 2,
    MAGO_OBJECT_LOG     = 3,
    MAGO_OBJECT_DEVICE_INFO = 4,
    MAGO_OBJECT_CONTEXT_CONFIG = 5
};
```

```c
        case MAGO_OBJECT_DEVICE_INFO:      return calloc(1, sizeof(ma_device_info));
        case MAGO_OBJECT_CONTEXT_CONFIG:   return calloc(1, sizeof(ma_context_config));
```

```c
MAGO_API void mago_context_config_init(void* pOut)
{
    if (pOut != NULL)
    {
        *(ma_context_config*)pOut = ma_context_config_init();
    }
}

MAGO_API void mago_context_config_set_log(void* pConfig, ma_log* pLog)
{
    if (pConfig != NULL)
    {
        ((ma_context_config*)pConfig)->pLog = pLog;
    }
}
```

- [ ] **Step 3: Verify the C compiles and exports**

Run:

```bash
zig cc -std=c11 -O2 -fPIC -shared -Wl,-soname,libminiaudio.so -I . \
  -o /tmp/libminiaudio_p1.so native/miniaudio_bridge.c -ldl -lm -lpthread
nm -D --defined-only /tmp/libminiaudio_p1.so | grep -E 'mago_(alloc|device_init|context_config_init|context_config_set_log)$'
```

Expected: the four symbols are listed.

- [ ] **Step 4: Update the Go mirrors**

In `types.go`:

```go
// streamConfigNative mirrors mago_stream_config (the bridge's own struct, not a
// miniaudio struct).
type streamConfigNative struct {
	DeviceID                  unsafe.Pointer
	DeviceIndex               int32
	Format                    Format
	Channels                  uint32
	SampleRate                uint32
	PeriodSizeInFrames        uint32
	PeriodSizeInMilliseconds  uint32
	Periods                   uint32
	PerformanceProfile        PerformanceProfile
	ShareMode                 ShareMode
	NoPreSilencedOutputBuffer uint32
	NoClip                    uint32
	NoDisableDenormals        uint32
	NoFixedSizedCallback      uint32
}

type deviceConfigNative struct {
	DeviceType           uint32
	Playback             *streamConfigNative
	Capture              *streamConfigNative
	DataCallback         uintptr
	NotificationCallback uintptr
	UserData             uintptr
}

const (
	magoObjectContext        int32 = 1
	magoObjectDevice         int32 = 2
	magoObjectLog            int32 = 3
	magoObjectDeviceInfo     int32 = 4
	magoObjectContextConfig  int32 = 5
)
```

Delete `playbackDeviceConfigNative`.

- [ ] **Step 5: Update the bindings generator**

Replace the single device entry and add the Phase 1 symbols in
`internal/gen/bindings/main.go`:

```go
{FieldName: "magoDeviceInit", Symbol: "mago_device_init", Type: "func(*contextHandle, *deviceConfigNative, **deviceHandle) Result"},
{FieldName: "maDeviceGetState", Symbol: "ma_device_get_state", Type: "func(*deviceHandle) DeviceState"},
{FieldName: "maDeviceGetName", Symbol: "ma_device_get_name", Type: "func(*deviceHandle, DeviceType, *byte, uintptr, *uintptr) Result"},
{FieldName: "maDeviceGetInfo", Symbol: "ma_device_get_info", Type: "func(*deviceHandle, DeviceType, *deviceInfoNative) Result"},
{FieldName: "maDeviceGetLog", Symbol: "ma_device_get_log", Type: "func(*deviceHandle) *logHandle"},
{FieldName: "maDeviceGetContext", Symbol: "ma_device_get_context", Type: "func(*deviceHandle) *contextHandle"},
{FieldName: "maContextGetLog", Symbol: "ma_context_get_log", Type: "func(*contextHandle) *logHandle"},
{FieldName: "maContextGetDeviceInfo", Symbol: "ma_context_get_device_info", Type: "func(*contextHandle, DeviceType, unsafe.Pointer, *deviceInfoNative) Result"},
{FieldName: "maDeviceSetMasterVolume", Symbol: "ma_device_set_master_volume", Type: "func(*deviceHandle, float32) Result"},
{FieldName: "maDeviceGetMasterVolume", Symbol: "ma_device_get_master_volume", Type: "func(*deviceHandle, *float32) Result"},
{FieldName: "maDeviceSetMasterVolumeDB", Symbol: "ma_device_set_master_volume_db", Type: "func(*deviceHandle, float32) Result"},
{FieldName: "maDeviceGetMasterVolumeDB", Symbol: "ma_device_get_master_volume_db", Type: "func(*deviceHandle, *float32) Result"},
{FieldName: "magoContextConfigInit", Symbol: "mago_context_config_init", Type: "func(unsafe.Pointer)"},
{FieldName: "magoContextConfigSetLog", Symbol: "mago_context_config_set_log", Type: "func(unsafe.Pointer, *logHandle)"},
```

Add `DeviceState` to `types.go`:

```go
type DeviceState int32

const (
	DeviceStateUninitialized DeviceState = 0
	DeviceStateStopped       DeviceState = 1
	DeviceStateStarted       DeviceState = 2
	DeviceStateStarting      DeviceState = 3
	DeviceStateStopping      DeviceState = 4
)
```

- [ ] **Step 6: Regenerate, build, and rebuild the libraries**

Run:

```bash
go run ./internal/gen/bindings
go build ./...
mise run build-lib-all
mise run generate
```

Expected: `zz_generated.bindings.go` and all `embed_*.go` update.

- [ ] **Step 7: Verify no drift and commit**

Run: `git status --short`
Expected: modified bridge, generator, `types.go`, generated files.

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go types.go zz_generated.bindings.go embed_*.go
git commit -m "refactor: generalize the device bridge and add Phase 1 bindings"
```

---

## Task 2: Device types (capture, duplex, loopback)

**Files:**
- Modify: `device_supported.go`
- Modify: `unsupported.go`
- Create: `device_test.go`
- Regenerate: none

**Interfaces:**
- Consumes: `deviceConfigNative`, `magoDeviceInit`, `DeviceType*` constants.
- Produces: `type StreamConfig struct` (the current `PlaybackDeviceConfig` fields minus callbacks).
- Produces: `type DeviceConfig struct { Type DeviceType; Playback, Capture *StreamConfig; DataCallback; NotificationCallback }`.
- Produces: `func (lib *Library) NewDevice(ctx *Context, config DeviceConfig) (*Device, error)`.
- Produces: `func (ctx *Context) NewDevice(config DeviceConfig) (*Device, error)`.
- Keeps: `type PlaybackDeviceConfig = StreamConfig` (compat alias) and `NewPlaybackDevice` translating to `DeviceConfig`.

- [ ] **Step 1: Write the failing test**

```go
// device_test.go
package mago

import (
	"testing"
	"time"
	"unsafe"
)

func TestNullBackendCaptureDevice(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	got := make(chan uint32, 8)
	device, err := ctx.NewDevice(DeviceConfig{
		Type: DeviceTypeCapture,
		Capture: &StreamConfig{
			DeviceIndex:        -1,
			Channels:           1,
			SampleRate:         48000,
			PeriodSizeInFrames: 64,
		},
		DataCallback: func(_ *Device, _ unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
			_ = input
			select {
			case got <- frameCount:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewDevice(capture): %v", err)
	}
	defer func() { _ = device.Close() }()

	if err := device.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = device.Stop() }()

	select {
	case n := <-got:
		if n == 0 {
			t.Fatal("expected non-zero capture frame count")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for capture callback")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestNullBackendCaptureDevice -v`
Expected: FAIL - `undefined: DeviceConfig` / `undefined: StreamConfig`.

- [ ] **Step 3: Implement the stream/device config and `NewDevice`**

Rename the current `PlaybackDeviceConfig` struct fields into `StreamConfig`
(keeping every field except `DataCallback`/`NotificationCallback`), then add:

```go
type StreamConfig struct {
	DeviceIndex               int
	Format                    Format
	Channels                  uint32
	SampleRate                uint32
	PeriodSizeInFrames        uint32
	PeriodSizeInMilliseconds  uint32
	Periods                   uint32
	PerformanceProfile        PerformanceProfile
	ShareMode                 ShareMode
	NoPreSilencedOutputBuffer bool
	NoClip                    bool
	NoDisableDenormals        bool
	NoFixedSizedCallback      bool
}

type DeviceConfig struct {
	Type                 DeviceType
	Playback             *StreamConfig
	Capture              *StreamConfig
	DataCallback         DataCallback
	NotificationCallback NotificationCallback
}
```

`NewDevice` validates: `Type` is one of the four; playback required for
playback/duplex; capture required for capture/duplex/loopback; device indices
only allowed when `ctx != nil`. It registers the callback state exactly as the
current `NewPlaybackDevice` does, builds a `deviceConfigNative` (converting
`StreamConfig` to `streamConfigNative` and setting `DeviceType`), calls
`magoDeviceInit`, and clears the callback on failure.

`func (ctx *Context) NewDevice(config DeviceConfig) (*Device, error)` forwards to
`ctx.lib.NewDevice(ctx, config)`.

Keep backward compatibility:

```go
// Deprecated: use StreamConfig with DeviceConfig.
type PlaybackDeviceConfig = StreamConfig
```

and change `NewPlaybackDevice` to translate its `PlaybackDeviceConfig` argument
into a `DeviceConfig{Type: DeviceTypePlayback, Playback: ...}`.

- [ ] **Step 4: Run the capture test**

Run: `go test . -run TestNullBackendCaptureDevice -v`
Expected: PASS.

- [ ] **Step 5: Add a duplex smoke test**

Add a duplex test mirroring Step 1 with `Type: DeviceTypeDuplex`, both
`Playback` and `Capture` set, asserting the data callback fires. Null-backend
duplex support may be limited; if the null backend refuses duplex, assert that
`NewDevice` returns an `*OpError` instead and skip the playback assertion.
Record the observed behavior in a comment.

Run: `go test . -run 'TestNullBackend(Capture|Duplex)' -v`
Expected: PASS (or the documented OpError assertion).

- [ ] **Step 6: Add unsupported-platform stubs**

In `unsupported.go`, add `StreamConfig`, `DeviceConfig`, and
`func (*Context) NewDevice(DeviceConfig) (*Device, error)` /
`func (*Library) NewDevice(*Context, DeviceConfig) (*Device, error)` returning
`errUnsupportedPlatform`. Keep `DefaultPlaybackDeviceConfig` working.

- [ ] **Step 7: Full test and commit**

Run: `mise run test`
Expected: PASS.

```bash
git add device_supported.go unsupported.go device_test.go
git commit -m "feat: support capture, duplex and loopback device types"
```

---

## Task 3: Device introspection (state, name, info, log)

**Files:**
- Modify: `device_supported.go`
- Modify: `types.go` (borrowed-ownership flag on `Log`/`Context` if not already present)
- Modify: `log_supported.go` (borrowed `Log`)
- Create: `device_info_test.go`

**Interfaces:**
- Consumes: `maDeviceGetState`, `maDeviceGetName`, `maDeviceGetInfo`, `maDeviceGetLog`, `maContextGetLog`, `magoAlloc(magoObjectDeviceInfo)`.
- Produces: `func (d *Device) State() DeviceState`.
- Produces: `func (d *Device) Name() (string, error)` and `func (d *Device) Info() (DeviceInfo, error)`.
- Produces: `func (d *Device) Log() *Log` (borrowed; nil when none attached).
- Produces: `func (ctx *Context) Log() *Log` (borrowed).
- Produces: `func (d *Device) Context() *Context` (borrowed).

- [ ] **Step 1: Add borrowed ownership to `Log` and `Context`**

Extend both types with an unexported `owned bool`. `NewLog`/`NewContext` set it
true. `Close` only calls `ma_log_uninit`/`mago_free` (or
`ma_context_uninit`/`mago_free`) when `owned`; for borrowed handles `Close`
just detaches the handle. Add a helper constructor for borrowed handles:

```go
func borrowedLog(lib *Library, handle *logHandle) *Log {
	if handle == nil {
		return nil
	}
	return &Log{lib: lib, handle: handle, owned: false}
}
```

- [ ] **Step 2: Implement the accessors**

```go
func (device *Device) State() DeviceState {
	if device == nil || device.handle == nil {
		return DeviceStateUninitialized
	}
	return device.lib.bindings.maDeviceGetState(device.handle)
}

func (device *Device) Name() (string, error) {
	if device == nil || device.handle == nil {
		return "", fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return "", err
	}

	var buffer [256]byte
	var length uintptr
	result := device.lib.bindings.maDeviceGetName(device.handle, DeviceTypePlayback, &buffer[0], uintptr(len(buffer)), &length)
	if result != Success {
		return "", device.lib.resultError("ma_device_get_name", result)
	}
	if length > uintptr(len(buffer)) {
		length = uintptr(len(buffer))
	}
	return string(buffer[:length]), nil
}

func (device *Device) Info() (DeviceInfo, error) {
	if device == nil || device.handle == nil {
		return DeviceInfo{}, fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return DeviceInfo{}, err
	}

	native := (*deviceInfoNative)(device.lib.bindings.magoAlloc(magoObjectDeviceInfo))
	if native == nil {
		return DeviceInfo{}, fmt.Errorf("mago: allocate device info: out of memory")
	}
	defer device.lib.bindings.magoFree(unsafe.Pointer(native))

	result := device.lib.bindings.maDeviceGetInfo(device.handle, DeviceTypePlayback, native)
	if result != Success {
		return DeviceInfo{}, device.lib.resultError("ma_device_get_info", result)
	}
	return copyDeviceInfo(native), nil
}

func (device *Device) Log() *Log {
	if device == nil || device.handle == nil {
		return nil
	}
	return borrowedLog(device.lib, device.lib.bindings.maDeviceGetLog(device.handle))
}

func (device *Device) Context() *Context {
	if device == nil || device.handle == nil {
		return nil
	}
	handle := device.lib.bindings.maDeviceGetContext(device.handle)
	if handle == nil {
		return nil
	}
	return &Context{lib: device.lib, handle: handle, owned: false}
}

func (ctx *Context) Log() *Log {
	if ctx == nil || ctx.handle == nil {
		return nil
	}
	return borrowedLog(ctx.lib, ctx.lib.bindings.maContextGetLog(ctx.handle))
}
```

For a capture/duplex device the device-type argument matters; add
`NameFor(t DeviceType)`, `InfoFor(t DeviceType)`, and keep `Name`/`Info` as
playback shortcuts. Record the type alongside the stream configs when creating
the device (`Device.primaryType`).

- [ ] **Step 3: Write tests**

```go
// device_info_test.go
package mago

import "testing"

func TestDeviceStateAndInfo(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	device, err := ctx.NewDevice(DeviceConfig{
		Type:    DeviceTypePlayback,
		Playback: &StreamConfig{DeviceIndex: 0, Channels: 2, SampleRate: 48000, PeriodSizeInFrames: 64},
	})
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer func() { _ = device.Close() }()

	if got := device.State(); got != DeviceStateStopped {
		t.Fatalf("state after init = %v, want stopped", got)
	}

	name, err := device.Name()
	if err != nil {
		t.Fatalf("Name: %v", err)
	}
	if name == "" {
		t.Fatal("expected a non-empty device name from the null backend")
	}

	if _, err := device.Info(); err != nil {
		t.Fatalf("Info: %v", err)
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test . -run TestDeviceStateAndInfo -v`
Expected: PASS.

- [ ] **Step 5: Full test and commit**

Run: `mise run test`
Expected: PASS.

```bash
git add device_supported.go types.go log_supported.go device_info_test.go
git commit -m "feat: expose device state, name, info and log accessors"
```

---

## Task 4: Master volume (linear and dB)

**Files:**
- Modify: `device_supported.go`
- Create: `volume_test.go`

**Interfaces:**
- Consumes: `maDeviceSetMasterVolume`, `maDeviceGetMasterVolume`, `maDeviceSetMasterVolumeDB`, `maDeviceGetMasterVolumeDB`.
- Produces: `SetMasterVolume(float64) error`, `MasterVolume() (float64, error)`, `SetMasterVolumeDB(float64) error`, `MasterVolumeDB() (float64, error)`.

- [ ] **Step 1: Write the failing test**

```go
// volume_test.go
package mago

import (
	"math"
	"testing"
)

func TestMasterVolumeRoundTrip(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	device, err := ctx.NewDevice(DeviceConfig{
		Type:     DeviceTypePlayback,
		Playback: &StreamConfig{DeviceIndex: 0, Channels: 2, SampleRate: 48000, PeriodSizeInFrames: 64},
	})
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer func() { _ = device.Close() }()

	if err := device.SetMasterVolume(0.5); err != nil {
		t.Fatalf("SetMasterVolume: %v", err)
	}
	got, err := device.MasterVolume()
	if err != nil {
		t.Fatalf("MasterVolume: %v", err)
	}
	if math.Abs(got-0.5) > 0.01 {
		t.Fatalf("MasterVolume = %v, want ~0.5", got)
	}

	if err := device.SetMasterVolumeDB(-6); err != nil {
		t.Fatalf("SetMasterVolumeDB: %v", err)
	}
	if _, err := device.MasterVolumeDB(); err != nil {
		t.Fatalf("MasterVolumeDB: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestMasterVolumeRoundTrip -v`
Expected: FAIL - `device.SetMasterVolume undefined`.

- [ ] **Step 3: Implement**

```go
func (device *Device) SetMasterVolume(volume float64) error {
	if device == nil || device.handle == nil {
		return fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return err
	}
	return device.lib.resultError("ma_device_set_master_volume",
		device.lib.bindings.maDeviceSetMasterVolume(device.handle, float32(volume)))
}

func (device *Device) MasterVolume() (float64, error) {
	if device == nil || device.handle == nil {
		return 0, fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var volume float32
	result := device.lib.bindings.maDeviceGetMasterVolume(device.handle, &volume)
	if result != Success {
		return 0, device.lib.resultError("ma_device_get_master_volume", result)
	}
	return float64(volume), nil
}
```

`SetMasterVolumeDB`/`MasterVolumeDB` mirror these with the `*DB` bindings.

- [ ] **Step 4: Run the test**

Run: `go test . -run TestMasterVolumeRoundTrip -v`
Expected: PASS.

- [ ] **Step 5: Full test and commit**

Run: `mise run test`
Expected: PASS.

```bash
git add device_supported.go volume_test.go
git commit -m "feat: expose device master volume in linear and dB units"
```

---

## Task 5: Context config and log

**Files:**
- Modify: `context_supported.go`
- Modify: `unsupported.go`
- Create: `context_config_test.go`

**Interfaces:**
- Consumes: `magoContextConfigInit`, `magoContextConfigSetLog`, `magoAlloc(magoObjectContextConfig)`, `maContextInit`, `maContextGetDeviceInfo`, `magoContextConfig` free.
- Produces: `func (lib *Library) NewContextWithLog(log *Log, backends ...Backend) (*Context, error)`.
- Produces: `func (ctx *Context) DeviceInfo(t DeviceType, deviceID unsafe.Pointer) (DeviceInfo, error)`.

- [ ] **Step 1: Write the failing test**

```go
// context_config_test.go
package mago

import "testing"

func TestContextAttachesLog(t *testing.T) {
	lib := newNullLibrary(t)

	log, err := lib.NewLog()
	if err != nil {
		t.Fatalf("NewLog: %v", err)
	}
	defer func() { _ = log.Close() }()

	ctx, err := lib.NewContextWithLog(log, BackendNull)
	if err != nil {
		t.Fatalf("NewContextWithLog: %v", err)
	}
	defer func() { _ = ctx.Close() }()

	if ctx.Log() == nil {
		t.Fatal("expected the context to report the attached log")
	}
}

func TestContextDeviceInfoForDefaultDevice(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	if _, err := ctx.DeviceInfo(DeviceTypePlayback, 0); err != nil {
		t.Fatalf("DeviceInfo: %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run 'TestContextAttachesLog|TestContextDeviceInfo' -v`
Expected: FAIL - `NewContextWithLog` undefined.

- [ ] **Step 3: Implement**

```go
func (lib *Library) NewContextWithLog(log *Log, backends ...Backend) (*Context, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	config := lib.bindings.magoAlloc(magoObjectContextConfig)
	if config == nil {
		return nil, fmt.Errorf("mago: allocate context config: out of memory")
	}
	defer lib.bindings.magoFree(config)

	lib.bindings.magoContextConfigInit(config)
	if log != nil {
		lib.bindings.magoContextConfigSetLog(config, log.handle)
	}
	return lib.newContextWithConfig(config, backends...)
}

// newContextWithConfig factors out the allocation + ma_context_init path used by
// NewContext and NewContextWithLog.
func (lib *Library) newContextWithConfig(config unsafe.Pointer, backends ...Backend) (*Context, error) {
	handle := (*contextHandle)(lib.bindings.magoAlloc(magoObjectContext))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate context: out of memory")
	}

	var result Result
	if len(backends) == 0 {
		result = lib.bindings.maContextInit(nil, 0, config, handle)
	} else {
		count, err := intToUint32(len(backends))
		if err != nil {
			lib.bindings.magoFree(unsafe.Pointer(handle))
			return nil, fmt.Errorf("mago: %w", err)
		}
		result = lib.bindings.maContextInit(&backends[0], count, config, handle)
		runtime.KeepAlive(backends)
	}
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_context_init", result)
	}
	return &Context{lib: lib, handle: handle, owned: true}, nil
}
```

Change `NewContext` to call `lib.newContextWithConfig(nil, backends...)`.

`Context.DeviceInfo`:

```go
func (ctx *Context) DeviceInfo(t DeviceType, deviceID unsafe.Pointer) (DeviceInfo, error) {
	if ctx == nil || ctx.handle == nil {
		return DeviceInfo{}, fmt.Errorf("mago: nil context")
	}
	if err := ctx.lib.ensureOpen(); err != nil {
		return DeviceInfo{}, err
	}

	native := (*deviceInfoNative)(ctx.lib.bindings.magoAlloc(magoObjectDeviceInfo))
	if native == nil {
		return DeviceInfo{}, fmt.Errorf("mago: allocate device info: out of memory")
	}
	defer ctx.lib.bindings.magoFree(unsafe.Pointer(native))

	if result := ctx.lib.bindings.maContextGetDeviceInfo(ctx.handle, t, deviceID, native); result != Success {
		return DeviceInfo{}, ctx.lib.resultError("ma_context_get_device_info", result)
	}
	return copyDeviceInfo(native), nil
}
```

Callers pass `nil` (unsafe.Pointer) for the default device.

- [ ] **Step 4: Run the tests**

Run: `go test . -run 'TestContextAttachesLog|TestContextDeviceInfo' -v`
Expected: PASS. If the null backend reports no device for the default ID, assert
the returned `*OpError` instead and note it.

- [ ] **Step 5: Add unsupported-platform stubs**

Add `func (*Library) NewContextWithLog(*Log, ...Backend) (*Context, error)` and
`func (*Context) DeviceInfo(DeviceType, unsafe.Pointer) (DeviceInfo, error)` to
`unsupported.go`.

- [ ] **Step 6: Full test, lint, and commit**

Run: `mise run test && mise run lint`
Expected: PASS, 0 lint issues.

```bash
git add context_supported.go unsupported.go context_config_test.go
git commit -m "feat: allow attaching a log to a context and query device info"
```

---

## Phase 1 exit criteria

- Capture, duplex and loopback device types are available through `DeviceConfig`
  / `NewDevice`, with `NewPlaybackDevice` still working.
- Device `State`, `Name`, `Info`, `Log` and `Context` accessors work on the null backend.
- Master volume round-trips in linear and dB units.
- A context can be created with an attached `Log`; `Context.DeviceInfo` returns
  data for a known device.
- `mise run check-cgo`, `mise run test` and `mise run lint` are green.
- `mise run build-lib-all && mise run generate` leaves `git diff --exit-code` clean.
- `bd ready` surfaces Phase 2 (`mago-8a3.3`) once Phase 1 closes.
