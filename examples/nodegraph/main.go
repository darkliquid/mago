// Command nodegraph builds an in-memory audio rack out of miniaudio's node
// graph: a waveform is split, one branch runs through a low-pass filter and the
// other through a delay, and both are summed back into the graph endpoint. It
// needs no audio device.
package main

import (
	"fmt"
	"math"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

const (
	channels   = 2
	sampleRate = 48000
	blockSize  = 1024
)

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	graph, err := lib.NewNodeGraph(mago.NodeGraphConfig{Channels: channels})
	example.Must(err)

	fmt.Printf("node graph: %d channel(s), processing block %d frame(s)\n",
		graph.Channels(), graph.ProcessingSizeInFrames())

	waveform, err := lib.NewWaveform(mago.WaveformConfig{
		Format:     mago.FormatF32,
		Channels:   channels,
		SampleRate: sampleRate,
		Type:       mago.WaveformTypeSawtooth,
		Amplitude:  0.25,
		Frequency:  220,
	})
	example.Must(err)

	source, err := graph.NewDataSourceNode(waveform)
	example.Must(err)

	splitter, err := graph.NewSplitterNode(mago.DefaultSplitterNodeConfig(channels))
	example.Must(err)

	// A very low cutoff keeps only the fundamental of the sawtooth.
	filter, err := graph.NewBiquadNode(mago.BiquadNodeConfig{
		Channels: channels,
		B0:       0.0025, B1: 0.005, B2: 0.0025,
		A0: 1, A1: -1.9, A2: 0.91,
	})
	example.Must(err)

	// A 12000-frame delay is a quarter second at 48 kHz, and a decay of 0.4
	// gives the echo a few audible repeats.
	delay, err := graph.NewDelayNode(mago.DefaultDelayConfig(channels, sampleRate, 12000, 0.4))
	example.Must(err)

	example.Must(source.AttachOutputBus(0, splitter, 0))
	example.Must(splitter.AttachOutputBus(0, filter, 0))
	example.Must(filter.AttachOutputBus(0, graph.Endpoint(), 0))
	example.Must(splitter.AttachOutputBus(1, delay, 0))
	example.Must(delay.AttachOutputBus(0, graph.Endpoint(), 0))

	fmt.Printf("routing: waveform -> splitter -> {biquad low-pass, delay} -> endpoint\n")

	for block := 1; block <= 4; block++ {
		peak, err := readPeak(graph, blockSize)
		example.Must(err)
		fmt.Printf("block %d: peak %.4f, graph time %d frames\n", block, peak, graph.Time())
	}

	// Per-bus volume is a routing control: muting the filter branch leaves only
	// the delay.
	example.Must(filter.SetOutputBusVolume(0, 0))
	peak, err := readPeak(graph, blockSize)
	example.Must(err)
	fmt.Printf("filter branch muted: peak %.4f\n", peak)
	example.Must(filter.SetOutputBusVolume(0, 1))

	// Closing order matters: nodes first, then the graph they belong to, then the
	// waveform the data source node was reading from.
	example.Must(delay.Close())
	example.Must(filter.Close())
	example.Must(splitter.Close())
	example.Must(source.Close())
	example.Must(graph.Close())
	example.Must(waveform.Close())
}

// readPeak reads one block from the graph and returns the largest sample
// magnitude in it.
func readPeak(graph *mago.NodeGraph, frames int) (float64, error) {
	samples := make([]float32, frames*int(graph.Channels()))
	if _, err := graph.Read(samples); err != nil {
		return 0, err
	}

	peak := 0.0
	for _, sample := range samples {
		if magnitude := math.Abs(float64(sample)); magnitude > peak {
			peak = magnitude
		}
	}
	return peak, nil
}
