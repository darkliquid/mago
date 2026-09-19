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
type DeviceIO struct{}

func (DeviceIO) FrameCount() uint32   { return 0 }
func (DeviceIO) OutputF32() []float32 { return nil }
func (DeviceIO) OutputS16() []int16   { return nil }
func (DeviceIO) OutputBytes() []byte  { return nil }
func (DeviceIO) InputF32() []float32  { return nil }
func (DeviceIO) InputS16() []int16    { return nil }
func (DeviceIO) InputBytes() []byte   { return nil }

type DataCallback func(*Device, DeviceIO)
type NotificationCallback func(*Device, NotificationType)
type PlaybackDeviceConfig struct {
	DeviceID *DeviceID
}
type StreamConfig struct {
	DeviceID *DeviceID
}
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
func (*Context) DeviceInfo(DeviceType, *DeviceID) (DeviceInfo, error) {
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

type AudioBufferConfig struct{}
type AudioBuffer struct{}
type AudioBufferRef struct{}
type RingBuffer struct{}
type PCMRingBuffer struct{}

func (*Library) NewAudioBuffer(AudioBufferConfig) (*AudioBuffer, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewAudioBufferCopy(AudioBufferConfig) (*AudioBuffer, error) {
	return nil, errUnsupportedPlatform
}
func (*AudioBuffer) ReadPCMFrames(unsafe.Pointer, uint64, bool) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*AudioBuffer) SeekToPCMFrame(uint64) error          { return errUnsupportedPlatform }
func (*AudioBuffer) Map() (unsafe.Pointer, uint64, error) { return nil, 0, errUnsupportedPlatform }
func (*AudioBuffer) Unmap(uint64) error                   { return errUnsupportedPlatform }
func (*AudioBuffer) CursorInPCMFrames() (uint64, error)   { return 0, errUnsupportedPlatform }
func (*AudioBuffer) LengthInPCMFrames() (uint64, error)   { return 0, errUnsupportedPlatform }
func (*AudioBuffer) AvailableFrames() (uint64, error)     { return 0, errUnsupportedPlatform }
func (*AudioBuffer) Close() error                         { return errUnsupportedPlatform }

func (*Library) NewAudioBufferRef(Format, uint32, unsafe.Pointer, uint64, ...any) (*AudioBufferRef, error) {
	return nil, errUnsupportedPlatform
}
func (*AudioBufferRef) SetData(unsafe.Pointer, uint64, ...any) error { return errUnsupportedPlatform }
func (*AudioBufferRef) ReadPCMFrames(unsafe.Pointer, uint64, bool) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*AudioBufferRef) SeekToPCMFrame(uint64) error { return errUnsupportedPlatform }
func (*AudioBufferRef) Map() (unsafe.Pointer, uint64, error) {
	return nil, 0, errUnsupportedPlatform
}
func (*AudioBufferRef) Unmap(uint64) error                 { return errUnsupportedPlatform }
func (*AudioBufferRef) AtEnd() bool                        { return true }
func (*AudioBufferRef) CursorInPCMFrames() (uint64, error) { return 0, errUnsupportedPlatform }
func (*AudioBufferRef) LengthInPCMFrames() (uint64, error) { return 0, errUnsupportedPlatform }
func (*AudioBufferRef) AvailableFrames() (uint64, error)   { return 0, errUnsupportedPlatform }
func (*AudioBufferRef) Close() error                       { return errUnsupportedPlatform }

func (*Library) NewRingBuffer(uint) (*RingBuffer, error) { return nil, errUnsupportedPlatform }
func (*Library) NewRingBufferEx(uint, uint, uint) (*RingBuffer, error) {
	return nil, errUnsupportedPlatform
}
func (*RingBuffer) AcquireRead(uint) (unsafe.Pointer, uint, error) {
	return nil, 0, errUnsupportedPlatform
}
func (*RingBuffer) AcquireWrite(uint) (unsafe.Pointer, uint, error) {
	return nil, 0, errUnsupportedPlatform
}
func (*RingBuffer) CommitRead(uint) error  { return errUnsupportedPlatform }
func (*RingBuffer) CommitWrite(uint) error { return errUnsupportedPlatform }
func (*RingBuffer) SeekRead(uint) error    { return errUnsupportedPlatform }
func (*RingBuffer) SeekWrite(uint) error   { return errUnsupportedPlatform }
func (*RingBuffer) Reset()                 {}
func (*RingBuffer) PointerDistance() int32 { return 0 }
func (*RingBuffer) AvailableRead() uint32  { return 0 }
func (*RingBuffer) AvailableWrite() uint32 { return 0 }
func (*RingBuffer) Close() error           { return errUnsupportedPlatform }

