//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"errors"
	"fmt"
	"unsafe"
)

// nodeGraphChannelMismatch reports a node whose channel count does not match the
// graph it is being created in. miniaudio would reject the mismatch later with an
// opaque result code, so this package checks first.
func nodeGraphChannelMismatch(op string, graph *NodeGraph, channels uint32) error {
	if graph == nil || graph.channels == 0 {
		return nil
	}
	if channels != graph.channels {
		return fmt.Errorf("mago: %s needs %d channel(s) to match the node graph, got %d", op, graph.channels, channels)
	}
	return nil
}

// allocateNode reserves the native memory for a node. The caller must free it
// with mago_free if initialisation fails.
func (g *NodeGraph) allocateNode(objectType int32) (*nodeHandle, unsafe.Pointer, error) {
	if err := g.ensure(); err != nil {
		return nil, nil, err
	}

	raw := g.lib.bindings.magoAlloc(objectType)
	if raw == nil {
		return nil, nil, fmt.Errorf("mago: allocate node failed")
	}
	return (*nodeHandle)(raw), raw, nil
}

// SplitterNodeConfig configures a splitter node: a node that copies its single
// input to several identical output buses.
type SplitterNodeConfig struct {
	Channels uint32
	// OutputBusCount is how many copies of the input to produce. It defaults to
	// two, and must not exceed 254.
	OutputBusCount uint32
}

// DefaultSplitterNodeConfig returns a splitter for channels that produces two
// output buses.
func DefaultSplitterNodeConfig(channels uint32) SplitterNodeConfig {
	return SplitterNodeConfig{Channels: channels, OutputBusCount: 2}
}

// BiquadNodeConfig configures a biquad filter node. Node graphs always process
// f32 audio, so there is no format field.
type BiquadNodeConfig struct {
	Channels uint32
	B0       float64
	B1       float64
	B2       float64
	A0       float64
	A1       float64
	A2       float64
}

// FilterNodeConfig configures a low-pass, high-pass or band-pass filter node.
type FilterNodeConfig struct {
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

// NotchNodeConfig configures a notch filter node.
type NotchNodeConfig struct {
	Channels   uint32
	SampleRate uint32
	Q          float64
	Frequency  float64
}

// PeakNodeConfig configures a peaking filter node.
type PeakNodeConfig struct {
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	Q          float64
	Frequency  float64
}

// ShelfNodeConfig configures a low-shelf or high-shelf filter node. ShelfSlope
// is what miniaudio calls the shelf's Q.
type ShelfNodeConfig struct {
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

// ---------------------------------------------------------------------------
// Data source node
// ---------------------------------------------------------------------------

// DataSourceNode feeds a DataSource into a graph. It has no input buses and one
// output bus, and the data source must produce f32 audio: miniaudio rejects
// anything else.
type DataSourceNode struct {
	nodeCommon
	source DataSource
	// wrapper is set when the source was a plain Go DataSource rather than one
	// of this package's native sources. The node owns it and closes it with
	// itself.
	wrapper *CustomDataSource
}

// NewDataSourceNode wraps source in a node. Sources that miniaudio already
// understands (Decoder, AudioBuffer, Waveform, Noise and CustomDataSource) are
// read directly; any other DataSource is registered as a custom data source
// first, and the node closes that registration when it closes.
func (g *NodeGraph) NewDataSourceNode(source DataSource) (*DataSourceNode, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}

	handle, wrapper, err := g.lib.resolveDataSource(source)
	if err != nil {
		return nil, err
	}

	handleNode, raw, err := g.allocateNode(magoObjectDataSourceNode)
	if err != nil {
		if wrapper != nil {
			_ = wrapper.Close()
		}
		return nil, err
	}

	config := dataSourceNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		DataSource: handle,
	}
	if result := g.lib.bindings.maDataSourceNodeInit(g.handle, &config, nil, handleNode); result != Success {
		g.lib.bindings.magoFree(raw)
		if wrapper != nil {
			_ = wrapper.Close()
		}
		return nil, g.lib.resultError("ma_data_source_node_init", result)
	}

	return &DataSourceNode{
		nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handleNode},
		source:     source,
		wrapper:    wrapper,
	}, nil
}

// Source reports the data source the node was created from.
func (n *DataSourceNode) Source() DataSource {
	if n == nil {
		return nil
	}
	return n.source
}

// SetLooping controls whether the data source restarts when it reaches the end.
func (n *DataSourceNode) SetLooping(looping bool) error {
	if err := n.ensure(); err != nil {
		return err
	}
	return n.lib.resultError("ma_data_source_node_set_looping",
		n.lib.bindings.maDataSourceNodeSetLooping(n.handle, boolToBool32(looping)))
}

