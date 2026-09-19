# miniaudio API Surface Expansion

**Status:** Proposed
**Date:** 2026-09-19
**Vendored miniaudio version:** `0.11.25`
**Primary source:** `miniaudio.h` v0.11.25 (downloaded from
`https://raw.githubusercontent.com/mackron/miniaudio/0.11.25/miniaudio.h`)

## 1. Context and motivation

`mago` currently exposes only the minimum needed to emit audio and enumerate
devices:

| Area | Exposed today |
| --- | --- |
| Library / version | `Open`, `Library.Version`, `VersionString`, `ResultDescription`, strict version check |
| Context | `NewContext(backends...)`, `Devices()` (playback + capture enumeration), `Close` |
| Playback device | `NewPlaybackDevice`, `Start`, `Stop`, `Close`, data + notification callbacks (F32) |
| PCM conversion | none (only F32 is delivered) |
| Everything else | unavailable |

miniaudio is far broader. It ships a low-level API (devices, data conversion,
buffers, decoders, encoders, filters, generation) and a high-level API (engine,
sounds, node graph, resource manager, spatialization). The Go wrapper should
expose the parts that add value over the Go standard library, while explicitly
declining the parts that duplicate idiomatic Go.

This document is the program-level spec. Each phase below becomes its own
implementation plan (`docs/superpowers/plans/`) when that phase starts; the
phases are independent enough to be planned and shipped one at a time, and
each produces working, testable software on its own.

## 2. Research summary: what miniaudio actually provides

The header declares 1,187 unique `MA_API` functions. Counts below are per
module prefix (unique function names, v0.11.25).

| Module | Fns | Notes |
| --- | ---: | --- |
| `ma_device_*` | 25 | init/start/stop, state, name, info, master volume, job thread |
| `ma_context_*` | 9 | init, enumerate, get device info, log, sizeof |
| `ma_engine_*` | 44 | high-level engine, listener/3D, gain, time |
| `ma_sound_*` | 141 | sounds and sound groups, spatialization, fades, pitch/pan |
| `ma_resource_manager_*` | 64 | async loading, registration, data source/buffer/stream |
| `ma_decoder_*` | 16 | file/memory/vfs init, read, seek, length, data format |
| `ma_encoder_*` | 10 | file/vfs init, write |
| `ma_resampler_*` | 13 | linear + custom algorithm, rate/ratio, latency |
| `ma_linear_resampler_*` | 13 | dedicated linear resampler |
| `ma_data_converter_*` | 16 | format + channel + rate in one pipeline |
| `ma_channel_converter_*` | 8 | channel mixing, custom weights |
| `ma_biquad_*` | 13 | second-order biquad + node |
| `ma_lpf_*` / `ma_lpf1_*` / `ma_lpf2_*` | 13 / 9 / 9 | higher-order, 1st-order, 2nd-order LPFs |
| `ma_hpf_*` / `ma_bpf_*` | 12 / 12 | HPF / BPF |
| `ma_notch2_*` / `ma_peak2_*` / `ma_loshelf2_*` / `ma_hishelf2_*` | 7 each | stand-alone second-order filters |
| `ma_delay_*` | 19 | delay line + node, wet/dry/decay |
| `ma_waveform_*` | 9 | sine/square/triangle/sawtooth |
| `ma_noise_*` | 9 | white/pink/brownian |
| `ma_audio_buffer_*` | 25 | in-memory PCM data source + ref variants |
| `ma_rb_*` / `ma_pcm_rb_*` | 17 / 21 | generic and PCM ring buffers |
| `ma_data_source_*` | 30 | data source base + `ma_data_source_node` |
| `ma_node_*` / `ma_node_graph_*` | 32 / 9 | node graph, buses, state, volumes |
| `ma_pcm_*` | 57 | per-format conversion + interleave/deinterleave |
| `ma_channel_*` | 20 | channel maps |
| `ma_convert_frames*` | 3 | full format/channel/rate conversion |
| `ma_vfs_*` | 18 | virtual file system + default VFS |
| `ma_log_*` | 9 | logging callbacks |
| `ma_fence_*`, `ma_async_notification_*` | 5 / 7 | async synchronization |

### 2.1 Findings that change the plan

These contradict common assumptions and were verified directly in the header:

