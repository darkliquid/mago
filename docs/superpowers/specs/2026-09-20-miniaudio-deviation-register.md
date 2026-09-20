# miniaudio Expansion: Deviation Register and Remediation Roadmap

**Status:** Proposed
**Date:** 2026-09-20
**Scope:** phases 0-9 of `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md`
**Vendored miniaudio version:** `0.11.25`

## 1. Why this document exists

Each phase split its work into tasks and committed with a message that explained any
deviation. Those messages are the only record, which means partial implementations and
stubs are easy to forget. This document is the durable register: every known deviation
from the phased plan, with evidence, and a remediation workstream for each.

It was produced by auditing the tree rather than relying on memory:

```bash
# stubs and "not supported" paths
grep -rnE "TODO|FIXME|not implemented|not supported|not wired|no effect" --include=*.go . | grep -v _test.go

# bindings that nothing calls
# (fields of bindingSet in zz_generated.bindings.go never referenced as bindings.<name>)

# API gaps: symbols present in the header but absent from the generator
```

**Rule going forward:** when a phase deviates, add a row here in the same commit that
introduces it. A deviation that is not in this table is a bug in the process.

## 2. Deviation register

Legend: **P** = originating phase, **Sev** = severity (H high, M medium, L low).

### 2.1 Partially implemented or stubbed behaviour

| ID | P | Sev | Deviation | Evidence |
| --- | --- | --- | --- | --- |
| D1 | 4 | M | `AudioBufferRef` has no `SeekToPCMFrame`. The binding is registered and unused; the safe-slice rewrite dropped the method that Phase 4 added. | `buffer.go` has no `func (r *AudioBufferRef) SeekToPCMFrame`; `maAudioBufferRefSeekToPCMFrame` is an unused binding |
| D2 | 9 | M | `Decoder.SetLooping` accepts the request and does nothing. Decoder looping is supported by miniaudio but not wired through. | `datasource_adapters.go`: "Decoder looping is not wired up yet" |
| D3 | 5 | L | `Noise.SetType` is a stub that always returns `InvalidOperation`, because `ma_noise_set_type` asserts inside miniaudio. The corresponding binding is registered and never called. | `noise.go` "directly returns InvalidOperation rather than calling"; `maNoiseSetType` unused |
| D4 | 5 | L | `Waveform`/`Noise` report errors for `CursorInPCMFrames`, `LengthInPCMFrames` and (for Noise) `SeekToPCMFrame`. Honest, but they are unusable where those operations are required. | `datasource_adapters.go` |
| D5 | 8 | M | `audio.Clip.Encode` is WAV-only and hardcodes `mago.DefaultEncoderConfig`. FLAC encoding exists in `Encoder` but is unreachable from a clip. | `audio/stream.go` |
| D6 | 8 | L | A `Clip` built by hand (not through an `Engine`) cannot be encoded; `Encode` errors instead of falling back. | `audio/stream.go` nil-library check |
| D7 | 1 | L | The loopback device type is accepted and validated but has no test; it is WASAPI-only, so it cannot be exercised on the null backend. | only `device_supported.go` references `DeviceTypeLoopback` |
| D8 | 3 | L | Custom resampling backends, `get_heap_size` and `init_preallocated` are not exposed. Recorded as a decision, not an oversight. | `mago-8a3.4.3`; `resample.go` rejects custom backends |
| D9 | 2/3 | L | Custom channel mixing weights are rejected in the channel converter and the data converter. | `channel.go`, `resample.go` |
| D19 | 10 | M | Custom nodes are not exposed. `ma_node_init` accepts a caller-supplied `ma_node_vtable`, but reaching it from Go needs `onProcess` and `onGetRequiredInputFrameCount` trampolines plus a bridge object, which belongs with the engine work. | `nodes.go` covers miniaudio's own node types only; `ma_node_init` is not bound |
| D20 | 10 | L | `ma_node_init_preallocated` and `ma_node_get_heap_size` are not exposed. mago always lets miniaudio own a node's heap. Recorded as a decision, matching D8. | `nodes.go` and `nodegraph.go` allocate with `mago_alloc` and pass NULL allocation callbacks |
| D23 | 11 | M | `ma_engine_get_device`, `ma_engine_get_log` and `ma_engine_get_resource_manager` are not exposed. Handing out a borrowed `Device` or `Log` would let a caller close something the engine owns, and the resource manager is Phase 12's subject. | `engine.go` exposes only the endpoint and the node graph, both of which refuse to close |
| D24 | 11 | L | The `_in_milliseconds` and `_in_seconds` variants are not bound. Go `time.Duration` wrappers cover the same ground on top of the frame-based functions, converting with the engine's sample rate exactly as miniaudio's millisecond variants do. | `sound.go` `*InDuration` methods; no `ma_sound_*_in_milliseconds` binding |
| D25 | 11 | L | `ma_sound_config` and `ma_sound_init_ex`, `ma_sound_init_from_file_w`, and the deprecated `ma_engine_get_time` / `set_time` and `*_config_init` helpers are not bound. The convenience initialisers build that config internally, so Go never has to mirror it. | `sound.go`; `ma_sound_config` has no Go mirror |
| D26 | 11 | L | `ma_engine_config.onProcess` and the engine's `dataCallback` / `notificationCallback` hooks are not exposed. A device-less engine gives the caller the mixed frames directly, which covers the common use. | `engineConfigNativeFrom` leaves them NULL |
| D27 | 9 | M | **Fixed in Phase 11.** A Go `DataSource` could never signal end of stream: the bridge returned `MA_SUCCESS` even when the source produced no frames, so miniaudio never set a sound's at-end flag and end callbacks could not fire. The bridge now returns `MA_AT_END` for a zero-frame read. | `datasource.go` `dataSourceReadPtr`; `TestSoundEndCallback` |

