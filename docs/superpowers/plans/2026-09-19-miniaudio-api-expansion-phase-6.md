# Miniaudio API Expansion Phase 6: Filtering and Effects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose miniaudio's digital signal processing (DSP) filter suite (`ma_biquad`, `ma_lpf`, `ma_lpf1`, `ma_lpf2`, `ma_hpf`, `ma_hpf1`, `ma_hpf2`, `ma_bpf`, `ma_bpf2`, `ma_notch2`, `ma_peak2`, `ma_loshelf2`, `ma_hishelf2`) and delay line effect (`ma_delay`) to pure Go with zero CGO.

**Architecture:**
- Native bridge (`native/miniaudio_bridge.c`): Extend `mago_object_type` and `mago_alloc` with object codes 16–29 for each filter and delay structure.
- Codegen (`internal/gen/bindings/main.go`): Bind exported `ma_*` functions for filter initialization, reinit, cache clearing, latency queries, PCM processing, and delay parameters.
- ABI Layout verification: Add struct definitions to `types.go`, probe layout against `miniaudio.h` via `internal/abi/layout_probe.c` and `layout_test.go`.
- Go API (`biquad.go`, `lpf.go`, `hpf.go`, `bpf.go`, `shelf.go`, `delay.go`): Idiomatic Go structs wrapping native handles with thread-safe lifecycle checks, memory management, and buffer processing.
- Cross-compilation & Embed generation: Build native binaries for all 7 supported targets (`mise run build-lib-all`) and regenerate embedded slices (`mise run generate`).

**Tech Stack:** Go 1.24+, `purego`, miniaudio 0.11.25, `zig cc` / `dockercross/osxcross`, `bd` (Beads).

---

## File Structure

| File | Responsibility |
| --- | --- |
| `native/miniaudio_bridge.c` | Enum values `MAGO_OBJECT_*` (16–29) and `mago_alloc` dispatch for filter and delay structs |
| `types.go` | Handle types, internal object type constants, and native config struct mirrors |
| `internal/gen/bindings/main.go` | Symbol binding specifications for all filter and delay functions |
| `zz_generated.bindings.go` | Generated purego bindings |
| `internal/abi/layout_probe.c` | C probe asserting sizeof and offsetof match `miniaudio.h` |
| `layout_test.go` | Go test comparing layout probe output with Go struct layouts |
| `biquad.go` & `biquad_test.go` | `BiquadConfig`, `Biquad` filter type and tests |
| `lpf.go` & `lpf_test.go` | `LowPassFilterConfig`, `LowPassFilter1Config`, `LowPassFilter2Config`, `LowPassFilter`, `LowPassFilter1`, `LowPassFilter2` |
| `hpf.go` & `hpf_test.go` | `HighPassFilterConfig`, `HighPassFilter1Config`, `HighPassFilter2Config`, `HighPassFilter`, `HighPassFilter1`, `HighPassFilter2` |
| `bpf.go` & `bpf_test.go` | `BandPassFilterConfig`, `BandPassFilter2Config`, `BandPassFilter`, `BandPassFilter2` |
| `shelf.go` & `shelf_test.go` | `NotchFilterConfig`, `PeakFilterConfig`, `LowShelfFilterConfig`, `HighShelfFilterConfig`, `NotchFilter`, `PeakFilter`, `LowShelfFilter`, `HighShelfFilter` |
| `delay.go` & `delay_test.go` | `DelayConfig`, `DefaultDelayConfig`, `Delay` type and tests |
| `unsupported.go` | Platform stubs for non-supported operating systems |
| `embed_*.go` | Tracked prebuilt library byte slices for all 7 platforms |

---

## Task 1: Native Bridge Allocation and ABI Struct Mirrors

Extend the C bridge and Go struct definitions to support memory allocation and layout verification for all filters and the delay effect.

