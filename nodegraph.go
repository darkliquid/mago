//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"errors"
	"fmt"
	"unsafe"
)

// nodeBusCountUnknown mirrors MA_NODE_BUS_COUNT_UNKNOWN. A node config that
// leaves its bus counts at this value takes them from the node's own vtable.
const nodeBusCountUnknown uint32 = 255

// maxNodeBusCount mirrors MA_MAX_NODE_BUS_COUNT.
const maxNodeBusCount uint32 = 254

var (
	errNilNode           = errors.New("mago: nil node")
	errNilNodeGraph      = errors.New("mago: nil node graph")
	errBorrowedNodeGraph = errors.New("mago: node graph is owned by its engine and cannot be closed")
)

// defaultNodeConfig returns what ma_node_config_init() would produce. Go cannot
// call it because it returns by value, but its result is fully determined: a
// NULL vtable (which every ma_*_node_init overwrites), the started state and
// unknown bus counts.
func defaultNodeConfig() nodeConfigNative {
	return nodeConfigNative{
		InitialState:   NodeStateStarted,
		InputBusCount:  nodeBusCountUnknown,
		OutputBusCount: nodeBusCountUnknown,
	}
}

// NodeGraphConfig configures a NodeGraph.
type NodeGraphConfig struct {
	// Channels is the channel count of the graph and of every frame it reads.
	Channels uint32
	// ProcessingSizeInFrames is the graph's preferred block size. Zero, the
	// default, lets miniaudio size each block from the frame count passed to
	// NodeGraph.Read.
	ProcessingSizeInFrames uint32
	// PreMixStackSizeInBytes bounds how deeply nodes may be nested. Zero uses
	// miniaudio's default of 512 KiB per channel.
	PreMixStackSizeInBytes uint64
}

// NodeGraph is miniaudio's node graph: a routable audio pipeline that is read
// frame by frame rather than driven by a device callback. Build a rack by
// creating nodes from the graph, attaching their output buses, and then reading
// the graph's endpoint.
//
// Thread safety follows miniaudio exactly. Only Read is lock-free, and it must
// be called from a single audio thread. Creating nodes, attaching and detaching
// buses, changing volumes and uninitialising nodes are control-thread
// operations; each bus carries a spinlock so they may run while Read is active,
// but detaching a bus can block until the audio thread has finished processing
// it. Read the lowest nodes in the chain first when tearing a rack down.
// Graph and node clocks advance on the audio thread, so read them from the
// audio thread too, or accept that they are sampled rather than exact.
type NodeGraph struct {
	lib      *Library
	handle   *nodeGraphHandle
	endpoint *nodeHandle
	channels uint32
	closed   bool
	// owned is false for a graph that belongs to something else, such as an
	// Engine. Close refuses to free one of those.
	owned bool
}

// NewNodeGraph creates an empty node graph with the given channel count.
func (lib *Library) NewNodeGraph(config NodeGraphConfig) (*NodeGraph, error) {
	if lib == nil {
		return nil, errors.New("mago: nil library")
	}
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if config.Channels == 0 {
		return nil, fmt.Errorf("mago: node graph needs at least one channel")
	}

	raw := lib.bindings.magoAlloc(magoObjectNodeGraph)
	if raw == nil {
		return nil, fmt.Errorf("mago: allocate node graph failed")
	}
	handle := (*nodeGraphHandle)(raw)

	native := nodeGraphConfigNative{
		Channels:               config.Channels,
		ProcessingSizeInFrames: config.ProcessingSizeInFrames,
		PreMixStackSizeInBytes: uintptr(config.PreMixStackSizeInBytes),
	}
	if result := lib.bindings.maNodeGraphInit(&native, nil, handle); result != Success {
		lib.bindings.magoFree(raw)
		return nil, lib.resultError("ma_node_graph_init", result)
	}

	return &NodeGraph{
		lib:      lib,
		handle:   handle,
		endpoint: lib.bindings.maNodeGraphGetEndpoint(handle),
		channels: config.Channels,
		owned:    true,
	}, nil
}

