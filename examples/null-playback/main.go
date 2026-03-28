package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
)

func main() {
	repoRoot, err := findRepoRoot()
	must(err)

	libPath := filepath.Join(repoRoot, "native", "libminiaudio.so")
	must(ensureSharedLibrary(repoRoot, libPath))

	lib, err := mago.Open(mago.WithLibraryPath(libPath))
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
	config.DataCallback = func(device *mago.Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
		if output != nil {
			samples := unsafe.Slice((*float32)(output), int(frameCount))
			for i := range samples {
				samples[i] = 0
			}
		}
		callbackCount.Add(1)
	}
	config.NotificationCallback = func(device *mago.Device, notification mago.NotificationType) {
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

func findRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func ensureSharedLibrary(repoRoot, libPath string) error {
	cmd := exec.Command("bash", filepath.Join(repoRoot, "native", "build.sh"), libPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
