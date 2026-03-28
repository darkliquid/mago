//go:build linux

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

type Version struct {
	Major    uint32
	Minor    uint32
	Revision uint32
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Revision)
}

type playbackDeviceConfigNative struct {
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
	DataCallback              uintptr
	NotificationCallback      uintptr
	UserData                  uintptr
}

type deviceInfoNative struct {
	Name      [256]byte
	IsDefault uint32
}

type DeviceInfo struct {
	Name      string
	IsDefault bool
}

type contextHandle struct{}
type deviceHandle struct{}
