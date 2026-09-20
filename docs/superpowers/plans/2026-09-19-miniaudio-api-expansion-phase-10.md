# Phase 10: Node Graph Routing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Go build and drive a miniaudio node graph: nodes, routing, bus volumes and scheduled state changes, ending in a source -> filter -> endpoint pipeline that a later phase's engine can reuse.

**Architecture:** Every `ma_*_node_config` is a small, fixed-layout struct that Go can mirror exactly and fill in itself, because each `ma_*_node_init` sets the node's vtable internally. That means this phase needs **no new C**: the only native change is two extra `calloc` cases in `mago_alloc`. Go mirrors the configs in `types.go`, allocates the object through `mago_alloc`, and binds `ma_node_*` and `ma_*_node_*` symbols directly.

**Tech Stack:** Go 1.26, `purego` v0.10.0, `zig cc`, mise tasks, `bd`.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md` (Phase 10)
**Deviations:** `docs/superpowers/specs/2026-09-20-miniaudio-deviation-register.md`
**Predecessors:** `.../phase-0.md` … `.../phase-9.md`

## Global Constraints

- **ZERO CGO.** `mise run check-cgo` must pass.
- **Minimal C.** No new bridge functions. The only C change is `mago_alloc` cases for
  the twelve new object types; nothing here needs allocation-by-value, a config mirror
  in C, or a callback trampoline.
- **Safe-slice APIs.** Public read methods take Go slices; examples must not import `unsafe`.
- **One native rebuild for the phase** (Task 1).
- **Never hand-edit** `zz_generated.bindings.go` or `embed_*.go`.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

### Verified miniaudio 0.11.25 facts for this phase

- `ma_node_graph_get_node_count` **does not exist**. The real graph introspection API is
  `ma_node_graph_get_channels`, `ma_node_graph_get_time`, `ma_node_graph_set_time`,
  `ma_node_graph_get_processing_size_in_frames`.
- `ma_lpf1_node`, `ma_lpf2_node`, `ma_hpf1_node`, `ma_hpf2_node`, `ma_bpf2_node`,
  `ma_notch2_node`, `ma_peak2_node`, `ma_loshelf2_node` and `ma_hishelf2_node` **do not
  exist**. The node variants are exactly: data source, splitter, biquad, lpf, hpf, bpf,
  notch, peak, loshelf, hishelf, delay.
- `ma_node_state` is `ma_node_state_started = 0`, `ma_node_state_stopped = 1`.
- `MA_NODE_BUS_COUNT_UNKNOWN` is `255`.
- `ma_node_config_init()` sets `vtable = NULL`, `initialState = started`,
  `inputBusCount = outputBusCount = MA_NODE_BUS_COUNT_UNKNOWN`.
- The `ma_*_node_config_init` functions only zero the config, copy `ma_node_config_init()`
  into `nodeConfig`, and fill the type-specific members with `ma_format_f32`. Crucially
  they do **not** set the vtable; each `ma_*_node_init` overrides `vtable` itself. A Go
  mirror can therefore build every config without ever seeing a vtable.
- `ma_data_source_node_init` requires the data source to report `ma_format_f32`, otherwise
  it returns `MA_INVALID_ARGS`.
- `ma_audio_buffer`, `ma_decoder`, `ma_noise` and `ma_waveform` all begin with
  `ma_data_source_base` (or a struct that does), so their pointer is already a valid
  `ma_data_source*`; `ma_audio_buffer` reaches it through `ma_audio_buffer_ref.ref`.
- Thread-safety model, quoted from the header: only `ma_node_graph_read_pcm_frames()` is
  lock-free and audio-thread-only. Attachment, detachment and uninitialization take
  per-bus spinlocks; `ma_node_detach_output_bus()` may stall until the audio thread
  finishes the bus. Global and local clocks advance on the audio thread.
- `ma_node_graph_read_pcm_frames` writes f32 frames in the graph's channel count.
- `MA_MAX_NODE_BUS_COUNT` is 254.

### Deliberate scope cuts (recorded for the deviation register)

- **Custom nodes** (`ma_node_init` with a Go-authored `ma_node_vtable`) are not exposed.
  They need `onProcess`/`onGetRequiredInputFrameCount` trampolines and a bridge object,
  which belongs with the engine work in Phase 11/12.
- `ma_node_init_preallocated` / `ma_node_get_heap_size` are not exposed; they exist only
  to let a caller own the node heap, and mago always uses miniaudio's own allocator.

---

## Task 1: Native allocation, bindings and layout probe (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c`
- Modify: `internal/gen/bindings/main.go`
- Modify: `types.go`
- Modify: `internal/abi/layout_probe.c`
- Modify: `layout_test.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`

