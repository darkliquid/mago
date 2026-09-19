package mago

import "testing"

func TestDeviceStateAndInfo(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	device, err := ctx.NewDevice(DeviceConfig{
		Type:     DeviceTypePlayback,
		Playback: &StreamConfig{DeviceIndex: 0, Channels: 2, SampleRate: 48000, PeriodSizeInFrames: 64},
	})
	if err != nil {
		t.Fatalf("NewDevice: %v", err)
	}
	defer func() { _ = device.Close() }()

	if got := device.State(); got != DeviceStateStopped {
		t.Fatalf("state after init = %v, want stopped", got)
	}

	name, err := device.Name()
	if err != nil {
		t.Fatalf("Name: %v", err)
	}
	if name == "" {
		t.Fatal("expected a non-empty device name from the null backend")
	}

	if _, err := device.Info(); err != nil {
		t.Fatalf("Info: %v", err)
	}

	if device.Context() == nil {
		t.Fatal("expected a non-nil borrowed context")
	}
}