- **There is no reverb API in 0.11.25.** `ma_reverb_*` and `ma_reverb_node`
  do not exist (grep count 0). Reverb is not part of any phase.
- **Stand-alone second-order notch/peak/shelf filters use a `2` suffix:**
  `ma_notch2`, `ma_peak2`, `ma_loshelf2`, `ma_hishelf2`. There is no
  `ma_notch_init` / `ma_peak_init` (only the `*_node` variants without the `2`).
- **There are no `ma_device_read` / `ma_device_write` functions.** Data only
  moves through the device callback.
- **`ma_device` job-thread functions exist** (`ma_device_job_thread_*`) but are
  plumbing for the resource manager; not a target.
- **Loopback device type is WASAPI-only.**
- **Decoders are on by default:** WAV, FLAC and MP3 unless `MA_NO_*` build
  flags disable them. `MA_NO_GENERATION` gates waveform/noise.
- **The engine is built on the resource manager**, which is built on decoders
  and VFS. Exposing the engine implicitly pulls in that stack; exposing VFS
  itself is still optional.
- **Structs are transparent and version-unstable.** miniaudio objects are
  caller-allocated and must never be copied; ABI is not guaranteed between
  releases. Go mirrors only the layouts it must (see section 3) and validates
  them against the vendored header.

## 3. Architecture: minimal C, zero CGO

**Hard rule: there is no CGO anywhere in this project.** No `import "C"`, no
cgo build tags, no `//export`, no `cgo_*.go` files. `CGO_ENABLED=0` always. The
native library is a prebuilt runtime artifact loaded with purego; the Go
toolchain never compiles or links C.

**Hard rule: the C bridge is the absolute minimum needed to drive miniaudio
through purego.** Prefer binding exported `ma_*` symbols directly. A `mago_*`
shim exists only when purego cannot express the ABI, and every new shim must
justify why a direct binding or a Go-side implementation is impossible.

### 3.1 What C is allowed to do (exhaustive)

1. **Allocate raw memory.** Go cannot call `malloc` without cgo, and purego
   cannot bind libc portably. A generic `mago_malloc(size_t)` / `mago_free(void*)`
   pair is the entire allocation surface. Go obtains sizes from miniaudio's
   `*_sizeof` functions where they exist (`ma_context_sizeof`), otherwise from a
   one-line `mago_sizeof_<type>()` (e.g. for `ma_device`).
2. **Build a by-value, large config that Go must not mirror.** miniaudio's
   `*_config_init` functions return config structs by value, and the structs are
   large, backend-specific and version-unstable (`ma_device_config` even embeds
   `ma_resampler_config` and per-backend sub-structs). One setter shim per config
   family: `mago_device_config_playback(...scalars..., ma_device_config* pOut)`
   calls `ma_device_config_init`, assigns fields from the scalar arguments, and
   returns. No branching, no validation beyond miniaudio's own.
3. **Bridge a callback whose C signature carries no user data.** miniaudio's
   device data and notification callbacks are `void (*)(ma_device*, ...)` with
   no `void* pUserData`; user data travels through `pDevice->pUserData`. Reading
   it from Go would require mirroring the enormous, version-unstable `ma_device`.
   Two static trampolines plus a tiny bridge struct are the minimum that avoids
   it. Callbacks that *do* carry `pUserData` (e.g.
   `ma_enum_devices_callback_proc`, `ma_log` callbacks) use `purego.NewCallback`
   directly with no C.

Anything already reachable through a direct `ma_*` binding must not get a
wrapper. In particular, allocation wrappers that also perform init/uninit or
copy data are redundant: `mago_context_init_default`,
`mago_context_init_with_backends`, `mago_context_uninit_free`,
`mago_context_get_devices` and `mago_context_free_device_infos` are Phase 0
removal targets. Go calls the `ma_*` symbols themselves. `mago_device_uninit_free`
is retained because it also frees the callback bridge (3.1 item 3), which Go
does not track.

### 3.2 What Go does instead

- **Direct symbol binding.** The generator registers `ma_*` functions directly,
  not just `mago_*` wrappers.
- **Struct layout mirroring.** Where purego passes a pointer, Go mirrors just
  enough layout to read the fields it needs. Enumeration uses
  `ma_context_enumerate_devices` (which carries `pUserData`) and a prefix mirror
  of `ma_device_info`, so no stride or full-struct mirror is required. Large
  by-value configs are handled by the setter shims in 3.1, never mirrored.
