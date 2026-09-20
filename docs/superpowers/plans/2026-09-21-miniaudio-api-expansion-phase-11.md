# Phase 11: Engine and Sound Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give mago miniaudio's high-level engine: an `Engine` that owns a node graph, a device and a resource manager, plus `Sound` and `SoundGroup` objects with volume, pan, pitch, fades and 3D spatialization.

**Architecture:** The engine is a node graph (its first member is `ma_node_graph`) that mixes sounds and can be read frame by frame with no device attached, which is what makes it testable on the null backend. `ma_sound_config` is large and unstable, so mago only calls the convenience initialisers (`ma_sound_init_from_file`, `_from_data_source`, `_init_copy`, `ma_sound_group_init`), which build that config internally. Every `ma_sound_group_*` accessor is a one-line forwarder to its `ma_sound_*` twin, so only the `ma_sound_*` side is bound and a single `soundCommon` serves both Go types.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (Phase 11)
**Deviations:** `docs/superpowers/specs/2026-09-20-miniaudio-deviation-register.md`
**Predecessors:** `.../phase-0.md` … `.../phase-10.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** Phase 11 adds exactly two things to the bridge: `mago_alloc` cases for
  `ma_engine` and `ma_sound`, and eight category 4 shims for `ma_vec3f` getters.
- **Safe-slice APIs.** Public read methods take Go slices; examples must not import `unsafe`.
- **One native rebuild for the phase** (Task 1).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

- `ma_engine`'s first member is `ma_node_graph`, so an `ma_engine*` is also a valid
  `ma_node_graph*`. This lets `Engine` hand out a borrowed `*NodeGraph` from Phase 10
  without any new code.
- `ma_engine_config.noDevice` skips device creation, and `ma_engine_read_pcm_frames`
  can then be called by hand. Such an engine still requires non-zero `channels` and
  `sampleRate`, or `ma_engine_init` returns `MA_INVALID_ARGS` and a context is never
  touched.
- `ma_engine_config_init()` sets `listenerCount = 1`, `monoExpansionMode = duplicate`,
  `resourceManagerResampling` to a linear resampler with `format = unknown`, and
  `pitchResampling` to a linear resampler with `format = f32` and `linear.lpfOrder = 0`.
  `ma_resampler_config_init` gives `linear.lpfOrder = 4`, which is what this package's
  `DefaultResamplerConfig` already returns, so the whole config can be mirrored in Go
  with no shim.
- `ma_engine_init` creates a resource manager with the default VFS when the config does
  not supply one, so `ma_sound_init_from_file` works with no Phase 12 work.
- Every `ma_sound_group_*` accessor (`uninit`, `start`, `stop`, every setter and getter)
  forwards to the identical `ma_sound_*` function. Only `ma_sound_group_init` and
  `ma_sound_group_init_ex` are distinct. `ma_sound_group` is a typedef of `ma_sound`.
- `ma_sound_init_from_file`, `ma_sound_init_from_data_source` and `ma_sound_init_copy`
  build their `ma_sound_config` from `ma_sound_config_init_2(pEngine)` internally, so Go
  never needs to mirror that struct.
- `ma_sound_end_proc` is `void (*)(void* pUserData, ma_sound*)`. It carries user data, so
  `purego.NewCallback` is enough and no bridge trampoline is needed.
- `ma_vec3f` getters return a struct by value. `purego.RegisterLibFunc` panics for struct
  returns on anything other than darwin and linux, and mago also targets windows,
  freebsd and netbsd, so these eight getters need category 4 shims.
- `ma_attenuation_model_none` means no distance attenuation and no spatialization, so a
  sound at the origin plays at full volume in every channel while `monoExpansionMode`
  duplicates a mono source across the engine's channels.

### Deliberate scope cuts (recorded for the deviation register)

- The `_in_milliseconds` and `_in_seconds` variants are not bound. Go wrappers taking
  `time.Duration` cover the same ground on top of the frame-based functions, using the
  engine's sample rate exactly as miniaudio does.
- `ma_engine_get_device`, `ma_engine_get_log` and `ma_engine_get_resource_manager` are
  not bound. Handing out borrowed `Device` and `Log` values would let a caller close
  something the engine owns, and the resource manager is Phase 12's subject.
- `ma_sound_config` / `ma_sound_init_ex`, `ma_sound_init_from_file_w` and the deprecated
  `ma_engine_get_time` / `set_time` and `*_config_init` helpers are not bound.
- `ma_engine_config.onProcess` and the engine's `dataCallback` / `notificationCallback`
  hooks are not exposed.

---

## Task 1: Native allocation, vec3f shims, bindings and layout probe (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c`
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

- [ ] **Step 1: Add the two object types and the eight vec3f shims**