**Files:**
- Modify: `native/miniaudio_bridge.c:50-95`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`

- [ ] **Step 1: Write the failing ABI layout test**

Add probe entries to `internal/abi/layout_probe.c` for:
- `ma_biquad_config`
- `ma_lpf1_config`, `ma_lpf2_config`, `ma_lpf_config`
- `ma_hpf1_config`, `ma_hpf2_config`, `ma_hpf_config`
- `ma_bpf2_config`, `ma_bpf_config`
- `ma_notch2_config`, `ma_peak2_config`, `ma_loshelf2_config`, `ma_hishelf2_config`
- `ma_delay_config`

Add matching struct checks in `layout_test.go`.

- [ ] **Step 2: Run layout test to verify it fails**

Run: `go test -run TestNativeLayouts ./...`
Expected: Compilation failure due to missing struct definitions in `types.go`.

- [ ] **Step 3: Update `native/miniaudio_bridge.c` and `types.go`**

In `native/miniaudio_bridge.c`:
```c
enum mago_object_type
{
    ...
    MAGO_OBJECT_BIQUAD            = 16,
    MAGO_OBJECT_LPF1              = 17,
    MAGO_OBJECT_LPF2              = 18,
    MAGO_OBJECT_LPF               = 19,
    MAGO_OBJECT_HPF1              = 20,
    MAGO_OBJECT_HPF2              = 21,
    MAGO_OBJECT_HPF               = 22,
    MAGO_OBJECT_BPF2              = 23,
    MAGO_OBJECT_BPF               = 24,
    MAGO_OBJECT_NOTCH2            = 25,
    MAGO_OBJECT_PEAK2             = 26,
    MAGO_OBJECT_LOSHELF2          = 27,
    MAGO_OBJECT_HISHELF2          = 28,
    MAGO_OBJECT_DELAY             = 29
};
```
And add matching cases in `mago_alloc`:
```c
    case MAGO_OBJECT_BIQUAD:            return calloc(1, sizeof(ma_biquad));
    case MAGO_OBJECT_LPF1:              return calloc(1, sizeof(ma_lpf1));
    case MAGO_OBJECT_LPF2:              return calloc(1, sizeof(ma_lpf2));
    case MAGO_OBJECT_LPF:               return calloc(1, sizeof(ma_lpf));
    case MAGO_OBJECT_HPF1:              return calloc(1, sizeof(ma_hpf1));
    case MAGO_OBJECT_HPF2:              return calloc(1, sizeof(ma_hpf2));
    case MAGO_OBJECT_HPF:               return calloc(1, sizeof(ma_hpf));
    case MAGO_OBJECT_BPF2:              return calloc(1, sizeof(ma_bpf2));
    case MAGO_OBJECT_BPF:               return calloc(1, sizeof(ma_bpf));
    case MAGO_OBJECT_NOTCH2:            return calloc(1, sizeof(ma_notch2));
    case MAGO_OBJECT_PEAK2:             return calloc(1, sizeof(ma_peak2));
    case MAGO_OBJECT_LOSHELF2:          return calloc(1, sizeof(ma_loshelf2));
    case MAGO_OBJECT_HISHELF2:          return calloc(1, sizeof(ma_hishelf2));
    case MAGO_OBJECT_DELAY:             return calloc(1, sizeof(ma_delay));
