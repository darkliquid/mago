// Command duplex-echo opens a duplex device and copies captured input straight
// to the output with a fixed gain, demonstrating the duplex device type. Use
// headphones: with a microphone and speakers the loop can feed back.
//
// The example also shows master-volume control while the device is running.
package main

import (
	"flag"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	backendFlag := flag.String("backend", example.Env("MAGO_BACKEND", ""), "backend to use; defaults to the first available, then null")
	deviceIndexFlag := flag.Int("device-index", example.EnvInt("MAGO_DEVICE_INDEX", -1), "device index from the enumerated playback list")
	durationFlag := flag.Duration("duration", example.EnvDuration("MAGO_ECHO_DURATION", 3*time.Second), "how long to run the echo")
	gainFlag := flag.Float64("gain", 0.5, "input-to-output gain (0..1)")
	flag.Parse()

	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	backend, err := example.SelectBackend(lib, *backendFlag)
	example.Must(err)
	ctx, err := lib.NewContext(backend.Backend)
	example.Must(err)
	defer func() { example.Must(ctx.Close()) }()

	playback, capture, err := ctx.Devices()
	example.Must(err)
	if len(playback) == 0 || len(capture) == 0 {
		example.Must(fmt.Errorf("duplex requires both playback and capture devices on backend %s", backend.Name))
	}

	index, name, err := example.SelectDevice(playback, *deviceIndexFlag, "")
	example.Must(err)
	fmt.Printf("backend: %s\nechoing [%d] %s for %s at gain %.2f\n", backend.Name, index, name, durationFlag.String(), *gainFlag)

	const (
		channels   = 1
		sampleRate = 48_000
	)

	gain := float32(*gainFlag)
	var frames atomic.Uint64

	device, err := ctx.NewDevice(mago.DeviceConfig{
		Type: mago.DeviceTypeDuplex,
		Playback: &mago.StreamConfig{
			DeviceIndex:        index,
			Format:             mago.FormatF32,
			Channels:           channels,
			SampleRate:         sampleRate,
			PeriodSizeInFrames: 256,
		},
		Capture: &mago.StreamConfig{
			DeviceIndex:        -1,
			Format:             mago.FormatF32,
			Channels:           channels,
			SampleRate:         sampleRate,
			PeriodSizeInFrames: 256,
		},
		DataCallback: func(_ *mago.Device, io mago.DeviceIO) {
			out := io.OutputF32()
			if len(out) == 0 {
				return
			}
			in := io.InputF32()
			if len(in) == 0 {
				for i := range out {
					out[i] = 0
				}
				return
			}
			for i := range out {
				out[i] = in[i] * gain
			}
			frames.Add(uint64(io.FrameCount()))
		},
	})
	example.Must(err)
	defer func() { example.Must(device.Close()) }()

	example.Must(device.SetMasterVolume(1))
	example.Must(device.Start())
	fmt.Printf("state: %s\n", example.StateName(device.State()))

	// Demonstrate live volume control half-way through.
	time.Sleep(*durationFlag / 2)
	example.Must(device.SetMasterVolume(0.3))
	volume, err := device.MasterVolume()
	example.Must(err)
	fmt.Printf("lowered master volume to %.2f\n", volume)

	time.Sleep(*durationFlag / 2)
	example.Must(device.Stop())

	fmt.Printf("echoed %d frames\n", frames.Load())
	fmt.Println("done")
}