- **Allocation via `sizeof` where possible.** Use `ma_context_sizeof` and any
  other `*_sizeof` functions to allocate from Go; fall back to 3.1 item 1 only
  when none exists.
- **Callbacks via the token registry.** `purego.NewCallback` plus a
  monotonically increasing token passed as `userData`, with state in a
  `sync.Map`. Proven in `device_supported.go`; required for log, custom data
  source, custom decoder and custom resampler callbacks.
- **Opaque Go types** wrap raw pointers; no raw binding leaks into the public API.
- **Every symbol through the generator.** Add each symbol to `functions` in
  `internal/gen/bindings/main.go`; never hand-edit `zz_generated.bindings.go`.

### 3.3 Keeping Go-side layouts honest

Because Go mirrors layouts, drift is the main correctness risk. Mitigations,
best first:

1. Prefer miniaudio's `*_sizeof` / `*_config_init` APIs whenever they remove the
   need to mirror a layout at all.
2. A test-time probe compiles a tiny C program against the vendored
   `miniaudio.h` (using the same zig/`buildlib` path as the existing tests) and
   asserts the Go mirror's size and field offsets match, on every OS in the CI
   test matrix. A shipped C shim is never used for validation.
3. Keep mirrored structs small and local to the file that uses them.

### 3.4 The heavyweight constraint: shipping new symbols

The native bridge is pre-compiled and embedded for 7 targets, and the embed
files are committed. Therefore **any new bridge symbol requires**:

```bash
mise run build-lib-all   # 7 targets: zig cc + osxcross for darwin
mise run generate        # rebind + regenerate embed_*.go
```

and committing the regenerated `embed_*.go` files, or CI's `verify-embeds`
job fails (`git diff --exit-code`). This makes each phase a deliberate,
batched addition: group all symbols for a phase into one bridge revision to
avoid repeated multi-platform rebuilds.

### 3.5 Cross-cutting requirements

- **No CGO.** `CGO_ENABLED=0`; runtime loading only. Never add a cgo file.
- **C bridge stays minimal.** Every new `mago_*` shim must be justified against
  section 3.1; new Go-side logic is always preferred.
- **Strict version match** stays; a phase may not relax it.
- **Opaque handles vs. mirrored value structs:** large, stateful,
  version-unstable objects (`decoder`, `encoder`, `resampler`,
  `data_converter`, `engine`, `sound`, `node_graph`, filters, buffers) are
  opaque handles. Value structs that miniaudio exposes as configs
  (`ma_waveform_config`, `ma_noise_config`, etc.) are mirrored in Go and
  validated per section 3.3.
- **Testing:** all audio tests use `BackendNull` and
  `internal/testlib.BuildRuntimeLibrary`; generators/buildlib unit tests use
  the `compilerOverride` hook and never invoke zig/Docker.
- **Error mapping:** every bridge call that returns `ma_result` maps through
  `lib.resultError` into `OpError`.
- **Docs and examples:** each phase ships at least one runnable example or
  test that exercises the new surface on the null backend.

## 4. Non-goals and deliberate omissions

These are intentionally *not* exposed, because idiomatic Go is a better fit.
They are recorded so future agents do not re-litigate them:

- **`ma_vfs_*` and the default VFS** - use `io/fs`, `os` and `io.Reader`.
- **Resource manager async loading / job queue** - use goroutines and channels.
  (The engine still uses it internally; we just do not surface it as the
  primary loading API.)
- **Raw thread primitives** (`ma_mutex`, `ma_event`, `ma_semaphore`) - use `sync`.
- **`ma_device_job_thread_*`** - internal plumbing.
- **Resource manager registration APIs** (`register_file`, `register_encoded_data`,
  async callbacks) - deferred to Phase 12, and only if real demand appears.
- **Custom resamplers / custom decoders** - advanced extension points; deferred
  behind a decision gate (Phases 3 and 7 respectively).

## 5. Phased plan

Dependency ordering is `0 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11`, with
Phase 12 independent. Phases 4-6 can proceed in parallel once Phase 3 lands.
Every phase ends with: full test suite green, examples build, native libs
rebuilt for all targets, embeds regenerated and committed.

