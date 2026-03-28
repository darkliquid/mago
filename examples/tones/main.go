package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
	"github.com/darkliquid/mago/internal/buildlib"
)

func main() {
	backendFlag := flag.String("backend", getenv("MAGO_BACKEND", ""), "backend to use (platform-dependent, for example pulse/alsa/jack/coreaudio/wasapi)")
	deviceIndexFlag := flag.Int("device-index", getenvInt("MAGO_DEVICE_INDEX", -1), "playback device index from the enumerated list")
	deviceNameFlag := flag.String("device-name", getenv("MAGO_DEVICE_NAME", ""), "substring to match in the playback device name")
	durationFlag := flag.Duration("duration", getenvDuration("MAGO_TONE_DURATION", 3*time.Second), "how long to play the tones")
	flag.Parse()

	repoRoot, err := findRepoRoot()
	must(err)

	libPath := buildlib.DefaultOutputPath(repoRoot)
	must(ensureSharedLibrary(repoRoot, libPath))

	lib, err := mago.Open(mago.WithLibraryPath(libPath))
	must(err)
	defer func() {
		must(lib.Close())
	}()

	backendName, backend, err := selectBackend(lib, *backendFlag)
	must(err)

	ctx, err := lib.NewContext(backend)
	must(err)
	defer func() {
		must(ctx.Close())
	}()

	playbackDevices, _, err := ctx.Devices()
	must(err)
	if len(playbackDevices) == 0 {
		must(fmt.Errorf("no playback devices found for backend %s", backendName))
	}

	fmt.Printf("backend: %s\n", backendName)
	fmt.Println("playback devices:")
	for i, device := range playbackDevices {
		marker := " "
		if device.IsDefault {
			marker = "*"
		}
		fmt.Printf("  %s [%d] %s\n", marker, i, device.Name)
	}

	selectedIndex, selectedName, err := selectDevice(playbackDevices, *deviceIndexFlag, *deviceNameFlag)
	must(err)
	fmt.Printf("selected device: [%d] %s\n", selectedIndex, selectedName)

	config := mago.DefaultPlaybackDeviceConfig()
	config.DeviceIndex = selectedIndex
	config.Channels = 2
	config.SampleRate = 48_000
	config.PeriodSizeInFrames = 256

	var phaseA float64
	var phaseB float64
	config.DataCallback = func(device *mago.Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
		_ = input
		samples := unsafe.Slice((*float32)(output), int(frameCount*config.Channels))
		for frame := 0; frame < int(frameCount); frame++ {
			value := float32(0.18*math.Sin(phaseA) + 0.12*math.Sin(phaseB))
			phaseA += 2 * math.Pi * 220 / float64(config.SampleRate)
			phaseB += 2 * math.Pi * 440 / float64(config.SampleRate)
			if phaseA >= 2*math.Pi {
				phaseA -= 2 * math.Pi
			}
			if phaseB >= 2*math.Pi {
				phaseB -= 2 * math.Pi
			}
			for ch := 0; ch < int(config.Channels); ch++ {
				samples[frame*int(config.Channels)+ch] = value
			}
		}
	}

	device, err := ctx.NewPlaybackDevice(config)
	must(err)
	defer func() {
		must(device.Close())
	}()

	fmt.Printf("playing tones for %s...\n", durationFlag.String())
	must(device.Start())
	time.Sleep(*durationFlag)
	must(device.Stop())
	fmt.Println("done")
}

func selectBackend(lib *mago.Library, requested string) (string, mago.Backend, error) {
	if requested != "" {
		name, backend, ok := parseBackend(requested)
		if !ok {
			return "", 0, fmt.Errorf("unknown backend %q", requested)
		}
		ctx, err := lib.NewContext(backend)
		if err == nil {
			_ = ctx.Close()
			return name, backend, nil
		}
		return "", 0, fmt.Errorf("backend %s is unavailable", name)
	}

	candidates := backendCandidates()
	for _, candidate := range candidates {
		ctx, err := lib.NewContext(candidate.backend)
		if err == nil {
			_ = ctx.Close()
			return candidate.name, candidate.backend, nil
		}
	}

	return "", 0, fmt.Errorf("could not find an available real-device backend; try --backend or check PulseAudio/ALSA/JACK")
}

func parseBackend(value string) (string, mago.Backend, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "wasapi":
		return "WASAPI", mago.BackendWASAPI, true
	case "dsound", "directsound":
		return "DirectSound", mago.BackendDSound, true
	case "winmm":
		return "WinMM", mago.BackendWinMM, true
	case "coreaudio", "core-audio":
		return "CoreAudio", mago.BackendCoreAudio, true
	case "audio4", "audio(4)":
		return "audio(4)", mago.BackendAudio4, true
	case "oss":
		return "OSS", mago.BackendOSS, true
	case "pulse", "pulseaudio":
		return "PulseAudio", mago.BackendPulseAudio, true
	case "alsa":
		return "ALSA", mago.BackendALSA, true
	case "jack":
		return "JACK", mago.BackendJACK, true
	default:
		return "", 0, false
	}
}

func selectDevice(devices []mago.DeviceInfo, requestedIndex int, requestedName string) (int, string, error) {
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
		return 0, "", fmt.Errorf("no playback device matched %q", requestedName)
	}

	for i, device := range devices {
		if device.IsDefault {
			return i, device.Name, nil
		}
	}

	return 0, devices[0].Name, nil
}

func findRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func ensureSharedLibrary(repoRoot, libPath string) error {
	return buildlib.Build(repoRoot, libPath)
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
		}
	case "darwin":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "CoreAudio", backend: mago.BackendCoreAudio},
		}
	case "freebsd":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "OSS", backend: mago.BackendOSS},
			{name: "JACK", backend: mago.BackendJACK},
		}
	case "netbsd":
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "audio(4)", backend: mago.BackendAudio4},
		}
	default:
		return []struct {
			name    string
			backend mago.Backend
		}{
			{name: "PulseAudio", backend: mago.BackendPulseAudio},
			{name: "ALSA", backend: mago.BackendALSA},
			{name: "JACK", backend: mago.BackendJACK},
		}
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
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

func getenvDuration(key string, fallback time.Duration) time.Duration {
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

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
