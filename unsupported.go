//go:build !darwin && !freebsd && !linux && !netbsd && !windows

package mago

import (
	"errors"
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
func (*ChannelConverter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*ChannelConverter) ProcessS16([]int16, []int16) error {
	return errUnsupportedPlatform
}
func (*ChannelConverter) InputChannelMap() (ChannelMap, error) {
	return ChannelMap{}, errUnsupportedPlatform
}
func (*ChannelConverter) OutputChannelMap() (ChannelMap, error) {
	return ChannelMap{}, errUnsupportedPlatform
}
func (*ChannelConverter) Close() error { return errUnsupportedPlatform }

func (*Library) ConvertPCMFrames([]byte, []byte, Format, Format, uint32, DitherMode) error {
	return errUnsupportedPlatform
}
func (*Library) ConvertF32ToS16([]int16, []float32, DitherMode) error {
	return errUnsupportedPlatform
}
func (*Library) ConvertS16ToF32([]float32, []int16) error {
	return errUnsupportedPlatform
}
func (*Library) ConvertFrames([]byte, Format, uint32, uint32, []byte, Format, uint32, uint32) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*Library) ConvertFramesF32ToS16([]int16, uint32, uint32, []float32, uint32, uint32) (uint64, error) {
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
func (*Resampler) Process([]float32, []float32) (uint64, uint64, error) {
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
func (*LinearResampler) Process([]float32, []float32) (uint64, uint64, error) {
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
func (*DataConverter) Process([]byte, []byte) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*DataConverter) ProcessF32([]float32, []float32) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*DataConverter) ProcessF32ToS16([]float32, []int16) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*DataConverter) ProcessS16ToF32([]int16, []float32) (uint64, uint64, error) {
	return 0, 0, errUnsupportedPlatform
}
func (*DataConverter) ProcessS16([]int16, []int16) (uint64, uint64, error) {
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

type AudioBufferConfig struct {
	Format       Format
	Channels     uint32
	SampleRate   uint32
	SizeInFrames uint64
	Data         []byte
	DataF32      []float32
}
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
func (*AudioBuffer) Read([]float32, bool) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*AudioBuffer) ReadS16([]int16, bool) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*AudioBuffer) SeekToPCMFrame(uint64) error        { return errUnsupportedPlatform }
func (*AudioBuffer) MapF32() ([]float32, error)         { return nil, errUnsupportedPlatform }
func (*AudioBuffer) MapBytes() ([]byte, error)          { return nil, errUnsupportedPlatform }
func (*AudioBuffer) Unmap(uint64) error                 { return errUnsupportedPlatform }
func (*AudioBuffer) CursorInPCMFrames() (uint64, error) { return 0, errUnsupportedPlatform }
func (*AudioBuffer) LengthInPCMFrames() (uint64, error) { return 0, errUnsupportedPlatform }
func (*AudioBuffer) AvailableFrames() (uint64, error)   { return 0, errUnsupportedPlatform }
func (*AudioBuffer) Close() error                       { return errUnsupportedPlatform }