func (*Library) NewPCMRingBuffer(Format, uint32, uint32) (*PCMRingBuffer, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewPCMRingBufferEx(Format, uint32, uint32, uint32, uint32) (*PCMRingBuffer, error) {
	return nil, errUnsupportedPlatform
}
func (*PCMRingBuffer) AcquireRead(uint32) (unsafe.Pointer, uint32, error) {
	return nil, 0, errUnsupportedPlatform
}
func (*PCMRingBuffer) AcquireWrite(uint32) (unsafe.Pointer, uint32, error) {
	return nil, 0, errUnsupportedPlatform
}
func (*PCMRingBuffer) CommitRead(uint32) error  { return errUnsupportedPlatform }
func (*PCMRingBuffer) CommitWrite(uint32) error { return errUnsupportedPlatform }
func (*PCMRingBuffer) SeekRead(uint32) error    { return errUnsupportedPlatform }
func (*PCMRingBuffer) SeekWrite(uint32) error   { return errUnsupportedPlatform }
func (*PCMRingBuffer) Reset()                   {}
func (*PCMRingBuffer) PointerDistance() int32   { return 0 }
func (*PCMRingBuffer) AvailableRead() uint32    { return 0 }
func (*PCMRingBuffer) AvailableWrite() uint32   { return 0 }
func (*PCMRingBuffer) Format() Format           { return FormatUnknown }
func (*PCMRingBuffer) Channels() uint32         { return 0 }
func (*PCMRingBuffer) SampleRate() uint32       { return 0 }
func (*PCMRingBuffer) Close() error             { return errUnsupportedPlatform }

type WaveformConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Type       WaveformType
	Amplitude  float64
	Frequency  float64
}
type Waveform struct{}

func (*Library) NewWaveform(WaveformConfig) (*Waveform, error) { return nil, errUnsupportedPlatform }
func (*Waveform) ReadPCMFrames(unsafe.Pointer, uint64) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*Waveform) SeekToPCMFrame(uint64) error { return errUnsupportedPlatform }
func (*Waveform) SetAmplitude(float64) error  { return errUnsupportedPlatform }
func (*Waveform) SetFrequency(float64) error  { return errUnsupportedPlatform }
func (*Waveform) SetType(WaveformType) error  { return errUnsupportedPlatform }
func (*Waveform) SetSampleRate(uint32) error  { return errUnsupportedPlatform }
func (*Waveform) Close() error                { return errUnsupportedPlatform }

type NoiseConfig struct {
	Format            Format
	Channels          uint32
	Type              NoiseType
	Seed              int32
	Amplitude         float64
	DuplicateChannels bool
}
type Noise struct{}

func (*Library) NewNoise(NoiseConfig) (*Noise, error)               { return nil, errUnsupportedPlatform }
func (*Noise) ReadPCMFrames(unsafe.Pointer, uint64) (uint64, error) { return 0, errUnsupportedPlatform }
func (*Noise) SetAmplitude(float64) error                           { return errUnsupportedPlatform }
func (*Noise) SetSeed(int32) error                                  { return errUnsupportedPlatform }
func (*Noise) SetType(NoiseType) error                              { return errUnsupportedPlatform }
func (*Noise) Close() error                                         { return errUnsupportedPlatform }

type BiquadConfig struct {
	Format   Format
	Channels uint32
	B0       float64
	B1       float64
	B2       float64
	A0       float64
	A1       float64
	A2       float64
}
type Biquad struct{}

func (*Library) NewBiquad(BiquadConfig) (*Biquad, error) { return nil, errUnsupportedPlatform }
func (*Biquad) Reinit(BiquadConfig) error                { return errUnsupportedPlatform }
func (*Biquad) ClearCache() error                        { return errUnsupportedPlatform }
func (*Biquad) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*Biquad) Latency() uint32 { return 0 }
func (*Biquad) Close() error    { return errUnsupportedPlatform }

type LowPassFilter1Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
}
type LowPassFilter2Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}
type LowPassFilterConfig struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

type (
	LPF1Config = LowPassFilter1Config
	LPF2Config = LowPassFilter2Config
	LPFConfig  = LowPassFilterConfig
	LPF1       = LowPassFilter1
	LPF2       = LowPassFilter2
	LPF        = LowPassFilter
)

type LowPassFilter1 struct{}
type LowPassFilter2 struct{}
type LowPassFilter struct{}