`MAGO_OBJECT_ENGINE = 44` and `MAGO_OBJECT_SOUND = 45` with `calloc(1, sizeof(ma_engine))`
and `calloc(1, sizeof(ma_sound))`, plus a documented category 4 block:

```c
static void mago_write_vec3f(ma_vec3f value, float* pOut)
{
    if (pOut != NULL)
    {
        pOut[0] = value.x;
        pOut[1] = value.y;
        pOut[2] = value.z;
    }
}

MAGO_API void mago_engine_listener_get_position(const ma_engine* pEngine, ma_uint32 listenerIndex, float* pOut);
MAGO_API void mago_engine_listener_get_direction(const ma_engine* pEngine, ma_uint32 listenerIndex, float* pOut);
MAGO_API void mago_engine_listener_get_velocity(const ma_engine* pEngine, ma_uint32 listenerIndex, float* pOut);
MAGO_API void mago_engine_listener_get_world_up(const ma_engine* pEngine, ma_uint32 listenerIndex, float* pOut);
MAGO_API void mago_sound_get_position(const ma_sound* pSound, float* pOut);
MAGO_API void mago_sound_get_direction(const ma_sound* pSound, float* pOut);
MAGO_API void mago_sound_get_velocity(const ma_sound* pSound, float* pOut);
MAGO_API void mago_sound_get_direction_to_listener(const ma_sound* pSound, float* pOut);
```

- [ ] **Step 2: Mirror `ma_engine_config` in `types.go`**

Field order and padding must follow the header; the resampler members reuse the existing
`resamplerConfigNative`.

- [ ] **Step 3: Bind the engine, listener and sound surface**

Engine lifecycle and mixing, the listener block, `ma_engine_play_sound`(+`_ex`), the
sound init/uninit trio plus `ma_sound_group_init`, and every non-deprecated `ma_sound_*`
setter, getter and action except the millisecond and second variants.

- [ ] **Step 4: Extend the layout probe and `layout_test.go`**

`sizeof` and `offsetof` for every `ma_engine_config` member, including the two resampler
members, since Go builds that struct and gets its defaults.

- [ ] **Step 5: Rebuild and regenerate**

```bash
mise run build-lib-all
mise run generate
go test -run TestMirroredStructLayouts ./...
go build ./...
```

---

## Task 2: `Engine` and `Listener`

**Files:**
- Create: `engine.go`
- Modify: `nodegraph.go` (borrowed graphs)

- [ ] **Step 1: Let a `NodeGraph` be borrowed**

Add an `owned` flag. `Close` returns an error for a borrowed graph, and a new
unexported `borrowedNodeGraph` constructor builds one for the engine.

- [ ] **Step 2: `Engine`**

```go
func (lib *Library) NewEngine(config EngineConfig) (*Engine, error)
func (e *Engine) NodeGraph() *NodeGraph
func (e *Engine) Endpoint() Node
func (e *Engine) Read(out []float32) (uint64, error)
func (e *Engine) Channels() uint32
func (e *Engine) SampleRate() uint32
func (e *Engine) Start() error
func (e *Engine) Stop() error
func (e *Engine) Volume() float32
func (e *Engine) SetVolume(volume float32) error
func (e *Engine) GainDB() float32
func (e *Engine) SetGainDB(gainDB float32) error
func (e *Engine) Time() uint64
func (e *Engine) SetTime(globalTime uint64) error
func (e *Engine) ListenerCount() uint32
func (e *Engine) ClosestListener(position Vec3) uint32
func (e *Engine) Listener(index uint32) *Listener
func (e *Engine) PlaySound(path string, group *SoundGroup) error
func (e *Engine) Close() error
```

`Read` mirrors `NodeGraph.Read`: it takes a Go slice, checks the channel alignment and
converts `AtEnd` into a normal short read.

- [ ] **Step 3: `Listener`**

A view of `(engine, listenerIndex)` with `Index`, `Position`/`SetPosition`,
`Direction`/`SetDirection`, `Velocity`/`SetVelocity`, `WorldUp`/`SetWorldUp`,
`Cone`/`SetCone`, `Enabled`/`SetEnabled`. Mutators return an error when the engine is
closed, matching the rest of the package; getters return the zero value.

- [ ] **Step 4: Build**

```bash
go build ./...
```

---

## Task 3: `Sound` and `SoundGroup`

**Files:**
- Create: `sound.go`

- [ ] **Step 1: Types**

`SoundFlags` with the `MA_SOUND_FLAG_*` values, `PanMode`, `AttenuationModel` and
`Positioning`.

- [ ] **Step 2: A single shared implementation**

