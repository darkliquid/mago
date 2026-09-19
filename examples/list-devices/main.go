package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/darkliquid/mago"
)

func main() {
	lib, err := mago.Open()
	must(err)
	defer func() {
		must(lib.Close())
	}()

	backends := backendCandidates()

	for _, entry := range backends {
		fmt.Printf("== %s ==\n", entry.name)

		ctx, err := lib.NewContext(entry.backend)
		if err != nil {
			fmt.Printf("unavailable: %v\n\n", err)
			continue
		}

		playback, capture, err := ctx.Devices()
		closeErr := ctx.Close()
		if err != nil {
			fmt.Printf("enumeration failed: %v\n\n", err)
			must(closeErr)
			continue
		}
		must(closeErr)

		printDevices("Playback", playback)
		printDevices("Capture", capture)
		fmt.Println()
	}
}

func printDevices(kind string, devices []mago.DeviceInfo) {
	fmt.Printf("%s devices:\n", kind)
	if len(devices) == 0 {
		fmt.Println("  (none)")
		return
	}

	for _, device := range devices {
		marker := " "
		if device.IsDefault {
			marker = "*"
		}
		fmt.Printf("  %s %s\n", marker, device.Name)
	}
}

func backendCandidates() []struct {
	name    string
	backend mago.Backend
} {
	switch runtime.GOOS {
	case "windows":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "WASAPI", backend: mago.BackendWASAPI},
			{name: "DirectSound", backend: mago.BackendDSound},
			{name: "WinMM", backend: mago.BackendWinMM},
			{name: "Null", backend: mago.BackendNull},
		}
	case "darwin":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "CoreAudio", backend: mago.BackendCoreAudio},
			{name: "Null", backend: mago.BackendNull},
		}
	case "freebsd":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "OSS", backend: mago.BackendOSS},
			{name: "JACK", backend: mago.BackendJACK},
			{name: "Null", backend: mago.BackendNull},
		}
	case "netbsd":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "audio(4)", backend: mago.BackendAudio4},
			{name: "Null", backend: mago.BackendNull},
		}
	default:
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "PulseAudio", backend: mago.BackendPulseAudio},
			{name: "ALSA", backend: mago.BackendALSA},
			{name: "JACK", backend: mago.BackendJACK},
			{name: "Null", backend: mago.BackendNull},
		}
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
