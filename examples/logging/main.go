// Command logging shows how to route miniaudio's own diagnostics into Go. It
// registers a callback on a ma_log, attaches that log to a context, posts a few
// messages through the log, and then runs a device briefly so any internal
// miniaudio logging is captured too.
package main

import (
	"flag"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	backendFlag := flag.String("backend", example.Env("MAGO_BACKEND", "null"), "backend to use; defaults to null")
	flag.Parse()

	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	backend, err := example.SelectBackend(lib, *backendFlag)
	example.Must(err)

	log, err := lib.NewLog()
	example.Must(err)
	defer func() { example.Must(log.Close()) }()

	var mu sync.Mutex
	var lines int
	token, err := log.Register(func(level mago.LogLevel, message string) {
		mu.Lock()
		defer mu.Unlock()
		lines++
		fmt.Printf("[miniaudio %s] %s\n", log.LevelString(level), message)
	})
	example.Must(err)
	defer func() { example.Must(log.Unregister(token)) }()

	ctx, err := lib.NewContextWithLog(log, backend.Backend)
	example.Must(err)
	defer func() { example.Must(ctx.Close()) }()

	fmt.Printf("backend: %s (context created with an attached log)\n", backend.Name)
	if ctx.Log() != nil {
		fmt.Println("context confirms it has a log attached")
	}

	example.Must(log.Post(mago.LogLevelInfo, "hello from the logging example"))
	example.Must(log.Post(mago.LogLevelWarning, "this is a warning-level message"))

	// Run a device briefly; any miniaudio diagnostics flow through the log.
	device, err := ctx.NewDevice(mago.DeviceConfig{
		Type:     mago.DeviceTypePlayback,
		Playback: &mago.StreamConfig{DeviceIndex: -1, Channels: 1, SampleRate: 48_000, PeriodSizeInFrames: 128},
		DataCallback: func(_ *mago.Device, output unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
			if output == nil {
				return
			}
			samples := unsafe.Slice((*float32)(output), int(frameCount))
			for i := range samples {
				samples[i] = 0
			}
		},
	})
	example.Must(err)
	defer func() { example.Must(device.Close()) }()

	example.Must(device.Start())
	time.Sleep(200 * time.Millisecond)
	example.Must(device.Stop())

	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("done, %d log lines received\n", lines)
}
