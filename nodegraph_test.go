package mago

import (
	"encoding/binary"
	"math"
	"testing"
)

// newNullNodeGraph creates a node graph on the null backend and registers
// cleanup. Cleanup runs last-in-first-out, so anything registered afterwards
// (nodes) is torn down before the graph.
func newNullNodeGraph(t *testing.T, channels uint32) *NodeGraph {
	t.Helper()

	lib := newNullLibrary(t)
	graph, err := lib.NewNodeGraph(NodeGraphConfig{Channels: channels})
	if err != nil {
		t.Fatalf("NewNodeGraph: %v", err)
	}
	t.Cleanup(func() {
		if err := graph.Close(); err != nil {
			t.Errorf("close node graph: %v", err)
		}
	})
	return graph
}

// newSineSourceNode builds a 1000 Hz sine waveform and wraps it in a data source
// node. The waveform is registered for cleanup before the node, so the node is
// closed first and the waveform is never freed while the graph still points at
// it.
func newSineSourceNode(t *testing.T, graph *NodeGraph, channels uint32) *DataSourceNode {
	t.Helper()

	waveform, err := graph.lib.NewWaveform(WaveformConfig{
		Format:     FormatF32,
		Channels:   channels,
		SampleRate: 48000,
		Type:       WaveformTypeSine,
		Amplitude:  1.0,
		Frequency:  1000.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	t.Cleanup(func() {
		if err := waveform.Close(); err != nil {
			t.Errorf("close waveform: %v", err)
		}
	})

	node, err := graph.NewDataSourceNode(waveform)
	if err != nil {
		t.Fatalf("NewDataSourceNode: %v", err)
	}
	t.Cleanup(func() {
		if err := node.Close(); err != nil {
			t.Errorf("close data source node: %v", err)
		}
	})
	return node
}

// readNodeGraphPeak reads frames frames from the graph and reports the peak
// amplitude across every channel.
func readNodeGraphPeak(t *testing.T, graph *NodeGraph, frames int) float64 {
	t.Helper()

	samples := make([]float32, frames*int(graph.Channels()))
	read, err := graph.Read(samples)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if int(read) != frames {
		t.Fatalf("read %d frames, want %d", read, frames)
	}

	peak := 0.0
	for _, sample := range samples {
		if magnitude := math.Abs(float64(sample)); magnitude > peak {
			peak = magnitude
		}
	}
	return peak
}

func TestNodeGraphSourceToEndpoint(t *testing.T) {
	const channels = 2
	graph := newNullNodeGraph(t, channels)
	source := newSineSourceNode(t, graph, channels)

	if err := source.AttachOutputBus(0, graph.Endpoint(), 0); err != nil {
		t.Fatalf("AttachOutputBus: %v", err)
	}

	if got := source.InputBusCount(); got != 0 {
		t.Errorf("source node input buses = %d, want 0", got)
	}
	if got := source.OutputBusCount(); got != 1 {
		t.Errorf("source node output buses = %d, want 1", got)
	}
	if got := source.OutputChannels(0); got != channels {
		t.Errorf("source node output channels = %d, want %d", got, channels)
	}
	if got := graph.Endpoint().InputBusCount(); got != 1 {
		t.Errorf("endpoint input buses = %d, want 1", got)
	}

	peak := readNodeGraphPeak(t, graph, 480)
	if peak < 0.9 || peak > 1.01 {
		t.Errorf("direct graph peak = %f, want ~1.0", peak)
	}
}

func TestNodeGraphBiquadAttenuatesSignal(t *testing.T) {
	const channels = 1

	direct := newNullNodeGraph(t, channels)
	directSource := newSineSourceNode(t, direct, channels)
	if err := directSource.AttachOutputBus(0, direct.Endpoint(), 0); err != nil {
		t.Fatalf("attach direct source: %v", err)
	}
	directPeak := readNodeGraphPeak(t, direct, 480)

	filtered := newNullNodeGraph(t, channels)
	filteredSource := newSineSourceNode(t, filtered, channels)
	biquad, err := filtered.NewBiquadNode(BiquadNodeConfig{
		Channels: channels,
		// A three-tap average: a very heavy low-pass at any audio frequency.
		B0: 0.01, B1: 0.01, B2: 0.01,
		A0: 1, A1: 0, A2: 0,
	})
	if err != nil {
		t.Fatalf("NewBiquadNode: %v", err)
	}
	t.Cleanup(func() {
		if err := biquad.Close(); err != nil {
			t.Errorf("close biquad node: %v", err)
		}
	})

	if err := filteredSource.AttachOutputBus(0, biquad, 0); err != nil {
		t.Fatalf("attach source to biquad: %v", err)
	}
	if err := biquad.AttachOutputBus(0, filtered.Endpoint(), 0); err != nil {
		t.Fatalf("attach biquad to endpoint: %v", err)
	}

	filteredPeak := readNodeGraphPeak(t, filtered, 480)
	if filteredPeak >= directPeak/10 {
		t.Errorf("filtered peak = %f, want well below the direct peak of %f", filteredPeak, directPeak)
	}

	// Reinit with pass-through coefficients and confirm the signal returns.
	if err := biquad.Reinit(BiquadNodeConfig{Channels: channels, B0: 1, A0: 1}); err != nil {
		t.Fatalf("Reinit: %v", err)
	}
	passThroughPeak := readNodeGraphPeak(t, filtered, 480)
	if passThroughPeak < 0.9 {
		t.Errorf("pass-through peak = %f, want ~1.0", passThroughPeak)
	}
}

func TestNodeGraphSplitterFeedsBothBranches(t *testing.T) {
	const channels = 1

	direct := newNullNodeGraph(t, channels)
	directSource := newSineSourceNode(t, direct, channels)
	if err := directSource.AttachOutputBus(0, direct.Endpoint(), 0); err != nil {
		t.Fatalf("attach direct source: %v", err)
	}
	directPeak := readNodeGraphPeak(t, direct, 480)

	split := newNullNodeGraph(t, channels)
	splitSource := newSineSourceNode(t, split, channels)
	splitter, err := split.NewSplitterNode(DefaultSplitterNodeConfig(channels))
	if err != nil {
		t.Fatalf("NewSplitterNode: %v", err)
	}
	t.Cleanup(func() {
		if err := splitter.Close(); err != nil {
			t.Errorf("close splitter node: %v", err)
		}
	})

	if got := splitter.OutputBusCount(); got != 2 {
		t.Fatalf("splitter output buses = %d, want 2", got)
	}
	if got := splitter.InputBusCount(); got != 1 {
		t.Fatalf("splitter input buses = %d, want 1", got)
	}

	if err := splitSource.AttachOutputBus(0, splitter, 0); err != nil {
		t.Fatalf("attach source to splitter: %v", err)
	}
	// Both splitter outputs land on the endpoint's single input bus, so the
	// endpoint sums two identical copies of the input.
	for bus := uint32(0); bus < 2; bus++ {
		if err := splitter.AttachOutputBus(bus, split.Endpoint(), 0); err != nil {
			t.Fatalf("attach splitter bus %d: %v", bus, err)
		}
	}

	splitPeak := readNodeGraphPeak(t, split, 480)
	if splitPeak < directPeak*1.8 {
		t.Errorf("split peak = %f, want about twice the direct peak of %f", splitPeak, directPeak)
	}
}

func TestNodeGraphDelayTailAccumulates(t *testing.T) {
	const (
		channels    = 1
		delayFrames = 240
	)

	graph := newNullNodeGraph(t, channels)
	source := newSineSourceNode(t, graph, channels)

	// A 240-frame delay is exactly five periods of the 1000 Hz tone, so the echo
	// lands in phase and each lap of the delay line adds decay times the tone
	// again: 1.0, then 1.5, then 1.75 at the peak.
	delay, err := graph.NewDelayNode(DefaultDelayConfig(channels, 48000, delayFrames, 0.5))
	if err != nil {
		t.Fatalf("NewDelayNode: %v", err)
	}
	t.Cleanup(func() {
		if err := delay.Close(); err != nil {
			t.Errorf("close delay node: %v", err)
		}
	})

	if err := source.AttachOutputBus(0, delay, 0); err != nil {
		t.Fatalf("attach source to delay: %v", err)
	}
	if err := delay.AttachOutputBus(0, graph.Endpoint(), 0); err != nil {
		t.Fatalf("attach delay to endpoint: %v", err)
	}

	for i, want := range []struct {
		low  float64
		high float64
	}{
		{0.95, 1.01},
		{1.45, 1.55},
		{1.70, 1.80},
	} {
		peak := readNodeGraphPeak(t, graph, delayFrames)
		if peak < want.low || peak > want.high {
			t.Errorf("window %d peak = %f, want between %f and %f", i, peak, want.low, want.high)
		}
	}

	if err := delay.SetWet(0.5); err != nil {
		t.Fatalf("SetWet: %v", err)
	}
	if err := delay.SetDry(0.5); err != nil {
		t.Fatalf("SetDry: %v", err)
	}
	if err := delay.SetDecay(0.25); err != nil {
		t.Fatalf("SetDecay: %v", err)
	}
}

func TestNodeGraphOutputBusVolume(t *testing.T) {
	const channels = 1

	graph := newNullNodeGraph(t, channels)
	source := newSineSourceNode(t, graph, channels)
	if err := source.AttachOutputBus(0, graph.Endpoint(), 0); err != nil {
		t.Fatalf("AttachOutputBus: %v", err)
	}

	if err := source.SetOutputBusVolume(0, 0); err != nil {
		t.Fatalf("SetOutputBusVolume: %v", err)
	}
	if got := source.OutputBusVolume(0); got != 0 {
		t.Errorf("output bus volume = %f, want 0", got)
	}
	if peak := readNodeGraphPeak(t, graph, 480); peak != 0 {
		t.Errorf("muted graph peak = %f, want 0", peak)
	}

	if err := source.SetOutputBusVolume(0, 0.5); err != nil {
		t.Fatalf("SetOutputBusVolume: %v", err)
	}
	if peak := readNodeGraphPeak(t, graph, 480); peak < 0.45 || peak > 0.51 {
		t.Errorf("half-volume graph peak = %f, want ~0.5", peak)
	}
}

func TestNodeGraphStateAndTime(t *testing.T) {
	const channels = 1

	graph := newNullNodeGraph(t, channels)
	source := newSineSourceNode(t, graph, channels)
	if err := source.AttachOutputBus(0, graph.Endpoint(), 0); err != nil {
		t.Fatalf("AttachOutputBus: %v", err)
	}

	if got := source.State(); got != NodeStateStarted {
		t.Errorf("initial state = %d, want NodeStateStarted", got)
	}

	if err := source.SetStateTime(NodeStateStopped, 240); err != nil {
		t.Fatalf("SetStateTime: %v", err)
	}
	if got := source.StateTime(NodeStateStopped); got != 240 {
		t.Errorf("scheduled stop time = %d, want 240", got)
	}
	if got := source.StateByTime(100); got != NodeStateStarted {
		t.Errorf("state at frame 100 = %d, want NodeStateStarted", got)
	}
	if got := source.StateByTime(480); got != NodeStateStopped {
		t.Errorf("state at frame 480 = %d, want NodeStateStopped", got)
	}

	// Reading advances the graph's global clock, and a stopped node stops
	// producing signal.
	readNodeGraphPeak(t, graph, 480)
	if got := graph.Time(); got != 480 {
		t.Errorf("graph time after reading 480 frames = %d, want 480", got)
	}
	if peak := readNodeGraphPeak(t, graph, 480); peak != 0 {
		t.Errorf("peak after scheduling a stop = %f, want 0", peak)
	}

	if err := source.SetState(NodeStateStarted); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	if got := source.State(); got != NodeStateStarted {
		t.Errorf("state after SetState(started) = %d, want NodeStateStarted", got)
	}

	if err := source.SetTime(0); err != nil {
		t.Fatalf("SetTime: %v", err)
	}
	if got := source.Time(); got != 0 {
		t.Errorf("node local time = %d, want 0", got)
	}

	if err := graph.SetTime(96); err != nil {
		t.Fatalf("SetTime: %v", err)
	}
	if got := graph.Time(); got != 96 {
		t.Errorf("graph time after SetTime = %d, want 96", got)
	}
}

func TestNodeGraphIntrospection(t *testing.T) {
	graph := newNullNodeGraph(t, 2)

	if got := graph.Channels(); got != 2 {
		t.Errorf("Channels = %d, want 2", got)
	}
	if got := graph.ProcessingSizeInFrames(); got != 0 {
		t.Errorf("ProcessingSizeInFrames = %d, want 0", got)
	}
	if got := graph.Endpoint().Graph(); got != graph {
		t.Errorf("endpoint graph = %p, want %p", got, graph)
	}
	if got := graph.Endpoint().OutputChannels(0); got != 2 {
		t.Errorf("endpoint output channels = %d, want 2", got)
	}
	if got := graph.Endpoint().InputBusCount(); got != 1 {
		t.Errorf("endpoint input buses = %d, want 1", got)
	}
	if got := graph.Endpoint().OutputBusCount(); got != 1 {
		t.Errorf("endpoint output buses = %d, want 1", got)
	}

	// A graph that requests a fixed block size reports it back.
	paged := newNullNodeGraph(t, 2)
	_ = paged.Close()

	lib := newNullLibrary(t)
	sized, err := lib.NewNodeGraph(NodeGraphConfig{Channels: 1, ProcessingSizeInFrames: 128})
	if err != nil {
		t.Fatalf("NewNodeGraph: %v", err)
	}
	t.Cleanup(func() {
		if err := sized.Close(); err != nil {
			t.Errorf("close node graph: %v", err)
		}
	})
	if got := sized.ProcessingSizeInFrames(); got != 128 {
		t.Errorf("ProcessingSizeInFrames = %d, want 128", got)
	}
}

func TestNodeGraphRejectsMismatchedChannelCount(t *testing.T) {
	graph := newNullNodeGraph(t, 2)

	if _, err := graph.NewSplitterNode(SplitterNodeConfig{Channels: 1, OutputBusCount: 2}); err == nil {
		t.Error("NewSplitterNode with 1 channel accepted a 2-channel graph")
	}
	if _, err := graph.NewBiquadNode(BiquadNodeConfig{Channels: 1, B0: 1, A0: 1}); err == nil {
		t.Error("NewBiquadNode with 1 channel accepted a 2-channel graph")
	}
	if _, err := graph.NewLowPassNode(FilterNodeConfig{Channels: 1, SampleRate: 48000, CutoffFrequency: 1000, Order: 1}); err == nil {
		t.Error("NewLowPassNode with 1 channel accepted a 2-channel graph")
	}
	if _, err := graph.NewDelayNode(DefaultDelayConfig(1, 48000, 240, 0)); err == nil {
		t.Error("NewDelayNode with 1 channel accepted a 2-channel graph")
	}

	if _, err := graph.NewSplitterNode(SplitterNodeConfig{Channels: 2}); err == nil {
		t.Error("NewSplitterNode with no output buses was accepted")
	}
	if _, err := graph.NewLowPassNode(FilterNodeConfig{Channels: 2, SampleRate: 48000, CutoffFrequency: 1000}); err == nil {
		t.Error("NewLowPassNode with order 0 was accepted")
	}
}

// s16Source is a DataSource that reports a non-f32 format, which a node graph
// cannot consume.
type s16Source struct{}

func (s16Source) DataFormat() (Format, uint32, uint32, error) { return FormatS16, 1, 48000, nil }
func (s16Source) ReadPCMFrames([]byte) (uint64, error)        { return 0, nil }
func (s16Source) SeekToPCMFrame(uint64) error                 { return nil }
func (s16Source) CursorInPCMFrames() (uint64, error)          { return 0, nil }
func (s16Source) LengthInPCMFrames() (uint64, error)          { return 0, nil }
func (s16Source) SetLooping(bool) error                       { return nil }

func TestDataSourceNodeRejectsNonFloatSource(t *testing.T) {
	graph := newNullNodeGraph(t, 1)

	if _, err := graph.NewDataSourceNode(s16Source{}); err == nil {
		t.Error("NewDataSourceNode accepted an s16 data source")
	}
	if _, err := graph.NewDataSourceNode(nil); err == nil {
		t.Error("NewDataSourceNode accepted a nil data source")
	}
}

// countingSource is a Go-only DataSource, so a node graph has to register it
// through the bridge rather than reading a native object.
type countingSource struct {
	frames uint64
	sample uint64
}

func (s *countingSource) DataFormat() (Format, uint32, uint32, error) {
	return FormatF32, 1, 48000, nil
}

func (s *countingSource) ReadPCMFrames(out []byte) (uint64, error) {
	remaining := s.frames - s.sample
	if remaining == 0 {
		return 0, nil
	}
	frames := uint64(len(out)) / 4
	if frames > remaining {
		frames = remaining
	}
	for i := uint64(0); i < frames; i++ {
		value := float32(0.5)
		if (s.sample+i)%2 == 1 {
			value = -0.5
		}
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(value))
	}
	s.sample += frames
	return frames, nil
}

func (s *countingSource) SeekToPCMFrame(frameIndex uint64) error {
	s.sample = frameIndex
	return nil
}

func (s *countingSource) CursorInPCMFrames() (uint64, error) { return s.sample, nil }
func (s *countingSource) LengthInPCMFrames() (uint64, error) { return s.frames, nil }
func (s *countingSource) SetLooping(bool) error              { return nil }

func TestDataSourceNodeWrapsGoSource(t *testing.T) {
	graph := newNullNodeGraph(t, 1)
	source := &countingSource{frames: 480}

	node, err := graph.NewDataSourceNode(source)
	if err != nil {
		t.Fatalf("NewDataSourceNode: %v", err)
	}
	t.Cleanup(func() {
		if err := node.Close(); err != nil {
			t.Errorf("close data source node: %v", err)
		}
	})

	if got := node.Source(); got != DataSource(source) {
		t.Errorf("Source = %v, want the registered Go source", got)
	}
	if err := node.SetLooping(true); err != nil {
		t.Fatalf("SetLooping: %v", err)
	}
	if !node.IsLooping() {
		t.Error("IsLooping = false after SetLooping(true)")
	}

	if err := node.AttachOutputBus(0, graph.Endpoint(), 0); err != nil {
		t.Fatalf("AttachOutputBus: %v", err)
	}
	if peak := readNodeGraphPeak(t, graph, 480); peak < 0.45 || peak > 0.51 {
		t.Errorf("graph peak from Go source = %f, want ~0.5", peak)
	}

	// Closing the node must also release the custom data source registration it
	// created, so a second close is a no-op rather than a double free.
	if err := node.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestNodeGraphConstructionOfEveryNodeType(t *testing.T) {
	const channels = 2
	graph := newNullNodeGraph(t, channels)

	type built struct {
		name string
		node Node
	}
	nodes := []built{}
	add := func(name string, node Node, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("new %s: %v", name, err)
		}
		nodes = append(nodes, built{name: name, node: node})
	}

	source := newSineSourceNode(t, graph, channels)
	nodes = append(nodes, built{name: "data source", node: source})

	splitter, err := graph.NewSplitterNode(DefaultSplitterNodeConfig(channels))
	add("splitter", splitter, err)
	biquad, err := graph.NewBiquadNode(BiquadNodeConfig{Channels: channels, B0: 1, A0: 1})
	add("biquad", biquad, err)
	lpf, err := graph.NewLowPassNode(FilterNodeConfig{Channels: channels, SampleRate: 48000, CutoffFrequency: 1000, Order: 2})
	add("low pass", lpf, err)
	hpf, err := graph.NewHighPassNode(FilterNodeConfig{Channels: channels, SampleRate: 48000, CutoffFrequency: 4000, Order: 2})
	add("high pass", hpf, err)
	bpf, err := graph.NewBandPassNode(FilterNodeConfig{Channels: channels, SampleRate: 48000, CutoffFrequency: 2000, Order: 2})
	add("band pass", bpf, err)
	notch, err := graph.NewNotchNode(NotchNodeConfig{Channels: channels, SampleRate: 48000, Q: 0.5, Frequency: 1000})
	add("notch", notch, err)
	peak, err := graph.NewPeakNode(PeakNodeConfig{Channels: channels, SampleRate: 48000, GainDB: 6, Q: 0.5, Frequency: 1000})
	add("peak", peak, err)
	loshelf, err := graph.NewLowShelfNode(ShelfNodeConfig{Channels: channels, SampleRate: 48000, GainDB: 6, ShelfSlope: 0.5, Frequency: 500})
	add("low shelf", loshelf, err)
	hishelf, err := graph.NewHighShelfNode(ShelfNodeConfig{Channels: channels, SampleRate: 48000, GainDB: 6, ShelfSlope: 0.5, Frequency: 5000})
	add("high shelf", hishelf, err)
	delay, err := graph.NewDelayNode(DefaultDelayConfig(channels, 48000, 240, 0))
	add("delay", delay, err)

	for _, entry := range nodes {
		if got := entry.node.Graph(); got != graph {
			t.Errorf("%s node graph = %p, want %p", entry.name, got, graph)
		}
		switch entry.name {
		case "data source":
			// A data source node has no inputs.
			if got := entry.node.InputBusCount(); got != 0 {
				t.Errorf("%s node input buses = %d, want 0", entry.name, got)
			}
		case "splitter":
			// A splitter is the only node here with more than one output.
			if got := entry.node.InputBusCount(); got != 1 {
				t.Errorf("%s node input buses = %d, want 1", entry.name, got)
			}
			if got := entry.node.OutputBusCount(); got != 2 {
				t.Errorf("%s node output buses = %d, want 2", entry.name, got)
			}
		default:
			if got := entry.node.InputBusCount(); got != 1 {
				t.Errorf("%s node input buses = %d, want 1", entry.name, got)
			}
			if got := entry.node.OutputBusCount(); got != 1 {
				t.Errorf("%s node output buses = %d, want 1", entry.name, got)
			}
		}
	}

	// Detach and reattach: routing changes must be safe on a live graph.
	if err := source.AttachOutputBus(0, biquad, 0); err != nil {
		t.Fatalf("attach source to biquad: %v", err)
	}
	if err := source.DetachOutputBus(0); err != nil {
		t.Fatalf("DetachOutputBus: %v", err)
	}
	if err := source.AttachOutputBus(0, graph.Endpoint(), 0); err != nil {
		t.Fatalf("reattach source: %v", err)
	}
	if err := source.DetachAllOutputBuses(); err != nil {
		t.Fatalf("DetachAllOutputBuses: %v", err)
	}

	for i := len(nodes) - 1; i >= 0; i-- {
		closer, ok := nodes[i].node.(interface{ Close() error })
		if !ok {
			t.Fatalf("%s node does not implement Close", nodes[i].name)
		}
		if err := closer.Close(); err != nil {
			t.Errorf("close %s node: %v", nodes[i].name, err)
		}
		if err := closer.Close(); err != nil {
			t.Errorf("second close of %s node: %v", nodes[i].name, err)
		}
	}
}
