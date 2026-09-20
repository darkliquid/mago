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

// NodeState mirrors ma_node_state: the playback state of a graph node.
type NodeState int32

const (
	NodeStateStarted NodeState = 0
	NodeStateStopped NodeState = 1
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

var (
	ErrInvalidSliceLength = fmt.Errorf("mago: slice length does not align with channel count")
	ErrOutputTooSmall     = fmt.Errorf("mago: output slice too small for input frames")
)

func validateFilterSlices[T float32 | int16](channels uint32, out, in []T) (uint64, error) {
	if len(in) == 0 {
		return 0, nil
	}
	if channels == 0 || len(in)%int(channels) != 0 {
		return 0, ErrInvalidSliceLength
	}
	if len(out) < len(in) {
		return 0, ErrOutputTooSmall
	}
	return uint64(len(in) / int(channels)), nil
}

// DeviceID is a type-safe opaque identifier for a hardware audio endpoint.
// It mirrors the ma_device_id union, whose largest member is a 256-byte buffer.
// Its size is validated by layout_test.go against the vendored header.
type DeviceID [256]byte

// IsZero reports whether the device ID is uninitialized / default.
func (id DeviceID) IsZero() bool {
	return id == DeviceID{}
}

// String returns a readable representation of the device identifier.
func (id DeviceID) String() string {
	for i, b := range id {
		if b == 0 {
			return string(id[:i])
		}
	}
	return string(id[:])
}

type deviceIDNative = DeviceID

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
	ID        DeviceID
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
	magoObjectResampler        int32 = 7
	magoObjectLinearResampler  int32 = 8
	magoObjectDataConverter    int32 = 9
	magoObjectAudioBuffer      int32 = 10
	magoObjectAudioBufferRef   int32 = 11
	magoObjectRingBuffer       int32 = 12
	magoObjectPCMRingBuffer    int32 = 13
	magoObjectWaveform         int32 = 14
	magoObjectNoise            int32 = 15
	magoObjectBiquad           int32 = 16
	magoObjectLPF1             int32 = 17
	magoObjectLPF2             int32 = 18
	magoObjectLPF              int32 = 19
	magoObjectHPF1             int32 = 20
	magoObjectHPF2             int32 = 21
	magoObjectHPF              int32 = 22
	magoObjectBPF2             int32 = 23
	magoObjectBPF              int32 = 24
	magoObjectNotch2           int32 = 25
	magoObjectPeak2            int32 = 26
	magoObjectLoShelf2         int32 = 27
	magoObjectHiShelf2         int32 = 28
	magoObjectDelay            int32 = 29
	magoObjectDecoder          int32 = 30
	magoObjectEncoder          int32 = 31
	magoObjectNodeGraph        int32 = 32
	magoObjectDataSourceNode   int32 = 33
	magoObjectSplitterNode     int32 = 34
	magoObjectBiquadNode       int32 = 35
	magoObjectLPFNode          int32 = 36
	magoObjectHPFNode          int32 = 37
	magoObjectBPFNode          int32 = 38
	magoObjectNotchNode        int32 = 39
	magoObjectPeakNode         int32 = 40
	magoObjectLoShelfNode      int32 = 41
	magoObjectHiShelfNode      int32 = 42
	magoObjectDelayNode        int32 = 43
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

// ResampleAlgorithm mirrors ma_resample_algorithm.
type ResampleAlgorithm int32

const (
	ResampleAlgorithmLinear ResampleAlgorithm = 0
	ResampleAlgorithmCustom ResampleAlgorithm = 1
)

// resamplerLinearConfigNative mirrors the anonymous linear sub-struct of
// ma_resampler_config.
type resamplerLinearConfigNative struct {
	LPFOrder uint32
}

// resamplerConfigNative mirrors ma_resampler_config. Validated by layout_test.go.
type resamplerConfigNative struct {
	Format          Format
	Channels        uint32
	SampleRateIn    uint32
	SampleRateOut   uint32
	Algorithm       ResampleAlgorithm
	BackendVTable   unsafe.Pointer
	BackendUserData unsafe.Pointer
	Linear          resamplerLinearConfigNative
}

// linearResamplerConfigNative mirrors ma_linear_resampler_config. Validated by
// layout_test.go.
type linearResamplerConfigNative struct {
	Format           Format
	Channels         uint32
	SampleRateIn     uint32
	SampleRateOut    uint32
	LPFOrder         uint32
	LPFNyquistFactor float64
}

// dataConverterConfigNative mirrors ma_data_converter_config. Validated by
// layout_test.go.
type dataConverterConfigNative struct {
	FormatIn                        Format
	FormatOut                       Format
	ChannelsIn                      uint32
	ChannelsOut                     uint32
	SampleRateIn                    uint32
	SampleRateOut                   uint32
	ChannelMapIn                    *uint8 // *ma_channel
	ChannelMapOut                   *uint8 // *ma_channel
	DitherMode                      DitherMode
	ChannelMixMode                  ChannelMixMode
	CalculateLFEFromSpatialChannels uint32
	ChannelWeights                  **float32 // ppChannelWeights, only custom weights
	AllowDynamicSampleRate          uint32
	Resampling                      resamplerConfigNative
}

type resamplerHandle struct{}
type linearResamplerHandle struct{}
type dataConverterHandle struct{}
type audioBufferHandle struct{}
type audioBufferRefHandle struct{}
type ringBufferHandle struct{}
type pcmRingBufferHandle struct{}

// allocationCallbacksNative mirrors ma_allocation_callbacks. mago always passes
// zeroed callbacks so miniaudio uses its default allocator.
type allocationCallbacksNative struct {
	UserData  unsafe.Pointer
	OnMalloc  unsafe.Pointer
	OnRealloc unsafe.Pointer
	OnFree    unsafe.Pointer
}

// audioBufferConfigNative mirrors ma_audio_buffer_config. Validated by
// layout_test.go.
type audioBufferConfigNative struct {
	Format              Format
	Channels            uint32
	SampleRate          uint32
	SizeInFrames        uint64
	Data                unsafe.Pointer
	AllocationCallbacks allocationCallbacksNative
}

// WaveformType identifies the periodic shape of a waveform.
type WaveformType int32

// NoiseType identifies the spectral distribution of generated noise.
type NoiseType int32

// waveformConfigNative mirrors ma_waveform_config. Validated by layout_test.go.
type waveformConfigNative struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Type       WaveformType
	Amplitude  float64
	Frequency  float64
}

