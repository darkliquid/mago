// Command capture records from an input device for a short time and reports the
// frame count, peak and RMS level. It demonstrates the capture device type
// added by the DeviceConfig/NewDevice API. With no argument it picks the first
// usable backend, falling back to the null backend (which yields silence but
// still exercises the capture callback).
package main

import (
	"flag"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	backendFlag := flag.String("backend", example.Env("MAGO_BACKEND", ""), "backend to use; defaults to the first available, then null")
	deviceIndexFlag := flag.Int("device-index", example.EnvInt("MAGO_DEVICE_INDEX", -1), "capture device index from the enumerated list")
	deviceNameFlag := flag.String("device-name", example.Env("MAGO_DEVICE_NAME", ""), "substring to match in the capture device name")
	durationFlag := flag.Duration("duration", example.EnvDuration("MAGO_CAPTURE_DURATION", 2*time.Second), "how long to capture")
	flag.Parse()

	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	backend, err := example.SelectBackend(lib, *backendFlag)
	example.Must(err)
	ctx, err := lib.NewContext(backend.Backend)
	example.Must(err)
	defer func() { example.Must(ctx.Close()) }()

	_, capture, err := ctx.Devices()
	example.Must(err)
	if len(capture) == 0 {
		example.Must(fmt.Errorf("no capture devices on backend %s", backend.Name))
	}

	index, name, err := example.SelectDevice(capture, *deviceIndexFlag, *deviceNameFlag)
	example.Must(err)
	fmt.Printf("backend: %s\ncapturing from [%d] %s for %s\n", backend.Name, index, name, durationFlag.String())

	const (
		channels   = 1
		sampleRate = 48_000
	)

	var (
		mu         sync.Mutex
		frames     uint64
		peak       float64
		sumSquares float64
	)

	device, err := ctx.NewDevice(mago.DeviceConfig{
		Type: mago.DeviceTypeCapture,
		Capture: &mago.StreamConfig{
			DeviceIndex:        index,
			Format:             mago.FormatF32,
			Channels:           channels,
			SampleRate:         sampleRate,
			PeriodSizeInFrames: 256,
		},
		DataCallback: func(_ *mago.Device, io mago.DeviceIO) {
			samples := io.InputF32()
			if len(samples) == 0 {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			frames += uint64(io.FrameCount())
			for _, sample := range samples {
				value := float64(sample)
				if magnitude := math.Abs(value); magnitude > peak {
					peak = magnitude
				}
				sumSquares += value * value
			}
		},
	})
	example.Must(err)
	defer func() { example.Must(device.Close()) }()

	fmt.Printf("capture device: %s (%s)\n", name, example.StateName(device.State()))

	example.Must(device.Start())
	time.Sleep(*durationFlag)
	example.Must(device.Stop())

	mu.Lock()
	defer mu.Unlock()

	rms := 0.0
	if total := frames * channels; total > 0 {
		rms = math.Sqrt(sumSquares / float64(total))
	}
	fmt.Printf("captured %d frames\n", frames)
	fmt.Printf("peak: %.4f\n", peak)
	fmt.Printf("rms:  %.4f\n", rms)
	if frames == 0 {
		fmt.Println("no frames captured; check the selected backend and device")
	} else if peak == 0 {
		fmt.Println("captured silence (expected on the null backend)")
	}
	fmt.Println("done")
}
