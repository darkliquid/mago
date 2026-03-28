package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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

	backends := []struct {
		name    string
		backend mago.Backend
	}{
		{name: "PulseAudio", backend: mago.BackendPulseAudio},
		{name: "ALSA", backend: mago.BackendALSA},
		{name: "JACK", backend: mago.BackendJACK},
		{name: "Null", backend: mago.BackendNull},
	}

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