### 2.2 Dead bindings

Ten of 404 registered bindings are never called. Dead bindings are not harmless: they are
symbols we must keep exported and regenerated for seven targets.

| Binding | Notes |
| --- | --- |
| `maAudioBufferRefSeekToPCMFrame` | see D1 |
| `maNoiseSetType` | see D3 |
| `maRBGetSubbufferSize` / `Stride` / `Offset` / `Ptr` | sub-buffer geometry is not surfaced in the Go API |
| `maPCMRBGetSubbufferSize` / `Stride` / `Offset` / `Ptr` | same, frame-based |

### 2.3 API-surface gaps

| ID | P | Sev | Gap | Notes |
| --- | --- | --- | --- | --- |
| D10 | 3 | M | `ma_convert_frames_ex` is not exposed | needs an `ma_data_converter_config` mirror or setter shim |
| D11 | 9 | M | Data source chaining, ranges and loop points are absent: `ma_data_source_set_next`, `set_range_in_pcm_frames`, `set_loop_point_in_pcm_frames`, `get_length_in_seconds`, `get_cursor_in_seconds`, `seek_pcm_frames` | `CustomDataSource` therefore cannot express chained or ranged playback |
| D12 | 7/8 | M | `*_vfs` variants (`ma_decoder_init_vfs`, `ma_encoder_init_vfs`) are absent | blocked by the Phase 12 VFS decision (D13) |
| D13 | 12 | M | The resource-manager/VFS decision is still open | `mago-8a3.13` |
| D14 | 4 | L | `RingBuffer`/`PCMRingBuffer` sub-buffer helpers are bound but not surfaced | see section 2.2 |

### 2.4 Platform and stub hygiene

| ID | P | Sev | Deviation | Evidence |
| --- | --- | --- | --- | --- |
| D15 | 0-9 | H | `zz_generated.bindings.go` has **no build tag** and imports `purego`, so an unsupported GOOS fails inside purego before the stub API is reached. `unsupported.go` is therefore unverifiable dead code. | `GOOS=plan9 go build ./...` fails with `undefined: syscall15Args` in purego |
| D16 | 7-9 | H | `unsupported.go` has no stubs for `Decoder`, `Encoder`, `CustomDataSource` or the Phase 9 adapters on `AudioBuffer`, `Waveform` and `Noise`, so its advertised API is incomplete even if D15 were fixed. The bare `DataSource` interface stub landed with Phase 10 (D22) because the node types need it. | grep counts are 0 for those names; see D22 |
| D17 | 0-9 | L | No test fails when a binding becomes dead, so section 2.2 will keep growing. | the audit above is manual |

### 2.5 Process

| ID | Sev | Deviation |
| --- | --- | --- |
| D18 | M | Deviations live only in commit messages and closed beads. This document is the first durable register; keeping it current is a process obligation, not a task. |
| D21 | L | The Phase 10 section of the program spec named APIs that do not exist in miniaudio 0.11.25: `ma_node_graph_get_node_count`, and the `ma_lpf1_node` / `ma_lpf2_node` / `ma_hpf1_node` / `ma_hpf2_node` / `ma_bpf2_node` / `ma_notch2_node` / `ma_peak2_node` / `ma_loshelf2_node` / `ma_hishelf2_node` filter-node variants. The spec hedged with "verify exact names at implementation time"; Phase 10 implemented the real surface, and the graph's introspection is `ma_node_graph_get_channels` / `get_time` / `set_time` / `get_processing_size_in_frames`. |
| D22 | L | Phase 10 needed a `DataSource` interface stub in `unsupported.go` because `DataSourceNode.Source` returns one, so a sliver of WS1.2 (D16) landed early. The rest of D16 is still open. |
| D28 | L | Phase 11 settled bead `mago-8a3.12.3`: `audio` stays a pure-Go mixer over a device callback, and `mago.Engine` is a sibling, miniaudio-backed alternative. Making `audio` engine-backed would change its `Clip`/`Stream` model and its threading, and both layers have to stay CGO-free anyway. The README states the split under "Choosing between `audio` and `Engine`". |

## 3. Remediation roadmap

The deviations fall into four workstreams. Each is independent enough to ship on its own.

### WS1 - Platform build hygiene (D15, D16, D17)