### Phase 0 - Minimal-C bridge foundation

**Goal:** lock in the minimal-C / zero-CGO discipline and the safety net that
makes Go-side struct mirroring reliable. Every later phase depends on this.

**Scope**
- **Audit and shrink the C bridge to the section 3.1 minimum.** Remove the
  redundant allocation/init wrappers (`mago_context_init_default`,
  `mago_context_init_with_backends`, `mago_context_uninit_free`,
  `mago_context_get_devices`, `mago_context_free_device_infos`,
  `mago_device_uninit_free`) and call the `ma_*` symbols directly instead.
  Keep, and document as justified:
  - `mago_malloc` / `mago_free` and a `mago_sizeof_device()` (no miniaudio
    equivalent for `ma_device`);
  - the `ma_device_config` setter shim (by-value `ma_device_config_init`, large
    backend-specific struct);
  - the device data/notification callback trampolines + bridge struct (the C
    callback signature carries no `pUserData`).
- **Move enumeration to direct binding.** Call `ma_context_enumerate_devices`
  with a `purego.NewCallback` and a prefix mirror of `ma_device_info`; drop the
  C copying loop.
- **Direct purego binding.** The generator registers `ma_*` symbols already;
  formalize it and stop adding wrappers for anything reachable directly.
- **Layout validation.** A test-time probe compiles a tiny C program against the
  vendored `miniaudio.h` (via the existing `testlib`/`buildlib` zig path) and
  asserts the Go mirrors' sizes and field offsets match, per platform.
- **Callback policy.** `purego.NewCallback` + token registry for callbacks that
  carry `pUserData`; C trampolines only for those that do not.
- `ma_log` via direct binding: `ma_log_init`, `ma_log_uninit`,
  `ma_log_register_callback`, `ma_log_unregister_callback`, `ma_log_post`,
  `ma_log_level_to_string`, wired to a Go logging surface. No native rebuild
  needed because these symbols are already exported.
- **Zero-CGO guard.** A test scans the repository for `import "C"`, `//export`
  and cgo build constraints and fails if any appear.
- Test harness for opaque object lifecycles (create -> use -> uninit -> use-after-uninit).
- Document the hard rules (no CGO; minimal C; justify every shim) in `AGENTS.md`
  and in a header comment on `native/miniaudio_bridge.c`.

**Deliverables:** a smaller `native/miniaudio_bridge.c` reduced to the three
justified categories; direct `ma_*` binding; a per-platform layout probe test;
log API (`mago.LogCallback`, log access from the library/context); a zero-CGO
guard; lifecycle tests; docs.

**Depends on:** nothing. **Acceptance:** no CGO anywhere; device behavior
unchanged; log callbacks delivered to Go on the null backend; the C bridge
contains only the three justified categories; the layout probe passes and fails
loudly when a mirrored struct diverges.

### Phase 1 - Low-level device and context completeness

**Goal:** round out the foundation the wrapper already covers.

**Scope (verified symbols)**
- Device types: playback, capture, duplex, loopback (`ma_device_type`,
  loopback WASAPI-only).
- `ma_device_config_init`, `ma_device_init`, `ma_device_init_ex`,
  `ma_device_uninit`, `ma_device_start`, `ma_device_stop`.
- `ma_device_get_state`, `ma_device_get_name`, `ma_device_get_info`,
  `ma_device_get_context`, `ma_device_get_log`.
- `ma_device_set_master_volume`, `ma_device_get_master_volume`,
  `ma_device_set_master_volume_db`, `ma_device_get_master_volume_db`.
- `ma_context_config_init`, `ma_context_get_device_info`,
  `ma_context_enumerate_devices`, `ma_context_sizeof`, `ma_context_get_log`.
- Capture (input) data flow through the same callback trampoline.

**Deliverables:** `CaptureDevice`/duplex support, device type in
`PlaybackDeviceConfig` (or a new `DeviceConfig`), master-volume/state/name
accessors, richer `DeviceInfo`.

**Depends on:** Phase 0. **Acceptance:** null-backend capture and duplex
tests; master volume round-trips; device state transitions observed.

### Phase 2 - PCM format conversion and channel mapping

**Goal:** remove the "F32 only" limitation and give callers conversion tools.

