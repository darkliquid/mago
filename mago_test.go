//go:build linux

package mago

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/darkliquid/mago/internal/buildlib"
)

func TestVersionAndNullBackendPlayback(t *testing.T) {
	t.Parallel()

	libPath := filepath.Join(t.TempDir(), buildlib.DefaultLibraryFilename("linux"))
	if err := buildlib.Build(".", libPath); err != nil {
		t.Fatalf("build shared library: %v", err)
	}

	lib, err := Open(WithLibraryPath(libPath))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	defer func() {
		if err := lib.Close(); err != nil {
			t.Fatalf("close library: %v", err)
		}
	}()

	version, err := lib.Version()
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != (Version{
		Major:    ExpectedMiniaudioVersionMajor,
		Minor:    ExpectedMiniaudioVersionMinor,
		Revision: ExpectedMiniaudioVersionRevision,
	}) {
		t.Fatalf("unexpected version: %+v", version)
	}

	versionString, err := lib.VersionString()
	if err != nil {
		t.Fatalf("version string: %v", err)
	}
	if versionString != version.String() {
		t.Fatalf("unexpected version string: %q", versionString)
	}

	ctx, err := lib.NewContext(BackendNull)
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	defer func() {
		if err := ctx.Close(); err != nil {
			t.Fatalf("close context: %v", err)
		}
	}()

	playbackDevices, captureDevices, err := ctx.Devices()
	if err != nil {
		t.Fatalf("enumerate devices: %v", err)
	}
	if len(playbackDevices) == 0 {
		t.Fatal("expected at least one playback device from the null backend")
	}
	_ = captureDevices

	callbacks := make(chan uint32, 8)
	notifications := make(chan NotificationType, 8)

	config := DefaultPlaybackDeviceConfig()
	config.DeviceIndex = 0
	config.Channels = 1
	config.SampleRate = 48000
	config.PeriodSizeInFrames = 64
	config.DataCallback = func(device *Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
		if output != nil {
			samples := unsafe.Slice((*float32)(output), int(frameCount))
			for i := range samples {
				samples[i] = 0
			}
		}
		select {
		case callbacks <- frameCount:
		default:
		}
	}
	config.NotificationCallback = func(device *Device, notification NotificationType) {
		select {
		case notifications <- notification:
		default:
		}
	}

	device, err := ctx.NewPlaybackDevice(config)
	if err != nil {
		t.Fatalf("new playback device: %v", err)
	}
	defer func() {
		if err := device.Close(); err != nil {
			t.Fatalf("close device: %v", err)
		}
	}()

	if err := device.Start(); err != nil {
		t.Fatalf("start device: %v", err)
	}

	select {
	case frames := <-callbacks:
		if frames == 0 {
			t.Fatal("expected a non-zero callback frame count")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audio callback")
	}

	if err := device.Stop(); err != nil {
		t.Fatalf("stop device: %v", err)
	}

	select {
	case got := <-notifications:
		if got != NotificationStarted && got != NotificationStopped {
			t.Fatalf("unexpected notification: %v", got)
		}
	default:
	}
}

func TestValidateLoadedVersionRejectsMismatch(t *testing.T) {
	t.Parallel()

	err := validateLoadedVersion(
		Version{
			Major:    ExpectedMiniaudioVersionMajor,
			Minor:    ExpectedMiniaudioVersionMinor + 1,
			Revision: ExpectedMiniaudioVersionRevision,
		},
		"0.12.25",
	)
	if err == nil {
		t.Fatal("expected a version mismatch error")
	}

	var mismatch *VersionMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected VersionMismatchError, got %T", err)
	}
}