// IsLooping reports whether the data source restarts when it reaches the end.
func (n *DataSourceNode) IsLooping() bool {
	if n == nil || n.handle == nil {
		return false
	}
	return n.lib.bindings.maDataSourceNodeIsLooping(n.handle) != 0
}

// Close uninitialises the node, and closes the custom data source wrapper if this
// node created one.
func (n *DataSourceNode) Close() error {
	if n == nil {
		return nil
	}
	err := n.closeNative(n.lib.bindings.maDataSourceNodeUninit)
	if n.wrapper != nil {
		_ = n.wrapper.Close()
		n.wrapper = nil
	}
	return err
}

var _ Node = (*DataSourceNode)(nil)

// ---------------------------------------------------------------------------
// Splitter node
// ---------------------------------------------------------------------------

// SplitterNode copies its input to several output buses, so a source can feed
// more than one effect chain.
type SplitterNode struct {
	nodeCommon
}

// NewSplitterNode creates a splitter node.
func (g *NodeGraph) NewSplitterNode(config SplitterNodeConfig) (*SplitterNode, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch("splitter node", g, config.Channels); err != nil {
		return nil, err
	}
	if config.OutputBusCount == 0 {
		return nil, errors.New("mago: splitter node needs at least one output bus")
	}
	if config.OutputBusCount > maxNodeBusCount {
		return nil, fmt.Errorf("mago: splitter node supports at most %d output buses, got %d", maxNodeBusCount, config.OutputBusCount)
	}

	handle, raw, err := g.allocateNode(magoObjectSplitterNode)
	if err != nil {
		return nil, err
	}

	native := splitterNodeConfigNative{
		NodeConfig:     defaultNodeConfig(),
		Channels:       config.Channels,
		OutputBusCount: config.OutputBusCount,
	}
	if result := g.lib.bindings.maSplitterNodeInit(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError("ma_splitter_node_init", result)
	}

	return &SplitterNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// Close uninitialises the node and frees its native memory.
func (n *SplitterNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maSplitterNodeUninit)
}

var _ Node = (*SplitterNode)(nil)

// ---------------------------------------------------------------------------
// Biquad node
// ---------------------------------------------------------------------------

// BiquadNode applies a raw biquad filter to the graph.
type BiquadNode struct {
	nodeCommon
}

// NewBiquadNode creates a biquad filter node.
func (g *NodeGraph) NewBiquadNode(config BiquadNodeConfig) (*BiquadNode, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch("biquad node", g, config.Channels); err != nil {
		return nil, err
	}

	handle, raw, err := g.allocateNode(magoObjectBiquadNode)
	if err != nil {
		return nil, err
	}

	native := biquadNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		Biquad:     biquadNodeFilterConfig(config),
	}
	if result := g.lib.bindings.maBiquadNodeInit(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError("ma_biquad_node_init", result)
	}

	return &BiquadNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

func biquadNodeFilterConfig(config BiquadNodeConfig) biquadConfigNative {
	return biquadConfigNative{
		Format:   FormatF32,
		Channels: config.Channels,
		B0:       config.B0,
		B1:       config.B1,
		B2:       config.B2,
		A0:       config.A0,
		A1:       config.A1,
		A2:       config.A2,
	}
}

// Reinit changes the filter coefficients without reallocating or routing the
// node out of the graph.
func (n *BiquadNode) Reinit(config BiquadNodeConfig) error {
	if err := n.ensure(); err != nil {
		return err
	}
	native := biquadNodeFilterConfig(config)
	return n.lib.resultError("ma_biquad_node_reinit", n.lib.bindings.maBiquadNodeReinit(&native, n.handle))
}

// Close uninitialises the node and frees its native memory.
func (n *BiquadNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maBiquadNodeUninit)
}

var _ Node = (*BiquadNode)(nil)

// ---------------------------------------------------------------------------
// Low-pass, high-pass and band-pass filter nodes
// ---------------------------------------------------------------------------

// LowPassNode attenuates frequencies above its cutoff.
type LowPassNode struct {
	nodeCommon
}

// HighPassNode attenuates frequencies below its cutoff.
type HighPassNode struct {
	nodeCommon
}

// BandPassNode attenuates frequencies outside a band around its cutoff.
type BandPassNode struct {
	nodeCommon
}