func (*Library) NewLowPassFilter1(LowPassFilter1Config) (*LowPassFilter1, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewLPF1(LowPassFilter1Config) (*LowPassFilter1, error) {
	return nil, errUnsupportedPlatform
}
func (*LowPassFilter1) Reinit(LowPassFilter1Config) error { return errUnsupportedPlatform }
func (*LowPassFilter1) ClearCache() error                 { return errUnsupportedPlatform }
func (*LowPassFilter1) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*LowPassFilter1) Latency() uint32 { return 0 }
func (*LowPassFilter1) Close() error    { return errUnsupportedPlatform }

func (*Library) NewLowPassFilter2(LowPassFilter2Config) (*LowPassFilter2, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewLPF2(LowPassFilter2Config) (*LowPassFilter2, error) {
	return nil, errUnsupportedPlatform
}
func (*LowPassFilter2) Reinit(LowPassFilter2Config) error { return errUnsupportedPlatform }
func (*LowPassFilter2) ClearCache() error                 { return errUnsupportedPlatform }
func (*LowPassFilter2) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*LowPassFilter2) Latency() uint32 { return 0 }
func (*LowPassFilter2) Close() error    { return errUnsupportedPlatform }

func (*Library) NewLowPassFilter(LowPassFilterConfig) (*LowPassFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewLPF(LowPassFilterConfig) (*LowPassFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*LowPassFilter) Reinit(LowPassFilterConfig) error { return errUnsupportedPlatform }
func (*LowPassFilter) ClearCache() error                { return errUnsupportedPlatform }
func (*LowPassFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*LowPassFilter) Latency() uint32 { return 0 }
func (*LowPassFilter) Close() error    { return errUnsupportedPlatform }

type HighPassFilter1Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
}

type HighPassFilter2Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}

type HighPassFilterConfig struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

type (
	HPF1Config = HighPassFilter1Config
	HPF2Config = HighPassFilter2Config
	HPFConfig  = HighPassFilterConfig
	HPF1       = HighPassFilter1
	HPF2       = HighPassFilter2
	HPF        = HighPassFilter
)

type HighPassFilter1 struct{}
type HighPassFilter2 struct{}
type HighPassFilter struct{}

func (*Library) NewHighPassFilter1(HighPassFilter1Config) (*HighPassFilter1, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewHPF1(HighPassFilter1Config) (*HighPassFilter1, error) {
	return nil, errUnsupportedPlatform
}
func (*HighPassFilter1) Reinit(HighPassFilter1Config) error { return errUnsupportedPlatform }
func (*HighPassFilter1) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*HighPassFilter1) Latency() uint32 { return 0 }
func (*HighPassFilter1) Close() error    { return errUnsupportedPlatform }

func (*Library) NewHighPassFilter2(HighPassFilter2Config) (*HighPassFilter2, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewHPF2(HighPassFilter2Config) (*HighPassFilter2, error) {
	return nil, errUnsupportedPlatform
}
func (*HighPassFilter2) Reinit(HighPassFilter2Config) error { return errUnsupportedPlatform }
func (*HighPassFilter2) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*HighPassFilter2) Latency() uint32 { return 0 }
func (*HighPassFilter2) Close() error    { return errUnsupportedPlatform }

func (*Library) NewHighPassFilter(HighPassFilterConfig) (*HighPassFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewHPF(HighPassFilterConfig) (*HighPassFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*HighPassFilter) Reinit(HighPassFilterConfig) error { return errUnsupportedPlatform }
func (*HighPassFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*HighPassFilter) Latency() uint32 { return 0 }
func (*HighPassFilter) Close() error    { return errUnsupportedPlatform }

type BandPassFilter2Config struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}

type BandPassFilterConfig struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

type (
	BPF2Config = BandPassFilter2Config
	BPFConfig  = BandPassFilterConfig
	BPF2       = BandPassFilter2
	BPF        = BandPassFilter
)

type BandPassFilter2 struct{}
type BandPassFilter struct{}

func (*Library) NewBandPassFilter2(BandPassFilter2Config) (*BandPassFilter2, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewBPF2(BandPassFilter2Config) (*BandPassFilter2, error) {
	return nil, errUnsupportedPlatform
}
func (*BandPassFilter2) Reinit(BandPassFilter2Config) error { return errUnsupportedPlatform }
func (*BandPassFilter2) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*BandPassFilter2) Latency() uint32 { return 0 }
func (*BandPassFilter2) Close() error    { return errUnsupportedPlatform }

