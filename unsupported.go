//go:build !darwin && !freebsd && !linux && !netbsd && !windows

package mago

import (
	"errors"
	"unsafe"
)

var errUnsupportedPlatform = errors.New("mago: this package is currently supported on darwin, freebsd, linux, netbsd, and windows")

type Library struct{}
type Context struct{}
type Device struct{}

type LibraryOption func(*struct{})
type DataCallback func(*Device, unsafe.Pointer, unsafe.Pointer, uint32)
type NotificationCallback func(*Device, NotificationType)
type PlaybackDeviceConfig struct{}
type StreamConfig struct{}
type DeviceConfig struct{}

type OpError struct {
	Op          string
	Code        Result
	Description string
}

func (e *OpError) Error() string                        { return errUnsupportedPlatform.Error() }
func WithLibraryPath(string) LibraryOption              { return func(*struct{}) {} }
func Open(...LibraryOption) (*Library, error)           { return nil, errUnsupportedPlatform }
func DefaultPlaybackDeviceConfig() PlaybackDeviceConfig { return PlaybackDeviceConfig{} }

func (*Library) NewDevice(*Context, DeviceConfig) (*Device, error) {
	return nil, errUnsupportedPlatform
}
func (*Context) NewDevice(DeviceConfig) (*Device, error) { return nil, errUnsupportedPlatform }
func (*Library) NewContextWithLog(*Log, ...Backend) (*Context, error) {
	return nil, errUnsupportedPlatform
}
func (*Context) DeviceInfo(DeviceType, unsafe.Pointer) (DeviceInfo, error) {
	return DeviceInfo{}, errUnsupportedPlatform
}
func (*Library) NewPlaybackDevice(*Context, PlaybackDeviceConfig) (*Device, error) {
	return nil, errUnsupportedPlatform
}
func (*Context) NewPlaybackDevice(PlaybackDeviceConfig) (*Device, error) {
	return nil, errUnsupportedPlatform
}

type LogLevel uint32

const (
	LogLevelDebug   LogLevel = 0
	LogLevelInfo    LogLevel = 1
	LogLevelWarning LogLevel = 2
	LogLevelError   LogLevel = 3
)

type LogCallback func(level LogLevel, message string)

type Log struct{}

func (lib *Library) NewLog() (*Log, error)           { return nil, errUnsupportedPlatform }
func (l *Log) Close() error                          { return errUnsupportedPlatform }
func (l *Log) Register(LogCallback) (uintptr, error) { return 0, errUnsupportedPlatform }
func (l *Log) Unregister(uintptr) error              { return errUnsupportedPlatform }
func (l *Log) Post(LogLevel, string) error           { return errUnsupportedPlatform }
func (l *Log) LevelString(LogLevel) string           { return "" }

type ChannelMap struct{}
type ChannelConverterConfig struct{}
type ChannelConverter struct{}

func (*Library) NewStandardChannelMap(StandardChannelMap, uint32) ChannelMap { return ChannelMap{} }
func (*Library) NewBlankChannelMap(uint32) ChannelMap                        { return ChannelMap{} }
func (ChannelMap) Len() int                                                  { return 0 }
func (ChannelMap) Channels() []Channel                                       { return nil }
func (ChannelMap) Get(int) Channel                                           { return ChannelNone }
func (ChannelMap) Clone() ChannelMap                                         { return ChannelMap{} }
func (ChannelMap) String() string                                            { return "" }

func (*Library) NewChannelConverter(ChannelConverterConfig) (*ChannelConverter, error) {
	return nil, errUnsupportedPlatform
}
func (*ChannelConverter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*ChannelConverter) InputChannelMap() (ChannelMap, error) {
	return ChannelMap{}, errUnsupportedPlatform
}
func (*ChannelConverter) OutputChannelMap() (ChannelMap, error) {
	return ChannelMap{}, errUnsupportedPlatform
}
func (*ChannelConverter) Close() error { return errUnsupportedPlatform }

func (*Library) ConvertPCMSamples(unsafe.Pointer, Format, unsafe.Pointer, Format, uint64, DitherMode) error {
	return errUnsupportedPlatform
}
func (*Library) ConvertPCMFramesFormat(unsafe.Pointer, Format, unsafe.Pointer, Format, uint64, uint32, DitherMode) error {
	return errUnsupportedPlatform
}
func (*Library) ConvertFrames(unsafe.Pointer, uint64, Format, uint32, uint32, unsafe.Pointer, uint64, Format, uint32, uint32) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func BytesPerSample(Format) uint32 { return 0 }

type ResamplerConfig struct{}
type LinearResamplerConfig struct{}
type DataConverterConfig struct{}
type Resampler struct{}
type LinearResampler struct{}
type DataConverter struct{}

func DefaultResamplerConfig(Format, uint32, uint32, uint32) ResamplerConfig {
	return ResamplerConfig{}
}
func DefaultLinearResamplerConfig(Format, uint32, uint32, uint32) LinearResamplerConfig {
	return LinearResamplerConfig{}
}
func DefaultDataConverterConfig(Format, Format, uint32, uint32, uint32, uint32) DataConverterConfig {
	return DataConverterConfig{}
}

func (*Library) NewResampler(ResamplerConfig) (*Resampler, error) {
	return nil, errUnsupportedPlatform
}
func (*Resampler) ProcessPCMFrames(unsafe.Pointer, uint64, unsafe.Pointer, uint64) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*Resampler) SetRate(uint32, uint32) error { return errUnsupportedPlatform }
func (*Resampler) SetRateRatio(float64) error   { return errUnsupportedPlatform }
func (*Resampler) Reset() error                 { return errUnsupportedPlatform }
func (*Resampler) RequiredInputFrameCount(uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*Resampler) ExpectedOutputFrameCount(uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*Resampler) Close() error { return errUnsupportedPlatform }

func (*Library) NewLinearResampler(LinearResamplerConfig) (*LinearResampler, error) {
	return nil, errUnsupportedPlatform
}
func (*LinearResampler) ProcessPCMFrames(unsafe.Pointer, uint64, unsafe.Pointer, uint64) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*LinearResampler) SetRate(uint32, uint32) error { return errUnsupportedPlatform }
func (*LinearResampler) SetRateRatio(float64) error   { return errUnsupportedPlatform }
func (*LinearResampler) Reset() error                 { return errUnsupportedPlatform }
func (*LinearResampler) RequiredInputFrameCount(uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*LinearResampler) ExpectedOutputFrameCount(uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*LinearResampler) Close() error { return errUnsupportedPlatform }

func (*Library) NewDataConverter(DataConverterConfig) (*DataConverter, error) {
	return nil, errUnsupportedPlatform
}
func (*DataConverter) ProcessPCMFrames(unsafe.Pointer, uint64, unsafe.Pointer, uint64) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*DataConverter) SetRate(uint32, uint32) error { return errUnsupportedPlatform }
func (*DataConverter) SetRateRatio(float64) error   { return errUnsupportedPlatform }
func (*DataConverter) Reset() error                 { return errUnsupportedPlatform }
func (*DataConverter) RequiredInputFrameCount(uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*DataConverter) ExpectedOutputFrameCount(uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*DataConverter) InputChannelMap() (ChannelMap, error) {
	return ChannelMap{}, errUnsupportedPlatform
}
func (*DataConverter) OutputChannelMap() (ChannelMap, error) {
	return ChannelMap{}, errUnsupportedPlatform
}
func (*DataConverter) Close() error { return errUnsupportedPlatform }