// borrowedNodeGraph wraps a node graph that another object owns, so its nodes can
// report their graph and callers can route into its endpoint without being able
// to free it.
func borrowedNodeGraph(lib *Library, handle *nodeGraphHandle, endpoint *nodeHandle, channels uint32) *NodeGraph {
	return &NodeGraph{
		lib:      lib,
		handle:   handle,
		endpoint: endpoint,
		channels: channels,
	}
}

func (g *NodeGraph) ensure() error {
	if g == nil || g.handle == nil {
		return errNilNodeGraph
	}
	return g.lib.ensureOpen()
}

// Endpoint returns the node that all graph output collects into. Every graph
// has exactly one; it is owned by the graph and must never be closed. Endpoint
// returns nil once the graph is closed.
func (g *NodeGraph) Endpoint() Node {
	if g == nil || g.closed || g.endpoint == nil {
		return nil
	}
	return &endpointNode{nodeCommon{lib: g.lib, graph: g, handle: g.endpoint}}
}

// Channels reports the graph's channel count.
func (g *NodeGraph) Channels() uint32 {
	if g == nil || g.handle == nil {
		return 0
	}
	return g.lib.bindings.maNodeGraphGetChannels(g.handle)
}

// ProcessingSizeInFrames reports the configured block size, or zero when the
// graph sizes blocks from each Read call.
func (g *NodeGraph) ProcessingSizeInFrames() uint32 {
	if g == nil || g.handle == nil {
		return 0
	}
	return g.lib.bindings.maNodeGraphGetProcessingSizeInFrames(g.handle)
}

// Read fills out with interleaved f32 frames read from the graph's endpoint and
// returns how many of them the graph actually produced. miniaudio always writes
// the whole slice, silencing whatever the graph had nothing to contribute, so the
// return value is short only when a node in the graph ran out of data.
func (g *NodeGraph) Read(out []float32) (uint64, error) {
	if err := g.ensure(); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	if g.channels == 0 || len(out)%int(g.channels) != 0 {
		return 0, ErrInvalidSliceLength
	}

	frameCount := uint64(len(out) / int(g.channels))
	var framesRead uint64
	result := g.lib.bindings.maNodeGraphReadPCMFrames(g.handle, unsafe.Pointer(&out[0]), frameCount, &framesRead)
	if result == AtEnd {
		return framesRead, nil
	}
	if result != Success {
		return framesRead, g.lib.resultError("ma_node_graph_read_pcm_frames", result)
	}
	return framesRead, nil
}

// Time reports the graph's global clock, in frames.
func (g *NodeGraph) Time() uint64 {
	if g == nil || g.handle == nil {
		return 0
	}
	return g.lib.bindings.maNodeGraphGetTime(g.handle)
}

// SetTime moves the graph's global clock, which is useful for seeking a rack
// built from scheduled nodes.
func (g *NodeGraph) SetTime(globalTime uint64) error {
	if err := g.ensure(); err != nil {
		return err
	}
	return g.lib.resultError("ma_node_graph_set_time", g.lib.bindings.maNodeGraphSetTime(g.handle, globalTime))
}

// Close uninitialises the graph and frees its memory. Close every node created
// from the graph first: miniaudio does not track nodes for you, and a graph that
// is freed while a node still points at it leaves that node dangling.
//
// A graph that belongs to another object, such as an Engine, reports an error
// rather than freeing something it does not own.
func (g *NodeGraph) Close() error {
	if g == nil || g.handle == nil {
		return nil
	}
	if !g.owned {
		return errBorrowedNodeGraph
	}
	if err := g.lib.ensureOpen(); err != nil {
		return err
	}

	g.lib.bindings.maNodeGraphUninit(g.handle, nil)
	g.lib.bindings.magoFree(unsafe.Pointer(g.handle))
	g.handle = nil
	g.endpoint = nil
	g.closed = true
	return nil
}