// NewLowPassNode creates a low-pass filter node.
func (g *NodeGraph) NewLowPassNode(config FilterNodeConfig) (*LowPassNode, error) {
	handle, err := g.newFilterNode("low-pass node", magoObjectLPFNode, config, g.lib.bindings.maLPFNodeInit)
	if err != nil {
		return nil, err
	}
	return &LowPassNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// NewHighPassNode creates a high-pass filter node.
func (g *NodeGraph) NewHighPassNode(config FilterNodeConfig) (*HighPassNode, error) {
	handle, err := g.newFilterNode("high-pass node", magoObjectHPFNode, config, g.lib.bindings.maHPFNodeInit)
	if err != nil {
		return nil, err
	}
	return &HighPassNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// NewBandPassNode creates a band-pass filter node.
func (g *NodeGraph) NewBandPassNode(config FilterNodeConfig) (*BandPassNode, error) {
	handle, err := g.newFilterNode("band-pass node", magoObjectBPFNode, config, g.lib.bindings.maBPFNodeInit)
	if err != nil {
		return nil, err
	}
	return &BandPassNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

func (g *NodeGraph) newFilterNode(
	op string,
	objectType int32,
	config FilterNodeConfig,
	init func(*nodeGraphHandle, *lpfNodeConfigNative, unsafe.Pointer, *nodeHandle) Result,
) (*nodeHandle, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch(op, g, config.Channels); err != nil {
		return nil, err
	}
	if config.Order == 0 {
		return nil, fmt.Errorf("mago: %s needs an order of at least 1", op)
	}

	handle, raw, err := g.allocateNode(objectType)
	if err != nil {
		return nil, err
	}

	native := lpfNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		LPF:        filterNodeFilterConfig(config),
	}
	if result := init(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError(op, result)
	}
	return handle, nil
}

func filterNodeFilterConfig(config FilterNodeConfig) lpfConfigNative {
	return lpfConfigNative{
		Format:          FormatF32,
		Channels:        config.Channels,
		SampleRate:      config.SampleRate,
		CutoffFrequency: config.CutoffFrequency,
		Order:           config.Order,
	}
}

// Reinit changes the cutoff and order without reallocating or rerouting.
func (n *LowPassNode) Reinit(config FilterNodeConfig) error {
	if n == nil {
		return nil
	}
	return n.reinitFilter("ma_lpf_node_reinit", config, n.lib.bindings.maLPFNodeReinit)
}

// Close uninitialises the node and frees its native memory.
func (n *LowPassNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maLPFNodeUninit)
}

// Reinit changes the cutoff and order without reallocating or rerouting.
func (n *HighPassNode) Reinit(config FilterNodeConfig) error {
	if n == nil {
		return nil
	}
	return n.reinitFilter("ma_hpf_node_reinit", config, n.lib.bindings.maHPFNodeReinit)
}

// Close uninitialises the node and frees its native memory.
func (n *HighPassNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maHPFNodeUninit)
}

// Reinit changes the cutoff and order without reallocating or rerouting.
func (n *BandPassNode) Reinit(config FilterNodeConfig) error {
	if n == nil {
		return nil
	}
	return n.reinitFilter("ma_bpf_node_reinit", config, n.lib.bindings.maBPFNodeReinit)
}

// Close uninitialises the node and frees its native memory.
func (n *BandPassNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maBPFNodeUninit)
}

func (n *nodeCommon) reinitFilter(
	op string,
	config FilterNodeConfig,
	reinit func(*lpfConfigNative, *nodeHandle) Result,
) error {
	if err := n.ensure(); err != nil {
		return err
	}
	native := filterNodeFilterConfig(config)
	return n.lib.resultError(op, reinit(&native, n.handle))
}

var (
	_ Node = (*LowPassNode)(nil)
	_ Node = (*HighPassNode)(nil)
	_ Node = (*BandPassNode)(nil)
)

// ---------------------------------------------------------------------------
// Notch, peak and shelf nodes
// ---------------------------------------------------------------------------

// NotchNode attenuates a narrow band around its centre frequency.
type NotchNode struct {
	nodeCommon
}

// PeakNode boosts or cuts a narrow band around its centre frequency.
type PeakNode struct {
	nodeCommon
}

// LowShelfNode boosts or cuts everything below its corner frequency.
type LowShelfNode struct {
	nodeCommon
}

// HighShelfNode boosts or cuts everything above its corner frequency.
type HighShelfNode struct {
	nodeCommon
}

