package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/darkliquid/mago"
)

func main() {
	lib, err := mago.Open()
	must(err)
	defer func() {
		must(lib.Close())
	}()

	version, err := lib.Version()
	must(err)
	fmt.Printf("loaded miniaudio %s from %s\n", version.String(), lib.Path())

	ctx, err := lib.NewContext(mago.BackendNull)
	must(err)
	defer func() {
		must(ctx.Close())
	}()

	var callbackCount atomic.Uint64
	config := mago.DefaultPlaybackDeviceConfig()
	config.Channels = 1
	config.SampleRate = 48_000
	config.PeriodSizeInFrames = 128
	config.DataCallback = func(_ *mago.Device, io mago.DeviceIO) {
		samples := io.OutputF32()
		for i := range samples {
			samples[i] = 0
		}
		callbackCount.Add(1)
	}
	config.NotificationCallback = func(_ *mago.Device, notification mago.NotificationType) {
		fmt.Printf("notification: %v\n", notification)
	}

	device, err := ctx.NewPlaybackDevice(config)
	must(err)
	defer func() {
		must(device.Close())
	}()

	fmt.Println("starting null-backend playback demo...")
	must(device.Start())
	time.Sleep(500 * time.Millisecond)
	must(device.Stop())

	fmt.Printf("demo complete, callbacks observed: %d\n", callbackCount.Load())
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
