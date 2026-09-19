// Command channel-map shows channel maps and the channel converter: standard
// layouts, blank maps, the default map for a channel count, cloning, and a
// stereo-to-mono down-mix. It needs no audio device.
package main

import (
	"fmt"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	fmt.Println("standard channel maps (Microsoft layout):")
	for _, channels := range []uint32{1, 2, 6} {
		m := lib.NewStandardChannelMap(mago.StandardChannelMapMicrosoft, channels)
		fmt.Printf("  %d channels: %s\n", channels, m)
	}

	blank := lib.NewBlankChannelMap(3)
	fmt.Printf("blank 3-channel map: %s\n", blank)

	def := lib.DefaultChannelMapFor(mago.ChannelMap{}, 2)
	fmt.Printf("default map for 2 channels: %s\n", def)

	clone := def.Clone()
	fmt.Printf("clone: %s (%d channels)\n", clone, clone.Len())

	converter, err := lib.NewChannelConverter(mago.ChannelConverterConfig{
		Format:      mago.FormatF32,
		ChannelsIn:  2,
		ChannelsOut: 1,
		MixingMode:  mago.ChannelMixModeRectangular,
	})
	example.Must(err)
	defer func() { example.Must(converter.Close()) }()

	input := []float32{0.25, 0.25, -0.5, -0.5, 1, 1, 0, 0}
	output := make([]float32, len(input)/2)

	example.Must(converter.Process(output, input))

	inMap, err := converter.InputChannelMap()
	example.Must(err)
	outMap, err := converter.OutputChannelMap()
	example.Must(err)

	fmt.Printf("converter input map:  %s\n", inMap)
	fmt.Printf("converter output map: %s\n", outMap)
	fmt.Println("stereo -> mono down-mix:")
	for i, value := range output {
		fmt.Printf("  frame %d: %+.3f\n", i, value)
	}
}