// NewNotchNode creates a notch filter node.
func (g *NodeGraph) NewNotchNode(config NotchNodeConfig) (*NotchNode, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch("notch node", g, config.Channels); err != nil {
		return nil, err
	}

	handle, raw, err := g.allocateNode(magoObjectNotchNode)
	if err != nil {
		return nil, err
	}

	native := notchNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		Notch: notch2ConfigNative{
			Format:     FormatF32,
			Channels:   config.Channels,
			SampleRate: config.SampleRate,
			Q:          config.Q,
			Frequency:  config.Frequency,
		},
	}
	if result := g.lib.bindings.maNotchNodeInit(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError("ma_notch_node_init", result)
	}

	return &NotchNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// Reinit changes the filter without reallocating or rerouting.
func (n *NotchNode) Reinit(config NotchNodeConfig) error {
	if err := n.ensure(); err != nil {
		return err
	}
	native := notch2ConfigNative{
		Format:     FormatF32,
		Channels:   config.Channels,
		SampleRate: config.SampleRate,
		Q:          config.Q,
		Frequency:  config.Frequency,
	}
	return n.lib.resultError("ma_notch_node_reinit", n.lib.bindings.maNotchNodeReinit(&native, n.handle))
}

// Close uninitialises the node and frees its native memory.
func (n *NotchNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maNotchNodeUninit)
}

// NewPeakNode creates a peaking filter node.
func (g *NodeGraph) NewPeakNode(config PeakNodeConfig) (*PeakNode, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch("peak node", g, config.Channels); err != nil {
		return nil, err
	}

	handle, raw, err := g.allocateNode(magoObjectPeakNode)
	if err != nil {
		return nil, err
	}

	native := peakNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		Peak: peak2ConfigNative{
			Format:     FormatF32,
			Channels:   config.Channels,
			SampleRate: config.SampleRate,
			GainDB:     config.GainDB,
			Q:          config.Q,
			Frequency:  config.Frequency,
		},
	}
	if result := g.lib.bindings.maPeakNodeInit(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError("ma_peak_node_init", result)
	}

	return &PeakNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// Reinit changes the filter without reallocating or rerouting.
func (n *PeakNode) Reinit(config PeakNodeConfig) error {
	if err := n.ensure(); err != nil {
		return err
	}
	native := peak2ConfigNative{
		Format:     FormatF32,
		Channels:   config.Channels,
		SampleRate: config.SampleRate,
		GainDB:     config.GainDB,
		Q:          config.Q,
		Frequency:  config.Frequency,
	}
	return n.lib.resultError("ma_peak_node_reinit", n.lib.bindings.maPeakNodeReinit(&native, n.handle))
}

// Close uninitialises the node and frees its native memory.
func (n *PeakNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maPeakNodeUninit)
}

// NewLowShelfNode creates a low-shelf filter node.
func (g *NodeGraph) NewLowShelfNode(config ShelfNodeConfig) (*LowShelfNode, error) {
	handle, err := g.newShelfNode("low-shelf node", magoObjectLoShelfNode, config, g.lib.bindings.maLoShelfNodeInit)
	if err != nil {
		return nil, err
	}
	return &LowShelfNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// NewHighShelfNode creates a high-shelf filter node.
func (g *NodeGraph) NewHighShelfNode(config ShelfNodeConfig) (*HighShelfNode, error) {
	handle, err := g.newShelfNode("high-shelf node", magoObjectHiShelfNode, config, g.lib.bindings.maHiShelfNodeInit)
	if err != nil {
		return nil, err
	}
	return &HighShelfNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

func (g *NodeGraph) newShelfNode(
	op string,
	objectType int32,
	config ShelfNodeConfig,
	init func(*nodeGraphHandle, *loshelfNodeConfigNative, unsafe.Pointer, *nodeHandle) Result,
) (*nodeHandle, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch(op, g, config.Channels); err != nil {
		return nil, err
	}

	handle, raw, err := g.allocateNode(objectType)
	if err != nil {
		return nil, err
	}

	native := loshelfNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		LoShelf:    shelfNodeFilterConfig(config),
	}
	if result := init(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError(op, result)
	}
	return handle, nil
}

func shelfNodeFilterConfig(config ShelfNodeConfig) loshelf2ConfigNative {
	return loshelf2ConfigNative{
		Format:     FormatF32,
		Channels:   config.Channels,
		SampleRate: config.SampleRate,
		GainDB:     config.GainDB,
		ShelfSlope: config.ShelfSlope,
		Frequency:  config.Frequency,
	}
}

// Reinit changes the shelf without reallocating or rerouting.
func (n *LowShelfNode) Reinit(config ShelfNodeConfig) error {
	if n == nil {
		return nil
	}
	return n.reinitShelf("ma_loshelf_node_reinit", config, n.lib.bindings.maLoShelfNodeReinit)
}

// Close uninitialises the node and frees its native memory.
func (n *LowShelfNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maLoShelfNodeUninit)
}