**Scope**
- `ma_pcm_convert`, `ma_convert_pcm_frames_format`, `ma_convert_frames`,
  `ma_convert_frames_ex`.
- Per-format converters `ma_pcm_<src>_to_<dst>` and
  `ma_pcm_interleave_*` / `ma_pcm_deinterleave_*` for u8/s16/s24/s32/f32.
- Channel maps: `ma_channel_map_init_standard`, `ma_channel_map_init_blank`,
  `ma_channel_map_copy`, `ma_channel_map_copy_or_default`,
  `ma_channel_map_get_channel`, `ma_channel_map_to_string`.
- `ma_channel_converter_*` (including custom weights via `ma_channel_mix_mode`).
- Extend `Format` and dithering (`ma_dither_mode`).

**Deliverables:** Go helpers operating on `[]float32` / raw byte slices,
channel-map type, `ChannelConverter` opaque object.

**Depends on:** Phase 0. **Acceptance:** round-trip conversion tests for every
format pair; channel down/up-mix tests against known vectors.

### Phase 3 - Resampling

**Goal:** sample-rate conversion and a one-stop data converter.

**Scope**
- `ma_resampler_config_init`, `ma_resampler_init`, `ma_resampler_uninit`,
  `ma_resampler_process_pcm_frames`, `ma_resampler_set_rate`,
  `ma_resampler_set_rate_ratio`, `ma_resampler_reset`,
  `ma_resampler_get_required_input_frame_count`,
  `ma_resampler_get_expected_output_frame_count`, `ma_resampler_get_heap_size`,
  `ma_resampler_init_preallocated`.
- `ma_linear_resampler_*` equivalents.
- `ma_data_converter_*` (format + channels + rate in one pipeline), including
  `get_input_channel_map` / `get_output_channel_map`.
- **Decision gate:** custom resamplers (`ma_resample_algorithm_custom`) and
  `ma_resampler_*_preallocated` / `get_heap_size` are only exposed if a
  consumer needs caller-managed memory; otherwise document as omitted.

**Deliverables:** `Resampler`, `LinearResampler`, `DataConverter` types.

**Depends on:** Phases 0, 2. **Acceptance:** 44.1k→48k and 48k→44.1k
null-backend round trips; latency and frame-count estimates match observed output.

### Phase 4 - Buffers and ring buffers

**Goal:** streaming and fixed in-memory buffer primitives.

**Scope**
- `ma_audio_buffer_config_init`, `ma_audio_buffer_init`,
  `ma_audio_buffer_init_copy`, `ma_audio_buffer_uninit`,
  `ma_audio_buffer_map`/`unmap`, `ma_audio_buffer_seek_to_pcm_frame`,
  `ma_audio_buffer_get_available_frames`, `ma_audio_buffer_get_cursor_in_pcm_frames`,
  `ma_audio_buffer_get_length_in_pcm_frames`,
  `ma_audio_buffer_alloc_and_init`, `ma_audio_buffer_uninit_and_free`.
- `ma_audio_buffer_ref_*` (non-owning view).
- `ma_rb_*` and `ma_pcm_rb_*` ring buffers (acquire/commit read/write,
  subbuffer accessors, seek, reset).
- Both buffer types are data sources, so they compose with Phase 9/10.

**Deliverables:** `AudioBuffer`, `AudioBufferRef`, `RingBuffer`, `PCMRingBuffer`.

**Depends on:** Phases 0, 2. **Acceptance:** underrun/overrun behavior and
acquire/commit semantics tested; buffer length/cursor consistency.

### Phase 5 - Waveform and noise generation

**Goal:** signal generation for tests, tools and synthesis.

**Scope**
- `ma_waveform_config_init`, `ma_waveform_init`, `ma_waveform_uninit`,
  `ma_waveform_read_pcm_frames`, `ma_waveform_set_type`,
  `ma_waveform_set_frequency`, `ma_waveform_set_amplitude`,
  `ma_waveform_set_sample_rate`, `ma_waveform_seek_to_pcm_frame`.
- `ma_noise_config_init`, `ma_noise_init`, `ma_noise_uninit`,
  `ma_noise_read_pcm_frames`, `ma_noise_set_type`, `ma_noise_set_amplitude`,
  `ma_noise_set_seed`.