Make the unsupported-platform story real rather than nominal.

1. Split the generated bindings output in two:
   - an **untagged** file with the version constants, `Result`/`Backend`/`Format`/...
     constants and nothing that imports purego;
   - a **build-tagged** file with `bindingSet`, its `register` method and the purego
     import.
   This keeps the public constants available on every GOOS while confining purego to
   supported platforms.
2. Complete `unsupported.go` for phases 5-9 (decoder, encoder, data source, adapters,
   filters) so the stub API matches the supported one.
3. Add a CI check: `GOOS=plan9 GOARCH=amd64 go build ./...` (or any GOOS outside the
   supported set) must succeed. This is the first time the stub would actually be
   compiled, so expect it to need work.
4. Add a test that fails when a registered binding is unused unless it is allowlisted
   with a reason, mirroring the `internal/cgoguard` approach. This is the guard that
   keeps section 2.2 from growing.

**Acceptance:** an unsupported GOOS builds; the dead-binding test fails on a newly
unused binding; the allowlist is small and justified.

### WS2 - Finish what is half-built (D1, D2, D3, D4, D5, D6)

1. Restore `AudioBufferRef.SeekToPCMFrame` and cover it with a test.
2. Either wire decoder looping through the data source API or remove `SetLooping` from
   the decoder adapter so it stops silently ignoring requests. Removing is acceptable if
   looping is not wanted yet, but then the `DataSource` interface needs a documented
   "optional operations" story.
3. Decide `Noise.SetType`: keep the documented stub and delete the binding, or remove the
   method entirely. Do not leave a registered symbol that nothing calls.
4. Define optional-operation semantics for `DataSource` (cursor, length, seek, looping)
   so generators do not look broken. A capability query or an `errors.Is` sentinel is
   enough.
5. Give `Clip.Encode` an encoding-format option and let `Encode` work for hand-built
   clips by accepting a library, or drop the library requirement by making the encoder
   resolve the embedded library itself.

**Acceptance:** no public method silently ignores its argument; the four adapters have a
coherent optional-operation contract.

### WS3 - Close the API gaps (D10, D11, D12, D14)

1. Mirror `ma_data_converter_config` (or add a setter shim) and expose
   `ma_convert_frames_ex`.
2. Expose data source chaining and ranges: `set_next`/`get_next`, `set_range_in_pcm_frames`,
   `set_loop_point_in_pcm_frames`, the seconds-based cursor/length accessors and
   `seek_pcm_frames`, and thread them through `CustomDataSource`.
3. Surface the ring-buffer sub-buffer helpers, or delete those bindings as part of WS1's
   dead-binding cleanup.
4. Add `*_vfs` variants only after the Phase 12 decision (D13); record the outcome here.

**Acceptance:** no bound-but-unused symbols remain; `CustomDataSource` can express chained
and ranged sources.

### WS4 - Test the untested (D7, D9)

1. Add a loopback test that skips with a clear reason when the backend does not support
   loopback, so the code path is at least exercised on WASAPI CI runs.
2. Add rejection tests for custom channel weights (they exist for the converter; extend
   to the data converter) so the documented limitation cannot regress silently.

**Acceptance:** every documented limitation has a test asserting the rejection.

## 4. Sequencing

WS1 and WS2 are independent and can run before or alongside the remaining phases 10-12.
WS3 item 4 and D13 depend on the Phase 12 decision. WS4 is a small tail.

Suggested order:

1. **WS1** (highest value: it makes an existing stub real and adds the guard that keeps
   this register short).
2. **WS2** (removes silent no-ops, which are the most misleading kind of deviation).
3. **WS4** (cheap, closes documented limitations).
4. **WS3** (largest; some items wait on Phase 12).

## 5. Tracking (beads)

Program epic for this register: **`mago-8a3.15`** - "Deviation register: hardening for
phases 0-9".

| Workstream | Epic | Covers |
| --- | --- | --- |
| WS1 platform build hygiene | `mago-8a3.15.1` | D15, D16, D17 (and the sliver of D16 that landed as D22) |
| WS2 finish half-built behaviour | `mago-8a3.15.2` | D1, D2, D3, D4, D5, D6 |
| WS3 close API gaps | `mago-8a3.15.3` | D10, D11, D12, D14, D19 (`mago-8a3.15.3.5`) |
| WS4 test the untested | `mago-8a3.15.4` | D7, D9 |
| Process | `mago-8a3.15.5` | D18, D21 (`mago-8a3.15.5.1`), D28 |

D20, D23, D24, D25 and D26 are recorded decisions rather than work, matching D8.
D22 is a note about how D16 landed, and D27 was fixed in Phase 11, so neither
needs a bead.

`mago-8a3.15.3.4` (`*_vfs` variants) is blocked by `mago-8a3.13`, the Phase 12 VFS
decision, because the variants cannot be designed before that choice is made.

## 6. Acceptance criteria for the register

- Every row in section 2 has a matching bead.
- The probe scripts in section 1 report no new dead bindings and no new "not wired" paths.
- This document is updated in the same commit as any new deviation.