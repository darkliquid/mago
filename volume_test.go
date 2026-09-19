package mago

import (
	"math"
	"testing"
)

func TestMasterVolumeRoundTrip(t *testing.T) {
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

	if err := device.SetMasterVolume(0.5); err != nil {
		t.Fatalf("SetMasterVolume: %v", err)
	}
	got, err := device.MasterVolume()
	if err != nil {
		t.Fatalf("MasterVolume: %v", err)
	}
	if math.Abs(got-0.5) > 0.01 {
		t.Fatalf("MasterVolume = %v, want ~0.5", got)
	}

	if err := device.SetMasterVolumeDB(-6); err != nil {
		t.Fatalf("SetMasterVolumeDB: %v", err)
	}
	if _, err := device.MasterVolumeDB(); err != nil {
		t.Fatalf("MasterVolumeDB: %v", err)
	}
}
