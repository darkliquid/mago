package mago

import (
	"testing"
	"time"
	"unsafe"
)

func TestNullBackendCaptureDevice(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	got := make(chan uint32, 8)
	device, err := ctx.NewDevice(DeviceConfig{
		Type: DeviceTypeCapture,
		Capture: &StreamConfig{
			DeviceIndex:        -1,
			Channels:           1,
			SampleRate:         48000,
			PeriodSizeInFrames: 64,
		},
		DataCallback: func(_ *Device, _ unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
			select {
			case got <- frameCount:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewDevice(capture): %v", err)
	}
	defer func() { _ = device.Close() }()

	if err := device.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = device.Stop() }()

	select {
	case n := <-got:
		if n == 0 {
			t.Fatal("expected non-zero capture frame count")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for capture callback")
	}
}

func TestNewDeviceValidatesConfig(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	cases := map[string]DeviceConfig{
		"playback without playback config": {Type: DeviceTypePlayback},
		"capture without capture config":   {Type: DeviceTypeCapture},
		"duplex with one config":           {Type: DeviceTypeDuplex, Playback: &StreamConfig{DeviceIndex: -1}},
		"unsupported type":                 {Type: DeviceType(99)},
		"index without context":            {Type: DeviceTypePlayback, Playback: &StreamConfig{DeviceIndex: 0}},
	}

	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			arg := config
			if name == "index without context" {
				// Deliberately pass a nil context to exercise the guard.
				if _, err := lib.NewDevice(nil, arg); err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if _, err := ctx.NewDevice(arg); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestNullBackendDuplexDevice(t *testing.T) {
	lib := newNullLibrary(t)
	ctx := newNullContext(t, lib)

	got := make(chan uint32, 8)
	device, err := ctx.NewDevice(DeviceConfig{
		Type:     DeviceTypeDuplex,
		Playback: &StreamConfig{DeviceIndex: -1, Channels: 1, SampleRate: 48000, PeriodSizeInFrames: 64},
		Capture:  &StreamConfig{DeviceIndex: -1, Channels: 1, SampleRate: 48000, PeriodSizeInFrames: 64},
		DataCallback: func(_ *Device, _ unsafe.Pointer, _ unsafe.Pointer, frameCount uint32) {
			select {
			case got <- frameCount:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewDevice(duplex): %v", err)
	}
	defer func() { _ = device.Close() }()

	if err := device.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = device.Stop() }()

	select {
	case n := <-got:
		if n == 0 {
			t.Fatal("expected non-zero duplex frame count")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for duplex callback")
	}
}