func (*Library) NewBandPassFilter(BandPassFilterConfig) (*BandPassFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewBPF(BandPassFilterConfig) (*BandPassFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*BandPassFilter) Reinit(BandPassFilterConfig) error { return errUnsupportedPlatform }
func (*BandPassFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*BandPassFilter) Latency() uint32 { return 0 }
func (*BandPassFilter) Close() error    { return errUnsupportedPlatform }

type NotchFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Q          float64
	Frequency  float64
}

type PeakFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	Q          float64
	Frequency  float64
}

type LowShelfFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

type HighShelfFilterConfig struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

type (
	Notch2Config     = NotchFilterConfig
	Peak2Config      = PeakFilterConfig
	LoShelf2Config   = LowShelfFilterConfig
	LowShelf2Config  = LowShelfFilterConfig
	HiShelf2Config   = HighShelfFilterConfig
	HighShelf2Config = HighShelfFilterConfig
	Notch2           = NotchFilter
	Peak2            = PeakFilter
	LoShelf2         = LowShelfFilter
	LowShelf2        = LowShelfFilter
	HiShelf2         = HighShelfFilter
	HighShelf2       = HighShelfFilter
)

type NotchFilter struct{}
type PeakFilter struct{}
type LowShelfFilter struct{}
type HighShelfFilter struct{}

func (*Library) NewNotchFilter(NotchFilterConfig) (*NotchFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewNotch2(NotchFilterConfig) (*NotchFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*NotchFilter) Reinit(NotchFilterConfig) error { return errUnsupportedPlatform }
func (*NotchFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*NotchFilter) Latency() uint32 { return 0 }
func (*NotchFilter) Close() error    { return errUnsupportedPlatform }

func (*Library) NewPeakFilter(PeakFilterConfig) (*PeakFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewPeak2(PeakFilterConfig) (*PeakFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*PeakFilter) Reinit(PeakFilterConfig) error { return errUnsupportedPlatform }
func (*PeakFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*PeakFilter) Latency() uint32 { return 0 }
func (*PeakFilter) Close() error    { return errUnsupportedPlatform }

func (*Library) NewLowShelfFilter(LowShelfFilterConfig) (*LowShelfFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewLoShelf2(LowShelfFilterConfig) (*LowShelfFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewLowShelf2(LowShelfFilterConfig) (*LowShelfFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*LowShelfFilter) Reinit(LowShelfFilterConfig) error { return errUnsupportedPlatform }
func (*LowShelfFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*LowShelfFilter) Latency() uint32 { return 0 }
func (*LowShelfFilter) Close() error    { return errUnsupportedPlatform }

func (*Library) NewHighShelfFilter(HighShelfFilterConfig) (*HighShelfFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewHiShelf2(HighShelfFilterConfig) (*HighShelfFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewHighShelf2(HighShelfFilterConfig) (*HighShelfFilter, error) {
	return nil, errUnsupportedPlatform
}
func (*HighShelfFilter) Reinit(HighShelfFilterConfig) error { return errUnsupportedPlatform }
func (*HighShelfFilter) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint64) error {
	return errUnsupportedPlatform
}
func (*HighShelfFilter) Latency() uint32 { return 0 }
func (*HighShelfFilter) Close() error    { return errUnsupportedPlatform }

type DelayConfig struct {
	Channels      uint32
	SampleRate    uint32
	DelayInFrames uint32
	DelayStart    bool
	Wet           float32
	Dry           float32
	Decay         float32
}

func DefaultDelayConfig(channels, sampleRate, delayInFrames uint32, decay float32) DelayConfig {
	return DelayConfig{
		Channels:      channels,
		SampleRate:    sampleRate,
		DelayInFrames: delayInFrames,
		DelayStart:    decay == 0,
		Wet:           1.0,
		Dry:           1.0,
		Decay:         decay,
	}
}

type Delay struct{}

func (*Library) NewDelay(DelayConfig) (*Delay, error) {
	return nil, errUnsupportedPlatform
}
func (*Delay) ProcessPCMFrames(unsafe.Pointer, unsafe.Pointer, uint32) error {
	return errUnsupportedPlatform
}
func (*Delay) Wet() float32           { return 0 }
func (*Delay) SetWet(float32) error   { return errUnsupportedPlatform }
func (*Delay) Dry() float32           { return 0 }
func (*Delay) SetDry(float32) error   { return errUnsupportedPlatform }
func (*Delay) Decay() float32         { return 0 }
func (*Delay) SetDecay(float32) error { return errUnsupportedPlatform }
func (*Delay) Close() error           { return errUnsupportedPlatform }
