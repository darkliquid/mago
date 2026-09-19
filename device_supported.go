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

// StreamConfig configures one side of a device (playback or capture).
type StreamConfig struct {
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
}

// DeviceConfig describes a device of any type. Playback is required for
// playback and duplex devices; Capture is required for capture, duplex and
// loopback devices. The callbacks apply to the device as a whole.
type DeviceConfig struct {
	Type                 DeviceType
	Playback             *StreamConfig
	Capture              *StreamConfig
	DataCallback         DataCallback
	NotificationCallback NotificationCallback
}

type Device struct {
	lib         *Library
	handle      *deviceHandle
	token       uintptr
	primaryType DeviceType
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

	// enumerateCallbackPtr backs ma_context_enumerate_devices. miniaudio passes
	// the device info by pointer and our token as user data, so no C trampoline
	// is needed and the full ma_device_info layout is never mirrored.
	enumerateCallbackPtr = purego.NewCallback(func(_ uintptr, deviceType uint32, info uintptr, token uintptr) uintptr {
		value, ok := callbacks.Load(token)
		if !ok || info == 0 {
			return 1
		}
		collector, ok := value.(*deviceEnumerator)
		if !ok {
			return 1
		}
		item := copyDeviceInfo((*deviceInfoNative)(unsafe.Pointer(info)))
		switch DeviceType(deviceType) {
		case DeviceTypePlayback:
			collector.playback = append(collector.playback, item)
		case DeviceTypeCapture:
			collector.capture = append(collector.capture, item)
		}
		return 1 // continue enumeration
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

// NewDevice creates a device of the configured type. Playback must be set for
// playback and duplex devices; Capture must be set for capture, duplex and
// loopback devices.
func (lib *Library) NewDevice(ctx *Context, config DeviceConfig) (*Device, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if ctx != nil && ctx.lib != lib {
		return nil, fmt.Errorf("mago: context belongs to a different library")
	}

	switch config.Type {
	case DeviceTypePlayback:
		if config.Playback == nil {
			return nil, fmt.Errorf("mago: playback config is required for a playback device")
		}
		if config.Capture != nil {
			return nil, fmt.Errorf("mago: capture config is not valid for a playback device")
		}
	case DeviceTypeCapture, DeviceTypeLoopback:
		if config.Capture == nil {
			return nil, fmt.Errorf("mago: capture config is required for a %s device", deviceTypeName(config.Type))
		}
		if config.Playback != nil {
			return nil, fmt.Errorf("mago: playback config is not valid for a %s device", deviceTypeName(config.Type))
		}
	case DeviceTypeDuplex:
		if config.Playback == nil || config.Capture == nil {
			return nil, fmt.Errorf("mago: duplex devices require both playback and capture configs")
		}
	default:
		return nil, fmt.Errorf("mago: unsupported device type %d", config.Type)
	}

	for _, stream := range []*StreamConfig{config.Playback, config.Capture} {
		if stream == nil {
			continue
		}
		if stream.DeviceIndex < -1 {
			return nil, fmt.Errorf("mago: device index must be -1 or greater")
		}
		if stream.DeviceIndex > math.MaxInt32 {
			return nil, fmt.Errorf("mago: device index must be %d or less", math.MaxInt32)
		}
		if ctx == nil && stream.DeviceIndex >= 0 {
			return nil, fmt.Errorf("mago: selecting a device by index requires a context")
		}
	}

	token := uintptr(callbackSeq.Add(1))
	device := &Device{lib: lib, token: token, primaryType: config.Type}
	callbacks.Store(token, &callbackState{device: device, onData: config.DataCallback, onNotify: config.NotificationCallback})

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

	nativeConfig := deviceConfigNative{
		DeviceType:           uint32(config.Type), //nolint:gosec // validated to one of the four DeviceType values above
		Playback:             streamToNative(config.Playback),
		Capture:              streamToNative(config.Capture),
		DataCallback:         dataPtr,
		NotificationCallback: notifyPtr,
		UserData:             token,
	}

	var handle *deviceHandle
	result := lib.bindings.magoDeviceInit(ctxHandle, &nativeConfig, &handle)
	if result != Success {
		callbacks.Delete(token)
		return nil, lib.resultError("ma_device_init", result)
	}

	device.handle = handle
	return device, nil
}

func streamToNative(stream *StreamConfig) *streamConfigNative {
	if stream == nil {
		return nil
	}
	return &streamConfigNative{
		DeviceIndex:               int32(stream.DeviceIndex), //nolint:gosec // bounded by the MaxInt32 check in NewDevice
		Format:                    stream.Format,
		Channels:                  stream.Channels,
		SampleRate:                stream.SampleRate,
		PeriodSizeInFrames:        stream.PeriodSizeInFrames,
		PeriodSizeInMilliseconds:  stream.PeriodSizeInMilliseconds,
		Periods:                   stream.Periods,
		PerformanceProfile:        stream.PerformanceProfile,
		ShareMode:                 stream.ShareMode,
		NoPreSilencedOutputBuffer: boolToBool32(stream.NoPreSilencedOutputBuffer),
		NoClip:                    boolToBool32(stream.NoClip),
		NoDisableDenormals:        boolToBool32(stream.NoDisableDenormals),
		NoFixedSizedCallback:      boolToBool32(stream.NoFixedSizedCallback),
	}
}

func deviceTypeName(t DeviceType) string {
	switch t {
	case DeviceTypePlayback:
		return "playback"
	case DeviceTypeCapture:
		return "capture"
	case DeviceTypeDuplex:
		return "duplex"
	case DeviceTypeLoopback:
		return "loopback"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

// NewPlaybackDevice is the playback-only convenience form of NewDevice.
func (lib *Library) NewPlaybackDevice(ctx *Context, config PlaybackDeviceConfig) (*Device, error) {
	stream := StreamConfig{
		DeviceIndex:               config.DeviceIndex,
		Format:                    config.Format,
		Channels:                  config.Channels,
		SampleRate:                config.SampleRate,
		PeriodSizeInFrames:        config.PeriodSizeInFrames,
		PeriodSizeInMilliseconds:  config.PeriodSizeInMilliseconds,
		Periods:                   config.Periods,
		PerformanceProfile:        config.PerformanceProfile,
		ShareMode:                 config.ShareMode,
		NoPreSilencedOutputBuffer: config.NoPreSilencedOutputBuffer,
		NoClip:                    config.NoClip,
		NoDisableDenormals:        config.NoDisableDenormals,
		NoFixedSizedCallback:      config.NoFixedSizedCallback,
	}
	return lib.NewDevice(ctx, DeviceConfig{
		Type:                 DeviceTypePlayback,
		Playback:             &stream,
		DataCallback:         config.DataCallback,
		NotificationCallback: config.NotificationCallback,
	})
}

func (ctx *Context) NewDevice(config DeviceConfig) (*Device, error) {
	if ctx == nil {
		return nil, fmt.Errorf("mago: nil context")
	}
	return ctx.lib.NewDevice(ctx, config)
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

// State reports the device's current state.
func (device *Device) State() DeviceState {
	if device == nil || device.handle == nil {
		return DeviceStateUninitialized
	}
	return device.lib.bindings.maDeviceGetState(device.handle)
}

// Name reports the device name for its primary type.
func (device *Device) Name() (string, error) {
	return device.NameFor(device.primaryType)
}

// NameFor reports the device name for a specific stream type.
func (device *Device) NameFor(t DeviceType) (string, error) {
	if device == nil || device.handle == nil {
		return "", fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return "", err
	}

	var buffer [256]byte
	var length uintptr
	result := device.lib.bindings.maDeviceGetName(device.handle, t, &buffer[0], uintptr(len(buffer)), &length)
	if result != Success {
		return "", device.lib.resultError("ma_device_get_name", result)
	}
	if length > uintptr(len(buffer)) {
		length = uintptr(len(buffer))
	}
	return string(buffer[:length]), nil
}

// Info reports basic information about the device for its primary type.
func (device *Device) Info() (DeviceInfo, error) {
	return device.InfoFor(device.primaryType)
}

// InfoFor reports basic information about the device for a specific stream type.
func (device *Device) InfoFor(t DeviceType) (DeviceInfo, error) {
	if device == nil || device.handle == nil {
		return DeviceInfo{}, fmt.Errorf("mago: nil device")
	}
	if err := device.lib.ensureOpen(); err != nil {
		return DeviceInfo{}, err
	}

	native := (*deviceInfoNative)(device.lib.bindings.magoAlloc(magoObjectDeviceInfo))
	if native == nil {
		return DeviceInfo{}, fmt.Errorf("mago: allocate device info: out of memory")
	}
	defer device.lib.bindings.magoFree(unsafe.Pointer(native))

	if result := device.lib.bindings.maDeviceGetInfo(device.handle, t, native); result != Success {
		return DeviceInfo{}, device.lib.resultError("ma_device_get_info", result)
	}
	return copyDeviceInfo(native), nil
}

// Log returns the device's log, or nil when none is attached. The result is
// borrowed: closing it only detaches the handle.
func (device *Device) Log() *Log {
	if device == nil || device.handle == nil {
		return nil
	}
	return borrowedLog(device.lib, device.lib.bindings.maDeviceGetLog(device.handle))
}

// Context returns the context that owns this device. The result is borrowed:
// closing it only detaches the handle.
func (device *Device) Context() *Context {
	if device == nil || device.handle == nil {
		return nil
	}
	handle := device.lib.bindings.maDeviceGetContext(device.handle)
	if handle == nil {
		return nil
	}
	return &Context{lib: device.lib, handle: handle, owned: false}
}

func boolToBool32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}