// noiseConfigNative mirrors ma_noise_config. Validated by layout_test.go.
type noiseConfigNative struct {
	Format            Format
	Channels          uint32
	Type              NoiseType
	Seed              int32
	Amplitude         float64
	DuplicateChannels uint32
}

type waveformHandle struct{}
type noiseHandle struct{}
type biquadHandle struct{}
type lpf1Handle struct{}
type lpf2Handle struct{}
type lpfHandle struct{}
type hpf1Handle struct{}
type hpf2Handle struct{}
type hpfHandle struct{}
type bpf2Handle struct{}
type bpfHandle struct{}
type notch2Handle struct{}
type peak2Handle struct{}
type loshelf2Handle struct{}
type hishelf2Handle struct{}
type delayHandle struct{}
type decoderHandle struct{}
type encoderHandle struct{}
type encoderBridgeNative struct{}
type dataSourceHandle struct{}

// encoderConfigNative mirrors ma_encoder_config. Validated by layout_test.go.
type encoderConfigNative struct {
	EncodingFormat      EncodingFormat
	Format              Format
	Channels            uint32
	SampleRate          uint32
	AllocationCallbacks allocationCallbacksNative
}

// EncodingFormat mirrors ma_encoding_format.
type EncodingFormat int32

const (
	EncodingFormatUnknown EncodingFormat = 0
	EncodingFormatWAV     EncodingFormat = 1
	EncodingFormatFLAC    EncodingFormat = 2
	EncodingFormatMP3     EncodingFormat = 3
	EncodingFormatVorbis  EncodingFormat = 4
)

// decoderConfigNative mirrors ma_decoder_config. Validated by layout_test.go.
type decoderConfigNative struct {
	Format                Format
	Channels              uint32
	SampleRate            uint32
	ChannelMap            *uint8 // *ma_channel
	ChannelMixMode        ChannelMixMode
	DitherMode            DitherMode
	Resampling            resamplerConfigNative
	AllocationCallbacks   allocationCallbacksNative
	EncodingFormat        EncodingFormat
	SeekPointCount        uint32
	CustomBackendVTables  unsafe.Pointer // **ma_decoding_backend_vtable, always nil
	CustomBackendCount    uint32
	CustomBackendUserData unsafe.Pointer
}

// biquadConfigNative mirrors ma_biquad_config. Validated by layout_test.go.
type biquadConfigNative struct {
	Format   Format
	Channels uint32
	B0       float64
	B1       float64
	B2       float64
	A0       float64
	A1       float64
	A2       float64
}

// lpf1ConfigNative mirrors ma_lpf1_config and ma_lpf2_config. Validated by layout_test.go.
type lpf1ConfigNative struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Q               float64
}

type lpf2ConfigNative = lpf1ConfigNative

// lpfConfigNative mirrors ma_lpf_config. Validated by layout_test.go.
type lpfConfigNative struct {
	Format          Format
	Channels        uint32
	SampleRate      uint32
	CutoffFrequency float64
	Order           uint32
}

// hpf1ConfigNative mirrors ma_hpf1_config. Validated by layout_test.go.
type hpf1ConfigNative = lpf1ConfigNative

// hpf2ConfigNative mirrors ma_hpf2_config. Validated by layout_test.go.
type hpf2ConfigNative = lpf1ConfigNative

