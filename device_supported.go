//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

type DataCallback func(device *Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32)
type NotificationCallback func(device *Device, notification NotificationType)

type PlaybackDeviceConfig struct {
	DeviceIndex               int
	Format                    Format
	Channels                  uint32
	SampleRate                uint32
	PeriodSizeInFrames        uint32
	PeriodSizeInMilliseconds  uint32
	Periods                   uint32
	PerformanceProfile        PerformanceProfile
	ShareMode                 ShareMode
	NoPreSilencedOutputBuffer bool
	NoClip                    bool
	NoDisableDenormals        bool
	NoFixedSizedCallback      bool
	DataCallback              DataCallback
	NotificationCallback      NotificationCallback
}

type Device struct {
	lib    *Library
	handle *deviceHandle
	token  uintptr
}

type callbackState struct {
	device   *Device
	onData   DataCallback
	onNotify NotificationCallback
}

var (
	callbackSeq atomic.Uint64
	callbacks   sync.Map

	dataCallbackPtr = purego.NewCallback(func(token uintptr, output uintptr, input uintptr, frameCount uint32) uintptr {
		value, ok := callbacks.Load(token)
		if !ok {
			return 0
		}
		state := value.(*callbackState)
		if state.onData == nil {
			return 0
		}
		state.onData(state.device, unsafe.Pointer(output), unsafe.Pointer(input), frameCount)
		return 0
	})

	notificationCallbackPtr = purego.NewCallback(func(token uintptr, notificationType uint32) uintptr {
		value, ok := callbacks.Load(token)
		if !ok {
			return 0
		}
		state := value.(*callbackState)
		if state.onNotify == nil {
			return 0
		}
		state.onNotify(state.device, NotificationType(notificationType))
		return 0
	})
)

func DefaultPlaybackDeviceConfig() PlaybackDeviceConfig {
	return PlaybackDeviceConfig{
		DeviceIndex:        -1,
		Format:             FormatF32,
		Channels:           2,
		PerformanceProfile: PerformanceProfileLowLatency,
		ShareMode:          ShareModeShared,
	}
}

func (lib *Library) NewPlaybackDevice(ctx *Context, config PlaybackDeviceConfig) (*Device, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if ctx != nil && ctx.lib != lib {
		return nil, fmt.Errorf("mago: context belongs to a different library")
	}
	if config.DeviceIndex < -1 {
		return nil, fmt.Errorf("mago: device index must be -1 or greater")
	}
	if config.DeviceIndex > math.MaxInt32 {
		return nil, fmt.Errorf("mago: device index must be %d or less", math.MaxInt32)
	}
	if ctx == nil && config.DeviceIndex >= 0 {
		return nil, fmt.Errorf("mago: selecting a device by index requires a context")
	}

	token := uintptr(callbackSeq.Add(1))
	device := &Device{lib: lib, token: token}
	callbacks.Store(token, &callbackState{device: device, onData: config.DataCallback, onNotify: config.NotificationCallback})

	var handle *deviceHandle
	var ctxHandle *contextHandle
	if ctx != nil {
		ctxHandle = ctx.handle
	}

	dataPtr := uintptr(0)
	notifyPtr := uintptr(0)
	if config.DataCallback != nil {
		dataPtr = dataCallbackPtr
	}
	if config.NotificationCallback != nil {
		notifyPtr = notificationCallbackPtr
	}

	nativeConfig := playbackDeviceConfigNative{
		DeviceIndex:               int32(config.DeviceIndex),
		Format:                    config.Format,
		Channels:                  config.Channels,
		SampleRate:                config.SampleRate,
		PeriodSizeInFrames:        config.PeriodSizeInFrames,
		PeriodSizeInMilliseconds:  config.PeriodSizeInMilliseconds,
		Periods:                   config.Periods,
		PerformanceProfile:        config.PerformanceProfile,
		ShareMode:                 config.ShareMode,
		NoPreSilencedOutputBuffer: boolToBool32(config.NoPreSilencedOutputBuffer),
		NoClip:                    boolToBool32(config.NoClip),
		NoDisableDenormals:        boolToBool32(config.NoDisableDenormals),
		NoFixedSizedCallback:      boolToBool32(config.NoFixedSizedCallback),
		DataCallback:              dataPtr,
		NotificationCallback:      notifyPtr,
		UserData:                  token,
	}

	result := lib.bindings.magoDeviceInitPlayback(ctxHandle, &nativeConfig, &handle)
	if result != Success {
		callbacks.Delete(token)
		return nil, lib.resultError("ma_device_init", result)
	}

	device.handle = handle
	return device, nil
}

func (ctx *Context) NewPlaybackDevice(config PlaybackDeviceConfig) (*Device, error) {
	if ctx == nil {
		return nil, fmt.Errorf("mago: nil context")
	}
	return ctx.lib.NewPlaybackDevice(ctx, config)
}

func (device *Device) Start() error {
	if device == nil || device.handle == nil {
		return fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return err
	}
	return device.lib.resultError("ma_device_start", device.lib.bindings.maDeviceStart(device.handle))
}

func (device *Device) Stop() error {
	if device == nil || device.handle == nil {
		return fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return err
	}
	return device.lib.resultError("ma_device_stop", device.lib.bindings.maDeviceStop(device.handle))
}

func (device *Device) Close() error {
	if device == nil || device.handle == nil {
		return nil
	}
	if err := device.lib.ensureOpen(); err != nil {
		return err
	}

	device.lib.bindings.magoDeviceUninitFree(device.handle)
	callbacks.Delete(device.token)
	device.handle = nil
	return nil
}

func boolToBool32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}
