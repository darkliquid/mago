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
		DataCallback: func(_ *Device, io DeviceIO) {
			select {
			case got <- io.FrameCount():
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
		DataCallback: func(_ *Device, io DeviceIO) {
			select {
			case got <- io.FrameCount():
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

func TestDeviceIOSlicing(t *testing.T) {
	const frames = 128
	const channels = 2
	totalSamples := frames * channels

	outBuf := make([]float32, totalSamples)
	inBuf := make([]float32, totalSamples)

	io := DeviceIO{
		output:           unsafe.Pointer(&outBuf[0]),
		input:            unsafe.Pointer(&inBuf[0]),
		frameCount:       frames,
		playbackChannels: channels,
		captureChannels:  channels,
	}

	if got := io.FrameCount(); got != frames {
		t.Fatalf("FrameCount: got %d, want %d", got, frames)
	}

	outF32 := io.OutputF32()
	if len(outF32) != totalSamples {
		t.Fatalf("OutputF32 length: got %d, want %d", len(outF32), totalSamples)
	}

	inF32 := io.InputF32()
	if len(inF32) != totalSamples {
		t.Fatalf("InputF32 length: got %d, want %d", len(inF32), totalSamples)
	}

	// Capture-only device: output is nil
	ioCap := DeviceIO{
		input:           unsafe.Pointer(&inBuf[0]),
		frameCount:      frames,
		captureChannels: channels,
	}
	if ioCap.OutputF32() != nil {
		t.Errorf("expected nil OutputF32 for capture-only IO")
	}
}

func TestDeviceIDIsZero(t *testing.T) {
	var id DeviceID
	if !id.IsZero() {
		t.Errorf("expected empty DeviceID to be zero")
	}
	id[0] = 1
	if id.IsZero() {
		t.Errorf("expected non-empty DeviceID to not be zero")
	}
}