- Types: `ma_waveform_type_{sine,square,triangle,sawtooth}`,
  `ma_noise_type_{white,pink,brownian}`.

**Deliverables:** `Waveform`, `Noise` data sources. Note the `examples/tones`
demo currently hand-rolls a sine generator and is a natural migration target.

**Depends on:** Phases 0, 2, 9 (as data sources). **Acceptance:** known
waveform sample vectors; noise reproducibility via `set_seed`.

### Phase 6 - Filtering and effects

**Goal:** expose the DSP filter set.

**Scope**
- Stand-alone filters: `ma_biquad_*`, `ma_lpf_*`, `ma_lpf1_*`, `ma_lpf2_*`,
  `ma_hpf_*`, `ma_bpf_*`, `ma_notch2_*`, `ma_peak2_*`, `ma_loshelf2_*`,
  `ma_hishelf2_*` (init, uninit, reinit, clear_cache, process_pcm_frames,
  get_latency, get_heap_size, init_preallocated).
- `ma_delay_*`: init, uninit, config_init, process_pcm_frames,
  set/get wet, dry, decay.
- Node variants (`ma_*_node_*`, `ma_delay_node_*`) are deferred to Phase 10,
  where they attach to the graph.

**Deliverables:** `Biquad`, `LowPassFilter`(+1st/2nd order), `HighPassFilter`,
`BandPassFilter`, `NotchFilter`, `PeakFilter`, `LowShelfFilter`, `HighShelfFilter`,
`Delay`. **No reverb.**

**Depends on:** Phases 0, 2. **Acceptance:** frequency-response sanity checks
(attenuation at/away from cutoff); delay wet/dry/feedback measured.

### Phase 7 - Decoding

**Goal:** decode real audio formats instead of pure-Go WAV only.

**Scope**
- `ma_decoder_config_init`, `ma_decoder_config_init_copy`/`_default`,
  `ma_decoder_init`, `ma_decoder_init_file`, `ma_decoder_init_memory`,
  `ma_decoder_init_vfs`, `ma_decoder_uninit`, `ma_decoder_read_pcm_frames`,
  `ma_decoder_seek_to_pcm_frame`, `ma_decoder_get_length_in_pcm_frames`,
  `ma_decoder_get_cursor_in_pcm_frames`, `ma_decoder_get_available_frames`,
  `ma_decoder_get_data_format`.
- Formats available by default: **WAV, FLAC, MP3**.
- Integrate with `audio`: allow `Engine.Load` to use `ma_decoder` for non-WAV
  inputs (the decoder is itself a data source, so it can be played directly).
- **Decision gate:** custom decoders (`ma_decoding_backend_vtable`) are
  deferred until a concrete format (e.g. Opus) is required.

**Deliverables:** `Decoder` type; `audio` package format support; example
playing FLAC/MP3 via the null backend.

**Depends on:** Phases 0, 2. **Acceptance:** decode a fixture of each format;
seek/length/cursor correct; integration test through `audio`.

### Phase 8 - Encoding

**Goal:** write audio out.

**Scope**
- `ma_encoder_config_init`, `ma_encoder_init`, `ma_encoder_init_file`,
  `ma_encoder_init_vfs`, `ma_encoder_uninit`, `ma_encoder_write_pcm_frames`.
- Default encoder: **WAV**.

**Deliverables:** `Encoder` type; `audio`/`Clip` export-to-WAV path; example.

**Depends on:** Phases 0, 2. **Acceptance:** encode known PCM, re-decode with
Phase 7 and compare bytes/frames.

### Phase 9 - Data sources and custom data sources

**Goal:** a uniform pull interface that composes everywhere.

**Scope**
- Base: `ma_data_source_config_init`, `ma_data_source_init`,
  `ma_data_source_uninit`, `ma_data_source_read_pcm_frames`,
  `ma_data_source_seek_to_pcm_frame`, `ma_data_source_get_data_format`,
  `ma_data_source_get_length_in_pcm_frames`, `ma_data_source_set_looping`,
  `ma_data_source_set_range_in_pcm_frames`, `ma_data_source_set_loop_point_in_pcm_frames`,
  `ma_data_source_set_current`/`get_current`, `set_next`/`get_next` (chaining).
- **Custom data sources:** implement `ma_data_source_base` + vtable from Go via
  the callback token registry, so Go-generated PCM participates in the graph.
