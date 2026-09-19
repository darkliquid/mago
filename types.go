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
	magoObjectContext          int32 = 1
	magoObjectDevice           int32 = 2
	magoObjectLog              int32 = 3
	magoObjectDeviceInfo       int32 = 4
	magoObjectContextConfig    int32 = 5
	magoObjectChannelConverter int32 = 6
)

type channelConverterHandle struct{}

// Channel mirrors ma_channel, which miniaudio typedefs to ma_uint8.
type Channel uint8

const (
	ChannelNone             Channel = 0
	ChannelMono             Channel = 1
	ChannelFrontLeft        Channel = 2
	ChannelFrontRight       Channel = 3
	ChannelFrontCenter      Channel = 4
	ChannelLFE              Channel = 5
	ChannelBackLeft         Channel = 6
	ChannelBackRight        Channel = 7
	ChannelFrontLeftCenter  Channel = 8
	ChannelFrontRightCenter Channel = 9
	ChannelBackCenter       Channel = 10
	ChannelSideLeft         Channel = 11
	ChannelSideRight        Channel = 12
	ChannelTopCenter        Channel = 13
	ChannelTopFrontLeft     Channel = 14
	ChannelTopFrontCenter   Channel = 15
	ChannelTopFrontRight    Channel = 16
	ChannelTopBackLeft      Channel = 17
	ChannelTopBackCenter    Channel = 18
	ChannelTopBackRight     Channel = 19
	ChannelAux0             Channel = 20
	ChannelAux1             Channel = 21
	ChannelAux2             Channel = 22
	ChannelAux3             Channel = 23
	ChannelAux4             Channel = 24
	ChannelAux5             Channel = 25
	ChannelAux6             Channel = 26
	ChannelAux7             Channel = 27
	ChannelAux8             Channel = 28
	ChannelAux9             Channel = 29
	ChannelAux10            Channel = 30
	ChannelAux11            Channel = 31
	ChannelAux12            Channel = 32
	ChannelAux13            Channel = 33
	ChannelAux14            Channel = 34
	ChannelAux15            Channel = 35
	ChannelAux16            Channel = 36
	ChannelAux17            Channel = 37
	ChannelAux18            Channel = 38
	ChannelAux19            Channel = 39
	ChannelAux20            Channel = 40
	ChannelAux21            Channel = 41
	ChannelAux22            Channel = 42
	ChannelAux23            Channel = 43
	ChannelAux24            Channel = 44
	ChannelAux25            Channel = 45
	ChannelAux26            Channel = 46
	ChannelAux27            Channel = 47
	ChannelAux28            Channel = 48
	ChannelAux29            Channel = 49
	ChannelAux30            Channel = 50
	ChannelAux31            Channel = 51
	ChannelPositionCount    Channel = 52
	ChannelLeft             Channel = ChannelFrontLeft
	ChannelRight            Channel = ChannelFrontRight
)

// DitherMode mirrors ma_dither_mode.
type DitherMode int32

const (
	DitherModeNone      DitherMode = 0
	DitherModeRectangle DitherMode = 1
	DitherModeTriangle  DitherMode = 2
)

// ChannelMixMode mirrors ma_channel_mix_mode.
type ChannelMixMode int32

const (
	ChannelMixModeRectangular   ChannelMixMode = 0
	ChannelMixModeSimple        ChannelMixMode = 1
	ChannelMixModeCustomWeights ChannelMixMode = 2
	ChannelMixModeDefault       ChannelMixMode = ChannelMixModeRectangular
)

// StandardChannelMap mirrors ma_standard_channel_map.
type StandardChannelMap int32

const (
	StandardChannelMapMicrosoft StandardChannelMap = 0
	StandardChannelMapALSA      StandardChannelMap = 1
	StandardChannelMapRFC3551   StandardChannelMap = 2
	StandardChannelMapFLAC      StandardChannelMap = 3
	StandardChannelMapVorbis    StandardChannelMap = 4
	StandardChannelMapSound4    StandardChannelMap = 5
	StandardChannelMapSndio     StandardChannelMap = 6
	StandardChannelMapDefault   StandardChannelMap = StandardChannelMapMicrosoft
)

// channelConverterConfigNative mirrors ma_channel_converter_config. Its size and
// field offsets are validated by layout_test.go.
type channelConverterConfigNative struct {
	Format                          Format
	ChannelsIn                      uint32
	ChannelsOut                     uint32
	ChannelMapIn                    *uint8 // *ma_channel
	ChannelMapOut                   *uint8 // *ma_channel
	MixingMode                      ChannelMixMode
	CalculateLFEFromSpatialChannels uint32
	Weights                         **float32 // ppWeights, only used by custom weights
}