func (*Library) NewAudioBufferRef(Format, uint32, []byte) (*AudioBufferRef, error) {
	return nil, errUnsupportedPlatform
}
func (*Library) NewAudioBufferRefF32(uint32, []float32) (*AudioBufferRef, error) {
	return nil, errUnsupportedPlatform
}
func (*AudioBufferRef) SetData([]byte) error       { return errUnsupportedPlatform }
func (*AudioBufferRef) SetDataF32([]float32) error { return errUnsupportedPlatform }
func (*AudioBufferRef) Read([]float32, bool) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*AudioBufferRef) ReadS16([]int16, bool) (uint64, error) {
	return 0, errUnsupportedPlatform
}
func (*AudioBufferRef) SeekToPCMFrame(uint64) error        { return errUnsupportedPlatform }
func (*AudioBufferRef) MapF32() ([]float32, error)         { return nil, errUnsupportedPlatform }
func (*AudioBufferRef) MapBytes() ([]byte, error)          { return nil, errUnsupportedPlatform }
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
func (*RingBuffer) AcquireRead(uint) ([]byte, error) {
	return nil, errUnsupportedPlatform
}
func (*RingBuffer) AcquireWrite(uint) ([]byte, error) {
	return nil, errUnsupportedPlatform
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
func (*PCMRingBuffer) AcquireRead(uint32) ([]float32, error) {
	return nil, errUnsupportedPlatform
}
func (*PCMRingBuffer) AcquireWrite(uint32) ([]float32, error) {
	return nil, errUnsupportedPlatform
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
func (*Waveform) Read([]float32) (uint64, error) {
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

func (*Library) NewNoise(NoiseConfig) (*Noise, error) { return nil, errUnsupportedPlatform }
func (*Noise) Read([]float32) (uint64, error)         { return 0, errUnsupportedPlatform }
func (*Noise) ReadS16([]int16) (uint64, error)        { return 0, errUnsupportedPlatform }
func (*Noise) SetAmplitude(float64) error             { return errUnsupportedPlatform }
func (*Noise) SetSeed(int32) error                    { return errUnsupportedPlatform }
func (*Noise) SetType(NoiseType) error                { return errUnsupportedPlatform }
func (*Noise) Close() error                           { return errUnsupportedPlatform }

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
func (*Biquad) Process([]float32, []float32) error       { return errUnsupportedPlatform }
func (*Biquad) ProcessS16([]int16, []int16) error        { return errUnsupportedPlatform }
func (*Biquad) Latency() uint32                          { return 0 }
func (*Biquad) Close() error                             { return errUnsupportedPlatform }

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
func (*LowPassFilter1) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*LowPassFilter1) ProcessS16([]int16, []int16) error {
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
func (*LowPassFilter2) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*LowPassFilter2) ProcessS16([]int16, []int16) error {
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
func (*LowPassFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*LowPassFilter) ProcessS16([]int16, []int16) error {
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
func (*HighPassFilter1) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*HighPassFilter1) ProcessS16([]int16, []int16) error {
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
func (*HighPassFilter2) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*HighPassFilter2) ProcessS16([]int16, []int16) error {
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
func (*HighPassFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*HighPassFilter) ProcessS16([]int16, []int16) error {
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
func (*BandPassFilter2) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*BandPassFilter2) ProcessS16([]int16, []int16) error {
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
func (*BandPassFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*BandPassFilter) ProcessS16([]int16, []int16) error {
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
func (*NotchFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*NotchFilter) ProcessS16([]int16, []int16) error {
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
func (*PeakFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*PeakFilter) ProcessS16([]int16, []int16) error {
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
func (*LowShelfFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*LowShelfFilter) ProcessS16([]int16, []int16) error {
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
func (*HighShelfFilter) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*HighShelfFilter) ProcessS16([]int16, []int16) error {
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
func (*Delay) Process([]float32, []float32) error {
	return errUnsupportedPlatform
}
func (*Delay) Wet() float32           { return 0 }
func (*Delay) SetWet(float32) error   { return errUnsupportedPlatform }
func (*Delay) Dry() float32           { return 0 }
func (*Delay) SetDry(float32) error   { return errUnsupportedPlatform }
func (*Delay) Decay() float32         { return 0 }
func (*Delay) SetDecay(float32) error { return errUnsupportedPlatform }
func (*Delay) Close() error           { return errUnsupportedPlatform }

// DataSource is the Go view of a miniaudio data source. The node graph stubs
// below need the interface even though the concrete sources are still awaiting
// their own stubs on unsupported platforms.
type DataSource interface {
	ReadPCMFrames(out []byte) (uint64, error)
	SeekToPCMFrame(frameIndex uint64) error
	DataFormat() (Format, uint32, uint32, error)
	CursorInPCMFrames() (uint64, error)
	LengthInPCMFrames() (uint64, error)
	SetLooping(looping bool) error
}

type NodeGraphConfig struct {
	Channels               uint32
	ProcessingSizeInFrames uint32
	PreMixStackSizeInBytes uint64
}
type NodeGraph struct{}

func (*Library) NewNodeGraph(NodeGraphConfig) (*NodeGraph, error) {
	return nil, errUnsupportedPlatform
}
func (*NodeGraph) Endpoint() Node                 { return nil }
func (*NodeGraph) Channels() uint32               { return 0 }
func (*NodeGraph) ProcessingSizeInFrames() uint32 { return 0 }
func (*NodeGraph) Read([]float32) (uint64, error) { return 0, errUnsupportedPlatform }
func (*NodeGraph) Time() uint64                   { return 0 }
func (*NodeGraph) SetTime(uint64) error           { return errUnsupportedPlatform }
func (*NodeGraph) Close() error                   { return errUnsupportedPlatform }

// Node is the routing view of an object in a NodeGraph.
type Node interface {
	Graph() *NodeGraph
	InputBusCount() uint32
	OutputBusCount() uint32
	InputChannels(inputBusIndex uint32) uint32
	OutputChannels(outputBusIndex uint32) uint32
	AttachOutputBus(outputBusIndex uint32, other Node, otherInputBusIndex uint32) error
	DetachOutputBus(outputBusIndex uint32) error
	DetachAllOutputBuses() error
	SetOutputBusVolume(outputBusIndex uint32, volume float32) error
	OutputBusVolume(outputBusIndex uint32) float32
	State() NodeState
	SetState(state NodeState) error
	SetStateTime(state NodeState, globalTime uint64) error
	StateTime(state NodeState) uint64
	StateByTime(globalTime uint64) NodeState
	StateByTimeRange(globalTimeBeg, globalTimeEnd uint64) NodeState
	Time() uint64
	SetTime(localTime uint64) error

	nodeHandle() *nodeHandle
}

// nodeStub implements the shared Node surface for every node type on
// unsupported platforms.
type nodeStub struct{}

func (nodeStub) Graph() *NodeGraph { return nil }
func (nodeStub) InputBusCount() uint32 {
	return 0
}
func (nodeStub) OutputBusCount() uint32 { return 0 }
func (nodeStub) InputChannels(uint32) uint32 {
	return 0
}
func (nodeStub) OutputChannels(uint32) uint32 { return 0 }
func (nodeStub) AttachOutputBus(uint32, Node, uint32) error {
	return errUnsupportedPlatform
}
func (nodeStub) DetachOutputBus(uint32) error { return errUnsupportedPlatform }
func (nodeStub) DetachAllOutputBuses() error  { return errUnsupportedPlatform }
func (nodeStub) SetOutputBusVolume(uint32, float32) error {
	return errUnsupportedPlatform
}
func (nodeStub) OutputBusVolume(uint32) float32 { return 0 }
func (nodeStub) State() NodeState               { return NodeStateStopped }
func (nodeStub) SetState(NodeState) error       { return errUnsupportedPlatform }
func (nodeStub) SetStateTime(NodeState, uint64) error {
	return errUnsupportedPlatform
}
func (nodeStub) StateTime(NodeState) uint64                { return 0 }
func (nodeStub) StateByTime(uint64) NodeState              { return NodeStateStopped }
func (nodeStub) StateByTimeRange(uint64, uint64) NodeState { return NodeStateStopped }
func (nodeStub) Time() uint64                              { return 0 }
func (nodeStub) SetTime(uint64) error                      { return errUnsupportedPlatform }
func (nodeStub) nodeHandle() *nodeHandle                   { return nil }

type SplitterNodeConfig struct {
	Channels       uint32
	OutputBusCount uint32
}

func DefaultSplitterNodeConfig(channels uint32) SplitterNodeConfig {
	return SplitterNodeConfig{Channels: channels, OutputBusCount: 2}
}

type BiquadNodeConfig struct {
	Channels uint32
	B0       float64
	B1       float64
	B2       float64
	A0       float64
	A1       float64
	A2       float64
}
type FilterNodeConfig struct {
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}
type NotchNodeConfig struct {
	Channels   uint32
	SampleRate uint32
	Q          float64
	Frequency  float64
}
type PeakNodeConfig struct {
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	Q          float64
	Frequency  float64
}
type ShelfNodeConfig struct {
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

type DataSourceNode struct{ nodeStub }

func (*NodeGraph) NewDataSourceNode(DataSource) (*DataSourceNode, error) {
	return nil, errUnsupportedPlatform
}
func (*DataSourceNode) Source() DataSource    { return nil }
func (*DataSourceNode) SetLooping(bool) error { return errUnsupportedPlatform }
func (*DataSourceNode) IsLooping() bool       { return false }
func (*DataSourceNode) Close() error          { return errUnsupportedPlatform }

type SplitterNode struct{ nodeStub }

func (*NodeGraph) NewSplitterNode(SplitterNodeConfig) (*SplitterNode, error) {
	return nil, errUnsupportedPlatform
}
func (*SplitterNode) Close() error { return errUnsupportedPlatform }

type BiquadNode struct{ nodeStub }

func (*NodeGraph) NewBiquadNode(BiquadNodeConfig) (*BiquadNode, error) {
	return nil, errUnsupportedPlatform
}
func (*BiquadNode) Reinit(BiquadNodeConfig) error { return errUnsupportedPlatform }
func (*BiquadNode) Close() error                  { return errUnsupportedPlatform }

type LowPassNode struct{ nodeStub }
type HighPassNode struct{ nodeStub }
type BandPassNode struct{ nodeStub }

func (*NodeGraph) NewLowPassNode(FilterNodeConfig) (*LowPassNode, error) {
	return nil, errUnsupportedPlatform
}
func (*NodeGraph) NewHighPassNode(FilterNodeConfig) (*HighPassNode, error) {
	return nil, errUnsupportedPlatform
}
func (*NodeGraph) NewBandPassNode(FilterNodeConfig) (*BandPassNode, error) {
	return nil, errUnsupportedPlatform
}
func (*LowPassNode) Reinit(FilterNodeConfig) error  { return errUnsupportedPlatform }
func (*LowPassNode) Close() error                   { return errUnsupportedPlatform }
func (*HighPassNode) Reinit(FilterNodeConfig) error { return errUnsupportedPlatform }
func (*HighPassNode) Close() error                  { return errUnsupportedPlatform }
func (*BandPassNode) Reinit(FilterNodeConfig) error { return errUnsupportedPlatform }
func (*BandPassNode) Close() error                  { return errUnsupportedPlatform }

type NotchNode struct{ nodeStub }
type PeakNode struct{ nodeStub }
type LowShelfNode struct{ nodeStub }
type HighShelfNode struct{ nodeStub }

func (*NodeGraph) NewNotchNode(NotchNodeConfig) (*NotchNode, error) {
	return nil, errUnsupportedPlatform
}
func (*NodeGraph) NewPeakNode(PeakNodeConfig) (*PeakNode, error) {
	return nil, errUnsupportedPlatform
}
func (*NodeGraph) NewLowShelfNode(ShelfNodeConfig) (*LowShelfNode, error) {
	return nil, errUnsupportedPlatform
}
func (*NodeGraph) NewHighShelfNode(ShelfNodeConfig) (*HighShelfNode, error) {
	return nil, errUnsupportedPlatform
}
func (*NotchNode) Reinit(NotchNodeConfig) error     { return errUnsupportedPlatform }
func (*NotchNode) Close() error                     { return errUnsupportedPlatform }
func (*PeakNode) Reinit(PeakNodeConfig) error       { return errUnsupportedPlatform }
func (*PeakNode) Close() error                      { return errUnsupportedPlatform }
func (*LowShelfNode) Reinit(ShelfNodeConfig) error  { return errUnsupportedPlatform }
func (*LowShelfNode) Close() error                  { return errUnsupportedPlatform }
func (*HighShelfNode) Reinit(ShelfNodeConfig) error { return errUnsupportedPlatform }
func (*HighShelfNode) Close() error                 { return errUnsupportedPlatform }

type DelayNode struct{ nodeStub }

func (*NodeGraph) NewDelayNode(DelayConfig) (*DelayNode, error) {
	return nil, errUnsupportedPlatform
}
func (*DelayNode) Wet() float32           { return 0 }
func (*DelayNode) SetWet(float32) error   { return errUnsupportedPlatform }
func (*DelayNode) Dry() float32           { return 0 }
func (*DelayNode) SetDry(float32) error   { return errUnsupportedPlatform }
func (*DelayNode) Decay() float32         { return 0 }
func (*DelayNode) SetDecay(float32) error { return errUnsupportedPlatform }
func (*DelayNode) Close() error           { return errUnsupportedPlatform }