```

In `types.go`:
Define constants `objectBiquad = 16`, ..., `objectDelay = 29`.
Define handle types: `biquadHandle`, `lpf1Handle`, `lpf2Handle`, `lpfHandle`, `hpf1Handle`, `hpf2Handle`, `hpfHandle`, `bpf2Handle`, `bpfHandle`, `notch2Handle`, `peak2Handle`, `loshelf2Handle`, `hishelf2Handle`, `delayHandle`.
Define native config mirror structs:
`biquadConfigNative`, `lpf1ConfigNative`, `lpf2ConfigNative`, `lpfConfigNative`, `hpf1ConfigNative`, `hpf2ConfigNative`, `hpfConfigNative`, `bpf2ConfigNative`, `bpfConfigNative`, `notch2ConfigNative`, `peak2ConfigNative`, `loshelf2ConfigNative`, `hishelf2ConfigNative`, `delayConfigNative`.

- [ ] **Step 4: Run layout test to verify it passes**

Run: `go test -run TestNativeLayouts ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add native/miniaudio_bridge.c types.go internal/abi/layout_probe.c layout_test.go
git commit -m "feat(abi): add filter and delay native types and ABI layout validation"
```

---

## Task 2: Purego Symbol Binding Generation

Bind all miniaudio C functions for the filter and delay line APIs.

**Files:**
- Modify: `internal/gen/bindings/main.go`
- Generate: `zz_generated.bindings.go`

- [ ] **Step 1: Update `internal/gen/bindings/main.go`**

Add function specifications to `functions`:
- Biquad: `ma_biquad_init`, `ma_biquad_uninit`, `ma_biquad_reinit`, `ma_biquad_clear_cache`, `ma_biquad_process_pcm_frames`, `ma_biquad_get_latency`
- LPF: `ma_lpf1_init`, `ma_lpf1_uninit`, `ma_lpf1_reinit`, `ma_lpf1_clear_cache`, `ma_lpf1_process_pcm_frames`, `ma_lpf1_get_latency`
  `ma_lpf2_init`, `ma_lpf2_uninit`, `ma_lpf2_reinit`, `ma_lpf2_clear_cache`, `ma_lpf2_process_pcm_frames`, `ma_lpf2_get_latency`
  `ma_lpf_init`, `ma_lpf_uninit`, `ma_lpf_reinit`, `ma_lpf_clear_cache`, `ma_lpf_process_pcm_frames`, `ma_lpf_get_latency`
- HPF: `ma_hpf1_init`, `ma_hpf1_uninit`, `ma_hpf1_reinit`, `ma_hpf1_process_pcm_frames`, `ma_hpf1_get_latency`
  `ma_hpf2_init`, `ma_hpf2_uninit`, `ma_hpf2_reinit`, `ma_hpf2_process_pcm_frames`, `ma_hpf2_get_latency`
  `ma_hpf_init`, `ma_hpf_uninit`, `ma_hpf_reinit`, `ma_hpf_process_pcm_frames`, `ma_hpf_get_latency`
- BPF: `ma_bpf2_init`, `ma_bpf2_uninit`, `ma_bpf2_reinit`, `ma_bpf2_process_pcm_frames`, `ma_bpf2_get_latency`
  `ma_bpf_init`, `ma_bpf_uninit`, `ma_bpf_reinit`, `ma_bpf_process_pcm_frames`, `ma_bpf_get_latency`
- Notch2: `ma_notch2_init`, `ma_notch2_uninit`, `ma_notch2_reinit`, `ma_notch2_process_pcm_frames`, `ma_notch2_get_latency`
- Peak2: `ma_peak2_init`, `ma_peak2_uninit`, `ma_peak2_reinit`, `ma_peak2_process_pcm_frames`, `ma_peak2_get_latency`
- LoShelf2: `ma_loshelf2_init`, `ma_loshelf2_uninit`, `ma_loshelf2_reinit`, `ma_loshelf2_process_pcm_frames`, `ma_loshelf2_get_latency`
- HiShelf2: `ma_hishelf2_init`, `ma_hishelf2_uninit`, `ma_hishelf2_reinit`, `ma_hishelf2_process_pcm_frames`, `ma_hishelf2_get_latency`
- Delay: `ma_delay_init`, `ma_delay_uninit`, `ma_delay_process_pcm_frames`, `ma_delay_set_wet`, `ma_delay_get_wet`, `ma_delay_set_dry`, `ma_delay_get_dry`, `ma_delay_set_decay`, `ma_delay_get_decay`

- [ ] **Step 2: Run generator and build prebuilt libraries**

Run:
```bash
go run ./internal/gen/bindings
mise run build-lib-all
mise run generate
```

- [ ] **Step 3: Run existing tests to verify bindings compile**

Run: `go test ./internal/gen/bindings`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/gen/bindings/main.go zz_generated.bindings.go embed_*.go
git commit -m "feat(bindings): generate bindings for filters and delay effects"
```

---

## Task 3: Biquad and Low-Pass Filters (`mago-8a3.7.1` Part 1)

Implement `Biquad`, `LowPassFilter`, `LowPassFilter1`, and `LowPassFilter2`.