**Interfaces:**
- Produces Go handles: `nodeHandle`, `nodeGraphHandle`.
- Produces Go mirrors: `nodeConfigNative`, `nodeGraphConfigNative`,
  `dataSourceNodeConfigNative`, `splitterNodeConfigNative`, `biquadNodeConfigNative`,
  `lpfNodeConfigNative` (aliased for hpf/bpf), `notchNodeConfigNative`,
  `peakNodeConfigNative`, `loshelfNodeConfigNative` (aliased for hishelf),
  `delayNodeConfigNative`.
- Produces bindings for the whole `ma_node_*` / `ma_node_graph_*` surface and the eleven
  `ma_*_node_*` wrappers.

- [ ] **Step 1: Add the twelve object types to the bridge allocator**

Append to `enum mago_object_type`:

```c
    MAGO_OBJECT_ENCODER            = 31,
    MAGO_OBJECT_NODE_GRAPH         = 32,
    MAGO_OBJECT_DATA_SOURCE_NODE   = 33,
    MAGO_OBJECT_SPLITTER_NODE      = 34,
    MAGO_OBJECT_BIQUAD_NODE        = 35,
    MAGO_OBJECT_LPF_NODE           = 36,
    MAGO_OBJECT_HPF_NODE           = 37,
    MAGO_OBJECT_BPF_NODE           = 38,
    MAGO_OBJECT_NOTCH_NODE         = 39,
    MAGO_OBJECT_PEAK_NODE          = 40,
    MAGO_OBJECT_LOSHELF_NODE       = 41,
    MAGO_OBJECT_HISHELF_NODE       = 42,
    MAGO_OBJECT_DELAY_NODE         = 43
```

and the matching `calloc` cases with `sizeof(ma_node_graph)`,
`sizeof(ma_data_source_node)`, `sizeof(ma_splitter_node)`, `sizeof(ma_biquad_node)`,
`sizeof(ma_lpf_node)`, `sizeof(ma_hpf_node)`, `sizeof(ma_bpf_node)`,
`sizeof(ma_notch_node)`, `sizeof(ma_peak_node)`, `sizeof(ma_loshelf_node)`,
`sizeof(ma_hishelf_node)`, `sizeof(ma_delay_node)`.

- [ ] **Step 2: Mirror the node object constants and configs in `types.go`**

```go
const (
	nodeGraphObjectType    int32 = 32
	dataSourceNodeType     int32 = 33
	...
)
```

plus the config mirrors listed above, each with the same `// mirrors ... Validated by
layout_test.go.` comment style used by the earlier phases.

- [ ] **Step 3: Add the binding specs**

Graph, node base and node types, e.g.:

```go
{FieldName: "maNodeGraphInit", Symbol: "ma_node_graph_init", Type: "func(*nodeGraphConfigNative, unsafe.Pointer, *nodeGraphHandle) Result"},
{FieldName: "maNodeGraphUninit", Symbol: "ma_node_graph_uninit", Type: "func(*nodeGraphHandle, unsafe.Pointer)"},
{FieldName: "maNodeGraphGetEndpoint", Symbol: "ma_node_graph_get_endpoint", Type: "func(*nodeGraphHandle) *nodeHandle"},
{FieldName: "maNodeGraphReadPCMFrames", Symbol: "ma_node_graph_read_pcm_frames", Type: "func(*nodeGraphHandle, unsafe.Pointer, uint64, *uint64) Result"},
{FieldName: "maNodeGraphGetChannels", Symbol: "ma_node_graph_get_channels", Type: "func(*nodeGraphHandle) uint32"},
{FieldName: "maNodeGraphGetTime", Symbol: "ma_node_graph_get_time", Type: "func(*nodeGraphHandle) uint64"},
{FieldName: "maNodeGraphSetTime", Symbol: "ma_node_graph_set_time", Type: "func(*nodeGraphHandle, uint64) Result"},
{FieldName: "maNodeGraphGetProcessingSizeInFrames", Symbol: "ma_node_graph_get_processing_size_in_frames", Type: "func(*nodeGraphHandle) uint32"},
{FieldName: "maNodeAttachOutputBus", Symbol: "ma_node_attach_output_bus", Type: "func(*nodeHandle, uint32, *nodeHandle, uint32) Result"},
...
```