// Reinit changes the shelf without reallocating or rerouting.
func (n *HighShelfNode) Reinit(config ShelfNodeConfig) error {
	if n == nil {
		return nil
	}
	return n.reinitShelf("ma_hishelf_node_reinit", config, n.lib.bindings.maHiShelfNodeReinit)
}

// Close uninitialises the node and frees its native memory.
func (n *HighShelfNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maHiShelfNodeUninit)
}

func (n *nodeCommon) reinitShelf(
	op string,
	config ShelfNodeConfig,
	reinit func(*loshelf2ConfigNative, *nodeHandle) Result,
) error {
	if err := n.ensure(); err != nil {
		return err
	}
	native := shelfNodeFilterConfig(config)
	return n.lib.resultError(op, reinit(&native, n.handle))
}

var (
	_ Node = (*NotchNode)(nil)
	_ Node = (*PeakNode)(nil)
	_ Node = (*LowShelfNode)(nil)
	_ Node = (*HighShelfNode)(nil)
)

// ---------------------------------------------------------------------------
// Delay node
// ---------------------------------------------------------------------------

// DelayNode is a delay line that lives in the graph. miniaudio keeps it
// processing continuously, so the tail of a delay is rendered even after its
// input goes silent.
type DelayNode struct {
	nodeCommon
}

// NewDelayNode creates a delay node. Use DefaultDelayConfig to get miniaudio's
// defaults.
func (g *NodeGraph) NewDelayNode(config DelayConfig) (*DelayNode, error) {
	if err := g.ensure(); err != nil {
		return nil, err
	}
	if err := nodeGraphChannelMismatch("delay node", g, config.Channels); err != nil {
		return nil, err
	}

	var delayStart uint32
	if config.DelayStart {
		delayStart = 1
	}

	handle, raw, err := g.allocateNode(magoObjectDelayNode)
	if err != nil {
		return nil, err
	}

	native := delayNodeConfigNative{
		NodeConfig: defaultNodeConfig(),
		Delay: delayConfigNative{
			Channels:      config.Channels,
			SampleRate:    config.SampleRate,
			DelayInFrames: config.DelayInFrames,
			DelayStart:    delayStart,
			Wet:           config.Wet,
			Dry:           config.Dry,
			Decay:         config.Decay,
		},
	}
	if result := g.lib.bindings.maDelayNodeInit(g.handle, &native, nil, handle); result != Success {
		g.lib.bindings.magoFree(raw)
		return nil, g.lib.resultError("ma_delay_node_init", result)
	}

	return &DelayNode{nodeCommon: nodeCommon{lib: g.lib, graph: g, handle: handle}}, nil
}

// Wet returns the wet mix factor (0.0 to 1.0).
func (n *DelayNode) Wet() float32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maDelayNodeGetWet(n.handle)
}

// SetWet sets the wet mix factor (0.0 to 1.0).
func (n *DelayNode) SetWet(wet float32) error {
	if err := n.ensure(); err != nil {
		return err
	}
	n.lib.bindings.maDelayNodeSetWet(n.handle, wet)
	return nil
}

// Dry returns the dry mix factor (0.0 to 1.0).
func (n *DelayNode) Dry() float32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maDelayNodeGetDry(n.handle)
}

// SetDry sets the dry mix factor (0.0 to 1.0).
func (n *DelayNode) SetDry(dry float32) error {
	if err := n.ensure(); err != nil {
		return err
	}
	n.lib.bindings.maDelayNodeSetDry(n.handle, dry)
	return nil
}

// Decay returns the feedback decay factor (0.0 to 1.0).
func (n *DelayNode) Decay() float32 {
	if n == nil || n.handle == nil {
		return 0
	}
	return n.lib.bindings.maDelayNodeGetDecay(n.handle)
}

// SetDecay sets the feedback decay factor (0.0 to 1.0).
func (n *DelayNode) SetDecay(decay float32) error {
	if err := n.ensure(); err != nil {
		return err
	}
	n.lib.bindings.maDelayNodeSetDecay(n.handle, decay)
	return nil
}

// Close uninitialises the node and frees its native memory.
func (n *DelayNode) Close() error {
	if n == nil {
		return nil
	}
	return n.closeNative(n.lib.bindings.maDelayNodeUninit)
}

var _ Node = (*DelayNode)(nil)