**Files:**
- Create: `biquad.go`
- Create: `biquad_test.go`
- Create: `lpf.go`
- Create: `lpf_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing tests for Biquad and LPF**

In `biquad_test.go`:
- Test passthrough coefficients: `b0=1, b1=0, b2=0, a0=1, a1=0, a2=0` yields exact copy of input buffer.
- Test `Reinit`, `ClearCache`, `Latency`, double-`Close`, and invalid args / nil receiver checks.

In `lpf_test.go`:
- Test `LowPassFilter`, `LowPassFilter1`, `LowPassFilter2`:
  - Input: 100 Hz sine wave vs. 5000 Hz sine wave sampled at 44100 Hz.
  - Cutoff: 1000 Hz.
  - Verify high frequency signal is significantly attenuated compared to low frequency signal.
- Test `Reinit`, `ClearCache`, `Latency`, and lifecycle safety.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestBiquad|TestLPF" ./...`
Expected: FAIL due to missing types and methods.

- [ ] **Step 3: Implement `biquad.go`, `lpf.go`, and stubs in `unsupported.go`**

Implement:
- `BiquadConfig`, `Biquad`: `NewBiquad(config BiquadConfig) (*Biquad, error)`
- `LowPassFilter1Config`, `LowPassFilter2Config`, `LowPassFilterConfig`
- `LowPassFilter1`, `LowPassFilter2`, `LowPassFilter`
- Add matching stubs to `unsupported.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestBiquad|TestLPF" ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add biquad.go biquad_test.go lpf.go lpf_test.go unsupported.go
git commit -m "feat: implement Biquad and LowPassFilter families"
```

---

## Task 4: High-Pass and Band-Pass Filters (`mago-8a3.7.1` Part 2)

Implement `HighPassFilter` (1st, 2nd, Nth order) and `BandPassFilter` (2nd, Nth order).

**Files:**
- Create: `hpf.go`
- Create: `hpf_test.go`
- Create: `bpf.go`
- Create: `bpf_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing tests for HPF and BPF**

In `hpf_test.go`:
- Test `HighPassFilter`, `HighPassFilter1`, `HighPassFilter2`:
  - Input: 100 Hz vs 5000 Hz sine wave at 44100 Hz with cutoff at 1000 Hz.
  - Verify low frequency signal is heavily attenuated while high frequency passes through.
  - Test `Reinit`, `Latency`, lifecycle safety.

In `bpf_test.go`:
- Test `BandPassFilter`, `BandPassFilter2`:
  - Center frequency 1000 Hz, Q = 2.
  - Test 1000 Hz (center) passes with minimal attenuation, while 100 Hz and 5000 Hz are attenuated.
  - Test `Reinit`, `Latency`, lifecycle safety.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestHPF|TestBPF" ./...`
Expected: FAIL due to missing types.

- [ ] **Step 3: Implement `hpf.go`, `bpf.go`, and stubs in `unsupported.go`**

Implement:
- `HighPassFilter1Config`, `HighPassFilter2Config`, `HighPassFilterConfig`
- `HighPassFilter1`, `HighPassFilter2`, `HighPassFilter`
- `BandPassFilter2Config`, `BandPassFilterConfig`
- `BandPassFilter2`, `BandPassFilter`
- Add matching stubs in `unsupported.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestHPF|TestBPF" ./...`
Expected: PASS

- [ ] **Step 5: Commit and close `mago-8a3.7.1` in Beads**

```bash
git add hpf.go hpf_test.go bpf.go bpf_test.go unsupported.go
git commit -m "feat: implement HighPassFilter and BandPassFilter families"
bd close mago-8a3.7.1
```

---

## Task 5: Notch, Peak, and Shelf Filters (`mago-8a3.7.2`)

Implement `NotchFilter`, `PeakFilter`, `LowShelfFilter`, and `HighShelfFilter`.