`soundCommon` holds the engine and handle and implements the whole shared surface:
`Start`, `Stop`, `StopWithFade`, the reset trio, volume, pan and pan mode, pitch,
spatialization enable, pinned listener and listener index, position, direction,
velocity, attenuation model, positioning, rolloff, min/max gain, min/max distance,
cone, Doppler and directional attenuation factors, fades, start and stop scheduling,
`IsPlaying`, `TimeInPCMFrames`, `Engine` and `Close`, plus `time.Duration` wrappers
for the millisecond variants.

- [ ] **Step 3: `SoundGroup`**

Embeds `soundCommon`; created with `Engine.NewSoundGroup`, optionally parented to another
group.

- [ ] **Step 4: `Sound`**

Embeds `soundCommon` and adds the data-source side: `Engine.NewSoundFromFile`,
`Engine.NewSoundFromDataSource`, `Sound.NewCopy`, `DataSource`, `SetLooping`/`IsLooping`,
`AtEnd`, `SeekToPCMFrame`/`SeekToDuration`, `DataFormat`, `CursorInPCMFrames`,
`LengthInPCMFrames`, `DurationInPCMFrames` and `SetEndCallback`.

`NewSoundFromDataSource` accepts anything that satisfies `DataSource` and reuses the
Phase 10 rule: sources miniaudio already understands are read natively, and any other Go
source is wrapped in a `CustomDataSource` the sound owns. The engine requires f32 too, so
non-f32 sources are rejected up front.

- [ ] **Step 5: Build and lint**

```bash
go build ./... && mise run lint
```

---

## Task 4: Unsupported-platform stubs

**Files:**
- Modify: `unsupported.go`

Add `Vec3`, `MonoExpansionMode`, `PanMode`, `AttenuationModel`, `Positioning`,
`SoundFlags`, `EngineConfig`, `Engine`, `Listener`, `SoundGroupConfig`, `SoundGroup`,
`SoundConfig`, `Sound` and the `NodeGraph` ownership change, all returning
`errUnsupportedPlatform`.

Verify with the overlay build used in Phase 10: copy the tree, blank every
`*_supported.go`, `loader_*.go`, `embed_*.go` and `zz_generated.bindings.go`, retag
`unsupported.go` for linux, then `go build .` and diff `go doc -all` against the
supported build.

---

## Task 5: Tests

**Files:**
- Create: `engine_test.go`
- Create: `sound_test.go`

- [ ] **Step 1: Device-less engine**

Config with `NoDevice`, channels and sample rate; assert `Channels`, `SampleRate`,
`ListenerCount`, that `NodeGraph()` is borrowed and refuses to close, and that `Read`
returns silence before any sound exists.

- [ ] **Step 2: Sound playback**

A waveform `CustomDataSource` in a sound, read through the engine, and assert the peak
matches the source amplitude.

- [ ] **Step 3: Group gain cascade**

A sound in a group inside another group, with a distinct volume at each of the three
levels, and assert the product is what comes out.

- [ ] **Step 4: Pan**

A mono source panned hard left and hard right, asserting the channel imbalance.

- [ ] **Step 5: Attenuation and listener updates**

An inverse-attenuated sound with min/max distance, checked at several distances, plus a
listener move that changes the level and a `Listener.Position` read-back.

- [ ] **Step 6: Scheduling and fades**

`SetStartTimeInDuration`, `SetStopTimeInDuration` and `SetFadeInDuration` verified
against the engine clock.

- [ ] **Step 7: End callback**

A finite data source with `SetEndCallback`, asserted through a buffered channel and a
timeout rather than a sleep.

- [ ] **Step 8: Sound from file**

Encode a tone to a WAV in `t.TempDir()`, load it with `NewSoundFromFile`, and assert
playback. Also exercise `Engine.PlaySound`.

- [ ] **Step 9: A real null-backend device**

An engine on a null-backend context that starts and stops, proving the device path.

- [ ] **Step 10: Lifecycle**

Double `Close`, use after `Close`, and closing a sound after its engine.

```bash
mise run test
```

---

## Task 6: Example, README and docs

**Files:**
- Create: `examples/engine/main.go`
- Modify: `README.md`
- Modify: `AGENTS.md` if a new convention was introduced

The example builds a device-less engine, loads a tone from an in-memory data source,
routes it through a group, moves a listener to show attenuation, and prints the level at
each step. It needs no audio device.

---

## Task 7: Verification

- [ ] `mise run test`
- [ ] `mise run lint` (includes `check-cgo` and `govulncheck`)
- [ ] `mise run build-lib-all && mise run generate` leaves the generated files unchanged
- [ ] Dead-binding audit: no new unused bindings
- [ ] Close `mago-8a3.12.1`, `mago-8a3.12.2`, `mago-8a3.12.3` and `mago-8a3.12`
- [ ] Update the deviation register with the new findings and file beads
- [ ] Push and open a PR