- `ma_data_source_node_*` (init, uninit, set_looping, config_init).

**Deliverables:** `DataSource` interface, adapters for Decoder/AudioBuffer/
Waveform/Noise, `NewDataSourceNode`, docs on the vtable contract.

**Depends on:** Phases 0, 2. **Acceptance:** a Go-implemented data source
streams to a null device; chaining and looping verified.

### Phase 10 - Node graph routing

**Goal:** graph-based mixing, routing and per-bus control.

**Scope**
- `ma_node_graph_config_init`, `ma_node_graph_init`, `ma_node_graph_uninit`,
  `ma_node_graph_read_pcm_frames`, `ma_node_graph_get_endpoint`,
  `ma_node_graph_set_time`, `ma_node_graph_get_node_count` (verify exact names
  at implementation time).
- `ma_node_*`: config_init, init, init_preallocated, uninit,
  attach_output_bus, detach_output_bus, detach_all_output_buses,
  set/get_output_bus_volume, get_node_graph, get_state, set_state,
  get/set state_by_time.
- Node types: `ma_data_source_node`, `ma_splitter_node`, `ma_biquad_node`,
  `ma_lpf_node`, `ma_lpf1_node`/`ma_lpf2_node` if present, `ma_hpf_node`,
  `ma_bpf_node`, `ma_notch_node`, `ma_peak_node`, `ma_loshelf_node`,
  `ma_hishelf_node`, `ma_delay_node`.
- `ma_engine_node` is internal to the engine and is not exposed directly.

**Deliverables:** `NodeGraph`, `Node` interface, concrete node wrappers,
example routing a source through a filter into the graph endpoint read by the
device callback.

**Depends on:** Phases 0, 2, 4, 6, 9. **Acceptance:** graph with source →
filter → endpoint produces expected output; bus volume and splitter routing
verified; thread-safety rules documented (audio thread vs. control thread).

### Phase 11 - High-level engine and sound

**Goal:** the high-level API: sounds, groups, fades and 3D spatialization.

**Scope**
- `ma_engine_config_init`, `ma_engine_init`, `ma_engine_uninit`,
  `ma_engine_start`, `ma_engine_stop`, `ma_engine_read_pcm_frames`,
  `ma_engine_get_device`, `ma_engine_get_node_graph`,
  `ma_engine_get_resource_manager`, `ma_engine_get_endpoint`,
  `ma_engine_set_volume`/`get_volume`, `set_gain_db`/`get_gain_db`,
  `ma_engine_set_time*`, `ma_engine_play_sound`(+`_ex`).
- Listener/3D: `ma_engine_listener_set_position`, `_set_direction`,
  `_set_velocity`, `_set_cone`, `_set_world_up`, `_set_enabled`,
  `_get_cone`.
- `ma_sound_*` (141 functions) and `ma_sound_group_*`: init from file / data
  source / copy, start/stop/fade, volume, pan (`ma_pan_mode`), pitch,
  spatialization (`ma_attenuation_model`, `ma_positioning`), cone, min/max
  distance/gain, rolloff, doppler, start/stop times, end callback.
- **Relationship to `audio`:** the existing `audio` package is a pure-Go mixer.
  The engine is an alternative, richer backend. Decide during this phase
  whether `audio` gains an engine-backed mode or whether `engine` is a
  sibling package. Both must remain no-CGO.

**Deliverables:** `engine` package (or `audio` engine mode), sound/groups,
spatialization, examples.

**Depends on:** Phases 0, 2, 7, 9, 10 (engine uses resource manager + decoder +
node graph internally). **Acceptance:** null-backend playback of a sound, group
gain cascade, pan and basic attenuation verified; listener updates reflected.

### Phase 12 - Resource management and VFS (decision gate)

**Goal:** explicitly decide what, if anything, to expose.

**Scope / options**
1. **Do not expose** (recommended default). Use Go `io`, `os`, `io/fs`, and the
   Phase 7 `Decoder` directly. Rationale: the resource manager's async loading
   and registration model duplicates Go concurrency and filesystem idioms.
2. **Minimal exposure** if engine consumers need registered sounds:
   `ma_resource_manager_init`, `ma_resource_manager_uninit`,
   `ma_resource_manager_register_encoded_data`/`register_decoded_data`,
   `ma_resource_manager_data_source_*` init/read/seek.