// hpfConfigNative mirrors ma_hpf_config. Validated by layout_test.go.
type hpfConfigNative = lpfConfigNative

// bpf2ConfigNative mirrors ma_bpf2_config. Validated by layout_test.go.
type bpf2ConfigNative = lpf1ConfigNative

// bpfConfigNative mirrors ma_bpf_config. Validated by layout_test.go.
type bpfConfigNative = lpfConfigNative

// notch2ConfigNative mirrors ma_notch2_config. Validated by layout_test.go.
type notch2ConfigNative struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	Q          float64
	Frequency  float64
}

// peak2ConfigNative mirrors ma_peak2_config. Validated by layout_test.go.
type peak2ConfigNative struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	Q          float64
	Frequency  float64
}

// loshelf2ConfigNative mirrors ma_loshelf2_config and ma_hishelf2_config. Validated by layout_test.go.
type loshelf2ConfigNative struct {
	Format     Format
	Channels   uint32
	SampleRate uint32
	GainDB     float64
	ShelfSlope float64
	Frequency  float64
}

type hishelf2ConfigNative = loshelf2ConfigNative

// delayConfigNative mirrors ma_delay_config. Validated by layout_test.go.
type delayConfigNative struct {
	Channels      uint32
	SampleRate    uint32
	DelayInFrames uint32
	DelayStart    uint32
	Wet           float32
	Dry           float32
	Decay         float32
}

// nodeGraphHandle is the mirror of ma_node_graph. It is opaque to Go; the size
// comes from mago_alloc.
type nodeGraphHandle struct{}

// nodeHandle is the mirror of ma_node. Every concrete node type begins with
// ma_node_base, so a node's own pointer is also a valid *nodeHandle.
type nodeHandle struct{}

// nodeConfigNative mirrors ma_node_config. Validated by layout_test.go.
//
// Go cannot call ma_node_config_init because it returns by value, but the
// function's result is fully determined: a NULL vtable, the started state, and
// MA_NODE_BUS_COUNT_UNKNOWN for both bus counts. The vtable is left NULL because
// every ma_*_node_init overwrites it.
type nodeConfigNative struct {
	VTable         unsafe.Pointer
	InitialState   NodeState
	InputBusCount  uint32
	OutputBusCount uint32
	InputChannels  *uint32
	OutputChannels *uint32
}

// nodeGraphConfigNative mirrors ma_node_graph_config. Validated by
// layout_test.go.
type nodeGraphConfigNative struct {
	Channels               uint32
	ProcessingSizeInFrames uint32
	PreMixStackSizeInBytes uintptr
}

// dataSourceNodeConfigNative mirrors ma_data_source_node_config. Validated by
// layout_test.go.
type dataSourceNodeConfigNative struct {
	NodeConfig nodeConfigNative
	DataSource *dataSourceHandle
}

// splitterNodeConfigNative mirrors ma_splitter_node_config. Validated by
// layout_test.go.
type splitterNodeConfigNative struct {
	NodeConfig     nodeConfigNative
	Channels       uint32
	OutputBusCount uint32
}

// biquadNodeConfigNative mirrors ma_biquad_node_config. Validated by
// layout_test.go.
type biquadNodeConfigNative struct {
	NodeConfig nodeConfigNative
	Biquad     biquadConfigNative
}

// lpfNodeConfigNative mirrors ma_lpf_node_config. ma_hpf_node_config and
// ma_bpf_node_config have identical members. Validated by layout_test.go.
type lpfNodeConfigNative struct {
	NodeConfig nodeConfigNative
	LPF        lpfConfigNative
}

type hpfNodeConfigNative = lpfNodeConfigNative
type bpfNodeConfigNative = lpfNodeConfigNative

// notchNodeConfigNative mirrors ma_notch_node_config (whose filter member is
// the ma_notch2_config layout). Validated by layout_test.go.
type notchNodeConfigNative struct {
	NodeConfig nodeConfigNative
	Notch      notch2ConfigNative
}

// peakNodeConfigNative mirrors ma_peak_node_config (whose filter member is the
// ma_peak2_config layout). Validated by layout_test.go.
type peakNodeConfigNative struct {
	NodeConfig nodeConfigNative
	Peak       peak2ConfigNative
}

// loshelfNodeConfigNative mirrors ma_loshelf_node_config. ma_hishelf_node_config
// has identical members. Validated by layout_test.go.
type loshelfNodeConfigNative struct {
	NodeConfig nodeConfigNative
	LoShelf    loshelf2ConfigNative
}

type hishelfNodeConfigNative = loshelfNodeConfigNative

// delayNodeConfigNative mirrors ma_delay_node_config. Validated by
// layout_test.go.
type delayNodeConfigNative struct {
	NodeConfig nodeConfigNative
	Delay      delayConfigNative
}