- [ ] **Step 4: Extend the layout probe and `layout_test.go`**

Add `sizeof:`/`offsetof:` lines for every mirrored struct and assert them. The
`nodeConfigNative` offsets are the important ones, because that struct is the one whose
padding Go must reproduce:

```go
node := nodeConfigNative{}
nodeOffsets := map[string]struct {
	got uintptr
	key string
}{
	"vtable":         {unsafe.Offsetof(node.VTable), "offsetof:ma_node_config.vtable"},
	"initialState":   {unsafe.Offsetof(node.InitialState), "offsetof:ma_node_config.initialState"},
	"inputBusCount":  {unsafe.Offsetof(node.InputBusCount), "offsetof:ma_node_config.inputBusCount"},
	"outputBusCount": {unsafe.Offsetof(node.OutputBusCount), "offsetof:ma_node_config.outputBusCount"},
	"pInputChannels": {unsafe.Offsetof(node.InputChannels), "offsetof:ma_node_config.pInputChannels"},
	"pOutputChannels": {unsafe.Offsetof(node.OutputChannels), "offsetof:ma_node_config.pOutputChannels"},
}
```

- [ ] **Step 5: Rebuild and regenerate**

```bash
mise run build-lib-all
mise run generate
go test -run TestMirroredStructLayouts ./...
go build ./...
```

---

## Task 2: `NodeGraph` and the `Node` base

**Files:**
- Create: `nodegraph.go`

- [ ] **Step 1: `NodeState` and `NodeGraphConfig`**

```go
// NodeState is the playback state of a graph node.
type NodeState int32

const (
	NodeStateStarted NodeState = 0
	NodeStateStopped NodeState = 1
)
```

`NodeGraphConfig{Channels, ProcessingSizeInFrames uint32, PreMixStackSizeInBytes uint64}`,
mirrored from `ma_node_graph_config`.

- [ ] **Step 2: `Node` interface and its shared implementation**

`Node` is the routing view of any graph object. It carries one unexported method
(`nodeHandle`) so only this package can implement it:

```go
type Node interface {
	Graph() *NodeGraph
	InputBusCount() uint32
	OutputBusCount() uint32
	InputChannels(inputBusIndex uint32) uint32
	OutputChannels(outputBusIndex uint32) uint32
	AttachOutputBus(outputBusIndex uint32, other Node, otherInputBusIndex uint32) error
	DetachOutputBus(outputBusIndex uint32) error
	DetachAllOutputBuses() error
	SetOutputBusVolume(outputBusIndex uint32, volume float32) error
	OutputBusVolume(outputBusIndex uint32) float32
	State() NodeState
	SetState(state NodeState) error
	SetStateTime(state NodeState, globalTime uint64) error
	StateTime(state NodeState) uint64
	StateByTime(globalTime uint64) NodeState
	StateByTimeRange(globalTimeBeg, globalTimeEnd uint64) NodeState
	Time() uint64
	SetTime(localTime uint64) error

	nodeHandle() *nodeHandle
}
```

`nodeCommon` implements all of it over `lib` + `handle`, so each concrete node only
embeds it.

- [ ] **Step 3: `NodeGraph`**

```go
func (lib *Library) NewNodeGraph(config NodeGraphConfig) (*NodeGraph, error)
func (g *NodeGraph) Endpoint() Node
func (g *NodeGraph) Read(out []float32) (uint64, error)  // len(out) % channels == 0
func (g *NodeGraph) Channels() uint32
func (g *NodeGraph) Time() uint64
func (g *NodeGraph) SetTime(globalTime uint64) error
func (g *NodeGraph) ProcessingSizeInFrames() uint32
func (g *NodeGraph) Close() error
```

`Endpoint()` returns a borrowed node that must not be closed. `Read` translates
`AtEnd` into a normal short read, matching the other read methods in this package.

- [ ] **Step 4: Build**

```bash
go build ./...
```

---

## Task 3: Concrete node wrappers

**Files:**
- Create: `nodes.go`

Each wrapper embeds `nodeCommon` and adds its own extras:

| Go type | init binding | extras |
| --- | --- | --- |
| `DataSourceNode` | `ma_data_source_node_init` | `SetLooping`, `IsLooping`, `Source` |
| `SplitterNode` | `ma_splitter_node_init` | none |
| `BiquadNode` | `ma_biquad_node_init` | `Reinit` |
| `LowPassNode` / `HighPassNode` / `BandPassNode` | `ma_*_node_init` | `Reinit` |
| `NotchNode` / `PeakNode` / `LowShelfNode` / `HighShelfNode` | `ma_*_node_init` | `Reinit` |
| `DelayNode` | `ma_delay_node_init` | `SetWet`/`Wet`/`SetDry`/`Dry`/`SetDecay`/`Decay` |

Constructors hang off the graph, because a node cannot exist without one:

```go
func (g *NodeGraph) NewDataSourceNode(source DataSource) (*DataSourceNode, error)
func (g *NodeGraph) NewSplitterNode(config SplitterNodeConfig) (*SplitterNode, error)
func (g *NodeGraph) NewBiquadNode(config BiquadNodeConfig) (*BiquadNode, error)
func (g *NodeGraph) NewLowPassNode(config FilterNodeConfig) (*LowPassNode, error)
...
func (g *NodeGraph) NewDelayNode(config DelayConfig) (*DelayNode, error)
```

Config types mirror the miniaudio node configs and therefore carry no `Format` field;
node processing is always f32.

`NewDataSourceNode` resolves a native `ma_data_source*`:

- `*Decoder`, `*AudioBuffer`, `*Waveform`, `*Noise` and `*CustomDataSource` already are
  data sources, so their pointer is used directly.
- any other `DataSource` is wrapped in an internal `CustomDataSource` that the
  `DataSourceNode` owns and closes with itself.

`Close()` on a node detaches all output buses, uninitialises and frees the object.

- [ ] **Step 1: Add the four `nativeDataSource` methods** to the existing objects and to
  `CustomDataSource`, then write `nodes.go`.
- [ ] **Step 2:** `go build ./...` and `mise run lint`.

---

## Task 4: Unsupported-platform stubs

**Files:**
- Modify: `unsupported.go`

Add `NodeState`, `Node`, `NodeGraph`, the config types and the node types with the same
signatures, all returning `errUnsupportedPlatform`.

---

## Task 5: Tests

**Files:**
- Create: `nodegraph_test.go`

- [ ] **Step 1: Source -> endpoint produces signal**

Build a `Waveform` sine as source, wrap it in a `DataSourceNode`, attach to the endpoint,
read a block and assert the peak is within a sane range and that silence is not returned.

- [ ] **Step 2: Source -> biquad -> endpoint changes the signal**

Route the same source through a low-pass biquad set to a frequency far below the tone and
assert the peak drops relative to the direct path.

- [ ] **Step 3: Splitter routing**

Split a waveform, attach output bus 0 straight to the endpoint and output bus 1 through a
delay node, and assert the graph output is louder than the direct-only path.

- [ ] **Step 4: Bus volume**

Set the data source node's output bus volume to 0 and assert the graph reads silence.

- [ ] **Step 5: State and time**

Assert `SetState(stopped)` stops the flow, `StateByTime`/`StateTime` round-trip the
schedule, and graph time advances with `Read`.

- [ ] **Step 6: Lifecycle**

Cover double `Close`, use after `Close`, and `Close` on a node from a closed library.
Add the node cases to `lifecycle_test.go` if that reads better.

```bash
mise run test
```

---

## Task 6: Example, README and package docs

**Files:**
- Create: `examples/nodegraph/main.go`
- Modify: `README.md`
- Modify: `doc.go` (if the root doc needs the graph mentioned)
- Modify: `AGENTS.md` if a new convention was introduced

The example builds a waveform -> low-pass -> delay rack, prints the graph channel count,
processing size and a few peak measurements, and needs no audio device at all.

---

## Task 7: Verification

- [ ] `mise run test`
- [ ] `mise run lint` (includes `check-cgo` and `govulncheck`)
- [ ] `mise run build-lib-all && mise run generate && git diff --exit-code` on the
      generated files
- [ ] Close `mago-8a3.11.1`, `mago-8a3.11.2`, `mago-8a3.10.3`, `mago-8a3.11.3` and
      `mago-8a3.11`
- [ ] Update the deviation register if any finding was resolved or created
- [ ] Push and open a PR