3. **Full exposure** only if demanded, including `ma_vfs_*` custom backends.

**Deliverable:** a written decision (beads `decision` type) plus implementation
if option 2 or 3 is chosen.

**Depends on:** Phase 11 (engine). **Acceptance:** decision recorded with
rationale; if implemented, registration round-trip tests pass.

## 6. Sequencing and dependency graph

```
Phase 0 (foundation)
  └─ Phase 1 (device/context)
  └─ Phase 2 (pcm/channel)
       ├─ Phase 3 (resampling)
       │    ├─ Phase 4 (buffers)
       │    ├─ Phase 5 (waveform/noise)
       │    └─ Phase 6 (filters)
       │         └─ Phase 9 (data sources)
       │              └─ Phase 10 (node graph)
       │                   └─ Phase 11 (engine/sound)
       │                        └─ Phase 12 (resource mgmt decision)
       ├─ Phase 7 (decoding)
       └─ Phase 8 (encoding)
```

Phases 4, 5, 6, 7, 8 are mutually independent once Phase 3 (or Phase 2 for
7/8) is done and may be scheduled in parallel. Each phase is a separate
multi-platform embed revision, so batching is a scheduling consideration.

## 7. Risks

| Risk | Mitigation |
| --- | --- |
| ABI drift: Go-mirrored structs break on version bumps | Generated layout assertions from the vendored header (section 3.3); prefer `*_sizeof`/`*_config_init`; keep mirrors minimal |
| Bridge creep: new features pile logic into C | Hard minimal-C rule (section 3.1); every shim must be justified; Phase 0 shrinks existing debt |
| Accidental CGO introduced by a new dependency | `CGO_ENABLED=0` everywhere; CI builds without a C compiler for Go code; no cgo files allowed |
| Every symbol addition forces a 7-target rebuild + embed commit | Batch symbols per phase; one rebuild per phase |
| Engine drags in resource manager, decoder, VFS, threads | Explicit dependency: Phase 11 last; accept internal resource manager, do not expose VFS |
| Callback lifetime bugs across the C boundary (dangling tokens) | Token registry + explicit uninit/Close; lifecycle tests; keep `runtime.KeepAlive` |
| Audio-thread vs. control-thread data races | Document miniaudio's locking model (section 7.2) and mirror its rules in Go wrappers |
| Feature sprawl beyond Go-idiomatic needs | Section 4 non-goals; Phase 12 decision gate |
| Version bump (`update-lib.yml`) desynchronizing new symbols | New symbols are added to the generator and bridge together; `verify-embeds` enforces |

## 8. Tracking (beads)

Program epic: **`mago-8a3`** - "Expand miniaudio API surface beyond device playback".

| Phase | Epic | |
| --- | --- | --- |
| 0 - minimal-C foundation | `mago-8a3.1` | |
| 1 - device/context | `mago-8a3.2` | |
| 2 - PCM/channels | `mago-8a3.3` | |
| 3 - resampling | `mago-8a3.4` | |
| 4 - buffers | `mago-8a3.5` | |
| 5 - waveform/noise | `mago-8a3.6` | |
| 6 - filters/effects | `mago-8a3.7` | |
| 7 - decoding | `mago-8a3.8` | |
| 8 - encoding | `mago-8a3.9` | |
| 9 - data sources | `mago-8a3.10` | |
| 10 - node graph | `mago-8a3.11` | |
| 11 - engine/sound | `mago-8a3.12` | |
| 12 - resource mgmt decision | `mago-8a3.13` | |

Each phase epic carries its child tasks and a `blocks` edge to the phases it
depends on. `bd ready` therefore surfaces Phase 0's tasks first. Run
`bd dep tree mago-8a3` for the full graph.

## 9. Appendix: full API inventory

The complete list of 1,187 declared functions was extracted from the vendored
header with:

```bash
sed -nE 's/^MA_API [^(]*\b(ma_[a-z0-9_]+)\(.*/\1/p' miniaudio.h | sort -u
```

Counts are grouped by module prefix in the table in section 2. The header's own
table of contents (sections 1-17, from "Low Level API" through "Optimization
Tips") is the authoritative grouping and matches the phasing above.