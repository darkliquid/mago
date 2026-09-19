package mago

import (
	"fmt"
	"unsafe"
)

type Result int32
type Backend int32
type DeviceType int32
type ShareMode int32
type PerformanceProfile int32
type Format int32
type NotificationType uint32

type DeviceState int32

const (
	DeviceStateUninitialized DeviceState = 0
	DeviceStateStopped       DeviceState = 1
	DeviceStateStarted       DeviceState = 2
	DeviceStateStarting      DeviceState = 3
	DeviceStateStopping      DeviceState = 4
)

type Version struct {
	Major    uint32
	Minor    uint32
	Revision uint32
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Revision)
}

// streamConfigNative mirrors mago_stream_config, the bridge's own per-stream
// struct (not a miniaudio struct). Field order and widths must match
// native/miniaudio_bridge.c.
type streamConfigNative struct {
	DeviceID                  unsafe.Pointer
	DeviceIndex               int32
	Format                    Format
	Channels                  uint32
	SampleRate                uint32
	PeriodSizeInFrames        uint32
	PeriodSizeInMilliseconds  uint32
	Periods                   uint32
	PerformanceProfile        PerformanceProfile
	ShareMode                 ShareMode
	NoPreSilencedOutputBuffer uint32
	NoClip                    uint32
	NoDisableDenormals        uint32
	NoFixedSizedCallback      uint32
}

// deviceConfigNative mirrors mago_device_config.
type deviceConfigNative struct {
	DeviceType           uint32
	Playback             *streamConfigNative
	Capture              *streamConfigNative
	DataCallback         uintptr
	NotificationCallback uintptr
	UserData             uintptr
}

// deviceIDNative mirrors the ma_device_id union, whose largest member is a
// 256-byte buffer. Its size is validated by layout_test.go against the vendored
// header.
type deviceIDNative [256]byte

// deviceInfoNative mirrors the prefix of ma_device_info that enumeration needs.
// Enumeration pushes one pointer per device, so the trailing
// nativeDataFormatCount/nativeDataFormats fields are not needed and the struct
// stride is irrelevant. Offsets are validated by layout_test.go.
type deviceInfoNative struct {
	ID        deviceIDNative
	Name      [256]byte
	IsDefault uint32
}

type DeviceInfo struct {
	Name      string
	IsDefault bool
}

type contextHandle struct{}
type deviceHandle struct{}
type logHandle struct{}

// Private ABI shared with the mago_object_type enum in native/miniaudio_bridge.c.
// Keep the two in sync when adding a new object type.
const (
	magoObjectContext       int32 = 1
	magoObjectDevice        int32 = 2
	magoObjectLog           int32 = 3
	magoObjectDeviceInfo    int32 = 4
	magoObjectContextConfig int32 = 5
)