**Files:**
- Create: `shelf.go`
- Create: `shelf_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing tests for Notch, Peak, and Shelving filters**

In `shelf_test.go`:
- **NotchFilter**: Notch at 1000 Hz; 1000 Hz signal is notched out while 200 Hz and 4000 Hz pass.
- **PeakFilter**: Peak at 1000 Hz with +6 dB gain boosts 1000 Hz; with -12 dB gain cuts 1000 Hz.
- **LowShelfFilter**: Shelf at 500 Hz with +6 dB gain boosts frequencies below 500 Hz while frequencies far above remain near unity gain.
- **HighShelfFilter**: Shelf at 2000 Hz with +6 dB gain boosts frequencies above 2000 Hz while frequencies far below remain near unity gain.
- Test `Reinit`, `Latency`, and lifecycle safety on all four filter types.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run TestShelfFilters ./...`
Expected: FAIL

- [ ] **Step 3: Implement `shelf.go` and stubs in `unsupported.go`**

Implement:
- `NotchFilterConfig`, `NotchFilter`
- `PeakFilterConfig`, `PeakFilter`
- `LowShelfFilterConfig`, `LowShelfFilter`
- `HighShelfFilterConfig`, `HighShelfFilter`
- Add matching stubs to `unsupported.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run TestShelfFilters ./...`
Expected: PASS

- [ ] **Step 5: Commit and close `mago-8a3.7.2` in Beads**

```bash
git add shelf.go shelf_test.go unsupported.go
git commit -m "feat: implement Notch, Peak, LowShelf, and HighShelf filters"
bd close mago-8a3.7.2
```

---

## Task 6: Delay Line Effect (`mago-8a3.7.3`)

Implement `Delay` line with wet, dry, and decay feedback controls.

**Files:**
- Create: `delay.go`
- Create: `delay_test.go`
- Modify: `unsupported.go`

- [ ] **Step 1: Write failing test for `Delay`**

In `delay_test.go`:
- Test impulse response with `delayInFrames = 4`, `dry = 0`, `wet = 1`, `decay = 0.5`:
  - Input: `[1.0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]`
  - Output at frame 4: `1.0`
  - Output at frame 8: `0.5`
  - Output at frame 12: `0.25`
- Test `Wet`, `SetWet`, `Dry`, `SetDry`, `Decay`, `SetDecay` getters and setters.
- Test lifecycle safety (`Close`, double-`Close`, use-after-close).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestDelay ./...`
Expected: FAIL

- [ ] **Step 3: Implement `delay.go` and stubs in `unsupported.go`**

Implement:
- `DelayConfig`, `DefaultDelayConfig(channels, sampleRate, delayInFrames uint32, decay float32) DelayConfig`
- `Delay`: `NewDelay(config DelayConfig) (*Delay, error)`
- Methods: `ProcessPCMFrames(pFramesOut, pFramesIn unsafe.Pointer, frameCount uint32) error`, `Wet() float32`, `SetWet(float32) error`, `Dry() float32`, `SetDry(float32) error`, `Decay() float32`, `SetDecay(float32) error`, `Close() error`.
- Add matching stubs to `unsupported.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestDelay ./...`
Expected: PASS

- [ ] **Step 5: Commit and close `mago-8a3.7.3` in Beads**

```bash
git add delay.go delay_test.go unsupported.go
git commit -m "feat: implement Delay line effect with wet/dry/decay controls"
bd close mago-8a3.7.3
```

---

## Task 7: Full Test Suite, Cross-Compilation, Linting, and PR

Run the full test suite, rebuild all target libraries, verify embeds, and close epic `mago-8a3.7`.

**Files:**
- All Phase 6 files

- [ ] **Step 1: Recompile all targets and regenerate embeds**

```bash
mise run build-lib-all
mise run generate
```

- [ ] **Step 2: Run linter and formatting**

```bash
mise run fmt
mise run lint
```
Expected: 0 lint issues, no vulnerabilities.

- [ ] **Step 3: Run full repository test suite**

```bash
mise run test
mise run build
```
Expected: All tests pass across all packages.

- [ ] **Step 4: Close Phase 6 epic `mago-8a3.7` in Beads**

```bash
bd close mago-8a3.7
```

- [ ] **Step 5: Commit, Push, and create PR**

```bash
git commit -m "chore: regenerate embeds and finalize Phase 6"
git push origin feat/miniaudio-phase-6
gh pr create --title "feat: expose DSP filters and delay line effect (Phase 6)" --body "..."
```
