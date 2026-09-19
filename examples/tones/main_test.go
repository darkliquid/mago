package main

import (
	"testing"
	"time"
	"unsafe"

	"github.com/darkliquid/mago"
)

func TestTonesWithNullBackend(t *testing.T) {
	lib, err := mago.Open()
	if err != nil {
		t.Skipf("mago.Open: %v (embedded library not available on this platform)", err)
	}
	defer func() { _ = lib.Close() }()

	ctx, err := lib.NewContext(mago.BackendNull)
	if err != nil {
		t.Fatalf("NewContext(BackendNull): %v", err)
	}
	defer func() { _ = ctx.Close() }()

	waveform, err := lib.NewWaveform(mago.WaveformConfig{
		Format:     mago.FormatF32,
		Channels:   2,
		SampleRate: 48000,
		Type:       mago.WaveformTypeSine,
		Amplitude:  0.2,
		Frequency:  440.0,
	})
	if err != nil {
		t.Fatalf("NewWaveform: %v", err)
	}
	defer func() { _ = waveform.Close() }()

	config := mago.DefaultPlaybackDeviceConfig()
	config.Channels = 2
	config.SampleRate = 48000
	config.PeriodSizeInFrames = 256
	config.DataCallback = func(_ *mago.Device, output unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
		_, _ = waveform.ReadPCMFrames(output, uint64(frameCount))
	}

	device, err := ctx.NewPlaybackDevice(config)
	if err != nil {
		t.Fatalf("NewPlaybackDevice: %v", err)
	}
	defer func() { _ = device.Close() }()

	if err := device.Start(); err != nil {
		t.Fatalf("device.Start: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := device.Stop(); err != nil {
		t.Fatalf("device.Stop: %v", err)
	}
}
