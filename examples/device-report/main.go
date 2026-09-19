// Command device-report opens a playback device and reports the new device
// introspection and master-volume APIs: state, name, info, and volume in both
// linear and decibel units.
package main

import (
	"flag"
	"fmt"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/examples/internal/example"
)

func main() {
	backendFlag := flag.String("backend", example.Env("MAGO_BACKEND", ""), "backend to use; defaults to the first available, then null")
	deviceIndexFlag := flag.Int("device-index", example.EnvInt("MAGO_DEVICE_INDEX", -1), "playback device index from the enumerated list")
	deviceNameFlag := flag.String("device-name", example.Env("MAGO_DEVICE_NAME", ""), "substring to match in the playback device name")
	volumeFlag := flag.Float64("volume", 0.5, "linear master volume to apply before the short playback")
	flag.Parse()

	lib, err := example.Open()
	example.Must(err)
	defer func() { example.Must(lib.Close()) }()

	version, err := lib.Version()
	example.Must(err)
	fmt.Printf("loaded miniaudio %s\n", version.String())

	backend, err := example.SelectBackend(lib, *backendFlag)
	example.Must(err)
	ctx, err := lib.NewContext(backend.Backend)
	example.Must(err)
	defer func() { example.Must(ctx.Close()) }()

	fmt.Printf("backend: %s\n", backend.Name)

	playback, capture, err := ctx.Devices()
	example.Must(err)
	example.PrintDevices("Playback", playback)
	example.PrintDevices("Capture", capture)

	if len(playback) == 0 {
		example.Must(fmt.Errorf("no playback devices on backend %s", backend.Name))
	}

	index, name, err := example.SelectDevice(playback, *deviceIndexFlag, *deviceNameFlag)
	example.Must(err)
	fmt.Printf("selected: [%d] %s\n", index, name)

	// Context.DeviceInfo reports details for one device without opening it.
	info, err := ctx.DeviceInfo(mago.DeviceTypePlayback, nil)
	example.Must(err)
	fmt.Printf("default device info: name=%q default=%v\n", info.Name, info.IsDefault)

	device, err := ctx.NewDevice(mago.DeviceConfig{
		Type: mago.DeviceTypePlayback,
		Playback: &mago.StreamConfig{
			DeviceIndex:        index,
			Format:             mago.FormatF32,
			Channels:           2,
			SampleRate:         48_000,
			PeriodSizeInFrames: 256,
		},
		DataCallback: func(_ *mago.Device, output unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
			if output == nil {
				return
			}
			samples := unsafe.Slice((*float32)(output), int(frameCount)*2)
			for i := range samples {
				samples[i] = 0
			}
		},
	})
	example.Must(err)
	defer func() { example.Must(device.Close()) }()

	fmt.Printf("state after init: %s\n", example.StateName(device.State()))
	deviceName, err := device.Name()
	example.Must(err)
	fmt.Printf("device name: %s\n", deviceName)
	deviceInfo, err := device.Info()
	example.Must(err)
	fmt.Printf("device info: name=%q default=%v\n", deviceInfo.Name, deviceInfo.IsDefault)
	if device.Context() != nil {
		fmt.Println("device reports an owning context")
	}

	example.Must(device.SetMasterVolume(*volumeFlag))
	linear, err := device.MasterVolume()
	example.Must(err)
	decibels, err := device.MasterVolumeDB()
	example.Must(err)
	fmt.Printf("master volume: requested %.2f, read back %.3f (%.2f dB)\n", *volumeFlag, linear, decibels)

	fmt.Println("starting a short silent playback...")
	example.Must(device.Start())
	fmt.Printf("state while running: %s\n", example.StateName(device.State()))
	time.Sleep(250 * time.Millisecond)
	example.Must(device.Stop())
	fmt.Printf("state after stop: %s\n", example.StateName(device.State()))
	fmt.Println("done")
}