// Node is the routing view of any object that lives in a NodeGraph. The
// unexported method keeps the interface closed: only the node types in this
// package satisfy it.
type Node interface {
	// Graph reports the graph the node belongs to.
	Graph() *NodeGraph
	// InputBusCount and OutputBusCount report how many buses the node has.
	InputBusCount() uint32
	OutputBusCount() uint32
	// InputChannels and OutputChannels report the channel count of one bus.
	InputChannels(inputBusIndex uint32) uint32
	OutputChannels(outputBusIndex uint32) uint32
	// AttachOutputBus connects one of this node's output buses to an input bus
	// of other. It blocks until the audio thread is done with the bus.
	AttachOutputBus(outputBusIndex uint32, other Node, otherInputBusIndex uint32) error
	// DetachOutputBus disconnects an output bus, blocking until the audio
	// thread has finished processing it.
	DetachOutputBus(outputBusIndex uint32) error
	// DetachAllOutputBuses disconnects every output bus at once.
	DetachAllOutputBuses() error
	// SetOutputBusVolume and OutputBusVolume control per-bus gain.
	SetOutputBusVolume(outputBusIndex uint32, volume float32) error
	OutputBusVolume(outputBusIndex uint32) float32
	// State reports the node's current playback state.
	State() NodeState
	// SetState starts or stops the node immediately.
	SetState(state NodeState) error
	// SetStateTime schedules a state change on the graph's global clock.
	SetStateTime(state NodeState, globalTime uint64) error
	// StateTime reports when a state change is scheduled.
	StateTime(state NodeState) uint64
	// StateByTime reports the state the node is in at a global time.
	StateByTime(globalTime uint64) NodeState
	// StateByTimeRange reports the state over a span of global time.
	StateByTimeRange(globalTimeBeg, globalTimeEnd uint64) NodeState
	// Time and SetTime work with the node's own local clock.
	Time() uint64
	SetTime(localTime uint64) error

	nodeHandle() *nodeHandle
}

// nodeCommon implements the shared ma_node_* surface for every node type.
type nodeCommon struct {
	lib    *Library
	graph  *NodeGraph
	handle *nodeHandle
}

func (n *nodeCommon) ensure() error {
	if n == nil || n.handle == nil {
		return errNilNode
	}
	return n.lib.ensureOpen()
}

func (n *nodeCommon) nodeHandle() *nodeHandle {
	if n == nil {
		return nil
	}
	return n.handle
}

// Graph reports the graph the node belongs to.
func (n *nodeCommon) Graph() *NodeGraph {
	if n == nil {
		return nil
	}
	return n.graph
}

// InputBusCount reports how many input buses the node has.
func (n *nodeCommon) InputBusCount() uint32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetInputBusCount(n.handle)
}

// OutputBusCount reports how many output buses the node has.
func (n *nodeCommon) OutputBusCount() uint32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetOutputBusCount(n.handle)
}

// InputChannels reports the channel count of one input bus.
func (n *nodeCommon) InputChannels(inputBusIndex uint32) uint32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetInputChannels(n.handle, inputBusIndex)
}

// OutputChannels reports the channel count of one output bus.
func (n *nodeCommon) OutputChannels(outputBusIndex uint32) uint32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetOutputChannels(n.handle, outputBusIndex)
}

// AttachOutputBus connects one of this node's output buses to an input bus of
// other. It blocks until the audio thread is done with the bus.
func (n *nodeCommon) AttachOutputBus(outputBusIndex uint32, other Node, otherInputBusIndex uint32) error {
	if err := n.ensure(); err != nil {
		return err
	}
	if other == nil {
		return errors.New("mago: cannot attach to a nil node")
	}
	target := other.nodeHandle()
	if target == nil {
		return errNilNode
	}

	return n.lib.resultError("ma_node_attach_output_bus",
		n.lib.bindings.maNodeAttachOutputBus(n.handle, outputBusIndex, target, otherInputBusIndex))
}

