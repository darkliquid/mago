// Package example holds helpers shared by the runnable examples: locating the
// repository, building and opening the native library, and selecting a backend
// and device. It lives under examples/internal so it cannot be imported by
// code outside the examples tree.
package example

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/internal/buildlib"
)

// Backend pairs a human-readable name with the mago backend value.
type Backend struct {
	Name    string
	Backend mago.Backend
}

// FindRepoRoot locates the repository root relative to this helper file, so it
// is correct regardless of which example calls it.
func FindRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}

// Open builds the native library for the current host, loads it, and returns the
// open library. The caller is responsible for closing it.
func Open() (*mago.Library, error) {
	root, err := FindRepoRoot()
	if err != nil {
		return nil, err
	}
	libPath := buildlib.DefaultOutputPath(root)
	if err := buildlib.Build(root, libPath, ""); err != nil {
		return nil, err
	}
	return mago.Open(mago.WithLibraryPath(libPath))
}

// Candidates lists the backends worth trying on the current platform, ending
// with the null backend so a demo always has a usable fallback.
func Candidates() []Backend {
	var backends []Backend
	switch runtime.GOOS {
	case "windows":
		backends = []Backend{
			{Name: "WASAPI", Backend: mago.BackendWASAPI},
			{Name: "DirectSound", Backend: mago.BackendDSound},
			{Name: "WinMM", Backend: mago.BackendWinMM},
		}
	case "darwin":
		backends = []Backend{
			{Name: "CoreAudio", Backend: mago.BackendCoreAudio},
		}
	case "freebsd":
		backends = []Backend{
			{Name: "OSS", Backend: mago.BackendOSS},
			{Name: "JACK", Backend: mago.BackendJACK},
		}
	case "netbsd":
		backends = []Backend{
			{Name: "audio(4)", Backend: mago.BackendAudio4},
		}
	default:
		backends = []Backend{
			{Name: "PulseAudio", Backend: mago.BackendPulseAudio},
			{Name: "ALSA", Backend: mago.BackendALSA},
			{Name: "JACK", Backend: mago.BackendJACK},
		}
	}
	return append(backends, Backend{Name: "Null", Backend: mago.BackendNull})
}

// ParseBackend resolves a backend name supplied on the command line.
func ParseBackend(value string) (Backend, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "wasapi":
		return Backend{Name: "WASAPI", Backend: mago.BackendWASAPI}, true
	case "dsound", "directsound":
		return Backend{Name: "DirectSound", Backend: mago.BackendDSound}, true
	case "winmm":
		return Backend{Name: "WinMM", Backend: mago.BackendWinMM}, true
	case "coreaudio", "core-audio":
		return Backend{Name: "CoreAudio", Backend: mago.BackendCoreAudio}, true
	case "audio4", "audio(4)":
		return Backend{Name: "audio(4)", Backend: mago.BackendAudio4}, true
	case "oss":
		return Backend{Name: "OSS", Backend: mago.BackendOSS}, true
	case "pulse", "pulseaudio":
		return Backend{Name: "PulseAudio", Backend: mago.BackendPulseAudio}, true
	case "alsa":
		return Backend{Name: "ALSA", Backend: mago.BackendALSA}, true
	case "jack":
		return Backend{Name: "JACK", Backend: mago.BackendJACK}, true
	case "null":
		return Backend{Name: "Null", Backend: mago.BackendNull}, true
	default:
		return Backend{}, false
	}
}

// SelectBackend returns the requested backend when one is given, otherwise the
// first backend that initializes on this machine. The null backend is always
// tried last, so a demo can run without audio hardware.
func SelectBackend(lib *mago.Library, requested string) (Backend, error) {
	if requested != "" {
		backend, ok := ParseBackend(requested)
		if !ok {
			return Backend{}, fmt.Errorf("unknown backend %q", requested)
		}
		ctx, err := lib.NewContext(backend.Backend)
		if err != nil {
			return Backend{}, fmt.Errorf("backend %s is unavailable: %w", backend.Name, err)
		}
		_ = ctx.Close()
		return backend, nil
	}

	for _, candidate := range Candidates() {
		ctx, err := lib.NewContext(candidate.Backend)
		if err == nil {
			_ = ctx.Close()
			return candidate, nil
		}
	}

	return Backend{}, fmt.Errorf("no usable backend found")
}

// SelectDevice resolves a device by index or name, defaulting to the
// system-default device.
func SelectDevice(devices []mago.DeviceInfo, requestedIndex int, requestedName string) (int, string, error) {
	if requestedIndex >= 0 {
		if requestedIndex >= len(devices) {
			return 0, "", fmt.Errorf("device index %d is out of range", requestedIndex)
		}
		return requestedIndex, devices[requestedIndex].Name, nil
	}

	if requestedName != "" {
		needle := strings.ToLower(requestedName)
		for i, device := range devices {
			if strings.Contains(strings.ToLower(device.Name), needle) {
				return i, device.Name, nil
			}
		}
		return 0, "", fmt.Errorf("no device matched %q", requestedName)
	}

	for i, device := range devices {
		if device.IsDefault {
			return i, device.Name, nil
		}
	}

	return 0, devices[0].Name, nil
}

// PrintDevices prints a labelled device list.
func PrintDevices(kind string, devices []mago.DeviceInfo) {
	fmt.Printf("%s devices:\n", kind)
	if len(devices) == 0 {
		fmt.Println("  (none)")
		return
	}
	for i, device := range devices {
		marker := " "
		if device.IsDefault {
			marker = "*"
		}
		fmt.Printf("  %s [%d] %s\n", marker, i, device.Name)
	}
}

// StateName returns a human-readable device state.
func StateName(state mago.DeviceState) string {
	switch state {
	case mago.DeviceStateUninitialized:
		return "uninitialized"
	case mago.DeviceStateStopped:
		return "stopped"
	case mago.DeviceStateStarted:
		return "started"
	case mago.DeviceStateStarting:
		return "starting"
	case mago.DeviceStateStopping:
		return "stopping"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

// Env returns the environment variable or a fallback.
func Env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// EnvInt returns an integer environment variable or a fallback.
func EnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// EnvDuration returns a duration environment variable or a fallback.
func EnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

// Must exits with a message when err is non-nil.
func Must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