// DetachOutputBus disconnects an output bus, blocking until the audio thread
// has finished processing it.
func (n *nodeCommon) DetachOutputBus(outputBusIndex uint32) error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_node_detach_output_bus", n.lib.bindings.maNodeDetachOutputBus(n.handle, outputBusIndex))
}

// DetachAllOutputBuses disconnects every output bus at once.
func (n *nodeCommon) DetachAllOutputBuses() error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_node_detach_all_output_buses", n.lib.bindings.maNodeDetachAllOutputBuses(n.handle))
}

// SetOutputBusVolume sets the gain applied to one output bus.
func (n *nodeCommon) SetOutputBusVolume(outputBusIndex uint32, volume float32) error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_node_set_output_bus_volume",
		n.lib.bindings.maNodeSetOutputBusVolume(n.handle, outputBusIndex, volume))
}

// OutputBusVolume reports the gain applied to one output bus.
func (n *nodeCommon) OutputBusVolume(outputBusIndex uint32) float32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetOutputBusVolume(n.handle, outputBusIndex)
}

// State reports the node's current playback state.
func (n *nodeCommon) State() NodeState {
	if n == nil || n.handle == nil {
		return NodeStateStopped
	}
	return n.lib.bindings.maNodeGetState(n.handle)
}

// SetState starts or stops the node immediately.
func (n *nodeCommon) SetState(state NodeState) error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_node_set_state", n.lib.bindings.maNodeSetState(n.handle, state))
}

// SetStateTime schedules a state change on the graph's global clock.
func (n *nodeCommon) SetStateTime(state NodeState, globalTime uint64) error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_node_set_state_time",
		n.lib.bindings.maNodeSetStateTime(n.handle, state, globalTime))
}

// StateTime reports when a state change is scheduled.
func (n *nodeCommon) StateTime(state NodeState) uint64 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetStateTime(n.handle, state)
}

// StateByTime reports the state the node is in at a global time.
func (n *nodeCommon) StateByTime(globalTime uint64) NodeState {
	if n == nil || n.handle == nil {
		return NodeStateStopped
	}
	return n.lib.bindings.maNodeGetStateByTime(n.handle, globalTime)
}

// StateByTimeRange reports the state over a span of global time.
func (n *nodeCommon) StateByTimeRange(globalTimeBeg, globalTimeEnd uint64) NodeState {
	if n == nil || n.handle == nil {
		return NodeStateStopped
	}
	return n.lib.bindings.maNodeGetStateByTimeRange(n.handle, globalTimeBeg, globalTimeEnd)
}

// Time reports the node's local clock, in frames.
func (n *nodeCommon) Time() uint64 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maNodeGetTime(n.handle)
}

// SetTime sets the node's local clock.
func (n *nodeCommon) SetTime(localTime uint64) error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_node_set_time", n.lib.bindings.maNodeSetTime(n.handle, localTime))
}

// closeNative detaches nothing itself: miniaudio's ma_*_node_uninit already
// detaches every remaining attachment, waiting for the audio thread as needed.
func (n *nodeCommon) closeNative(uninit func(*nodeHandle, unsafe.Pointer)) error {
	if n == nil || n.handle == nil {
		return nil
	}
	if err := n.lib.ensureOpen(); err != nil {
		return err
	}

	handle := n.handle
	n.handle = nil
	uninit(handle, nil)
	n.lib.bindings.magoFree(unsafe.Pointer(handle))
	return nil
}

// endpointNode is the graph's own endpoint. It is borrowed rather than owned,
// so it has no Close method and callers cannot free the graph through it.
type endpointNode struct {
	nodeCommon
}

var _ Node = (*endpointNode)(nil)
