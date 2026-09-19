//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"unsafe"
)

// ResamplerConfig configures a general resampler.
type ResamplerConfig struct {
	Format         Format
	Channels       uint32
	SampleRateIn   uint32
	SampleRateOut  uint32
	Algorithm      ResampleAlgorithm
	LinearLPFOrder uint32
}

// DefaultResamplerConfig returns miniaudio's defaults for a linear resampler:
// the linear algorithm with a low-pass filter order of 4.
func DefaultResamplerConfig(format Format, channels, sampleRateIn, sampleRateOut uint32) ResamplerConfig {
	return ResamplerConfig{
		Format:         format,
		Channels:       channels,
		SampleRateIn:   sampleRateIn,
		SampleRateOut:  sampleRateOut,
		Algorithm:      ResampleAlgorithmLinear,
		LinearLPFOrder: 4,
	}
}

// Resampler converts between sample rates for a fixed format and channel count.
type Resampler struct {
	lib      *Library
	handle   *resamplerHandle
	channels uint32
}

// NewResampler creates a resampler. Custom resampling backends are rejected.
func (lib *Library) NewResampler(config ResamplerConfig) (*Resampler, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if config.Algorithm == ResampleAlgorithmCustom {
		return nil, fmt.Errorf("mago: custom resampling backends are not supported")
	}

	native := resamplerConfigNative{
		Format:        config.Format,
		Channels:      config.Channels,
		SampleRateIn:  config.SampleRateIn,
		SampleRateOut: config.SampleRateOut,
		Algorithm:     config.Algorithm,
		Linear:        resamplerLinearConfigNative{LPFOrder: config.LinearLPFOrder},
	}

	handle := (*resamplerHandle)(lib.bindings.magoAlloc(magoObjectResampler))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate resampler: out of memory")
	}
	if result := lib.bindings.maResamplerInit(&native, nil, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_resampler_init", result)
	}

	return &Resampler{lib: lib, handle: handle, channels: config.Channels}, nil
}

// Process resamples input frames from in into out.
// It returns the number of input frames consumed and output frames produced.
func (r *Resampler) Process(in, out []float32) (framesInRead, framesOutWritten uint64, err error) {
	if r == nil || r.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	if r.channels == 0 || len(in)%int(r.channels) != 0 || len(out)%int(r.channels) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(r.channels))
	framesOut := uint64(len(out) / int(r.channels))
	inCount, outCount := framesIn, framesOut
	result := r.lib.bindings.maResamplerProcessPCMFrames(r.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, r.lib.resultError("ma_resampler_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// SetRate changes the input and output sample rates.
func (r *Resampler) SetRate(sampleRateIn, sampleRateOut uint32) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_resampler_set_rate", r.lib.bindings.maResamplerSetRate(r.handle, sampleRateIn, sampleRateOut))
}

// SetRateRatio changes the output-to-input rate ratio.
func (r *Resampler) SetRateRatio(ratio float64) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_resampler_set_rate_ratio", r.lib.bindings.maResamplerSetRateRatio(r.handle, float32(ratio)))
}

// Reset clears the resampler's internal state.
func (r *Resampler) Reset() error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_resampler_reset", r.lib.bindings.maResamplerReset(r.handle))
}

// RequiredInputFrameCount reports the input frames needed to produce
// outputFrameCount output frames.
func (r *Resampler) RequiredInputFrameCount(outputFrameCount uint64) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var count uint64
	if result := r.lib.bindings.maResamplerGetRequiredInputFrameCount(r.handle, outputFrameCount, &count); result != Success {
		return 0, r.lib.resultError("ma_resampler_get_required_input_frame_count", result)
	}
	return count, nil
}

// ExpectedOutputFrameCount reports the output frames produced from
// inputFrameCount input frames.
func (r *Resampler) ExpectedOutputFrameCount(inputFrameCount uint64) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var count uint64
	if result := r.lib.bindings.maResamplerGetExpectedOutputFrameCount(r.handle, inputFrameCount, &count); result != Success {
		return 0, r.lib.resultError("ma_resampler_get_expected_output_frame_count", result)
	}
	return count, nil
}

// Close uninitializes the resampler and frees it.
func (r *Resampler) Close() error {
	if r == nil || r.handle == nil {
		return nil
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}

	r.lib.bindings.maResamplerUninit(r.handle, nil)
	r.lib.bindings.magoFree(unsafe.Pointer(r.handle))
	r.handle = nil
	return nil
}

// LinearResamplerConfig configures a linear resampler.
type LinearResamplerConfig struct {
	Format           Format
	Channels         uint32
	SampleRateIn     uint32
	SampleRateOut    uint32
	LPFOrder         uint32
	LPFNyquistFactor float64
}

// DefaultLinearResamplerConfig returns miniaudio's defaults: a low-pass filter
// order of 4 and a Nyquist factor of 1.
func DefaultLinearResamplerConfig(format Format, channels, sampleRateIn, sampleRateOut uint32) LinearResamplerConfig {
	return LinearResamplerConfig{
		Format:           format,
		Channels:         channels,
		SampleRateIn:     sampleRateIn,
		SampleRateOut:    sampleRateOut,
		LPFOrder:         4,
		LPFNyquistFactor: 1,
	}
}

// LinearResampler is a fixed-rate linear resampler.
type LinearResampler struct {
	lib      *Library
	handle   *linearResamplerHandle
	channels uint32
}

// NewLinearResampler creates a linear resampler.
func (lib *Library) NewLinearResampler(config LinearResamplerConfig) (*LinearResampler, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	native := linearResamplerConfigNative(config)

	handle := (*linearResamplerHandle)(lib.bindings.magoAlloc(magoObjectLinearResampler))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate linear resampler: out of memory")
	}
	if result := lib.bindings.maLinearResamplerInit(&native, nil, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_linear_resampler_init", result)
	}

	return &LinearResampler{lib: lib, handle: handle, channels: config.Channels}, nil
}

// Process resamples input frames from in into out.
// It returns the number of input frames consumed and output frames produced.
func (r *LinearResampler) Process(in, out []float32) (framesInRead, framesOutWritten uint64, err error) {
	if r == nil || r.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil linear resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	if r.channels == 0 || len(in)%int(r.channels) != 0 || len(out)%int(r.channels) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(r.channels))
	framesOut := uint64(len(out) / int(r.channels))
	inCount, outCount := framesIn, framesOut
	result := r.lib.bindings.maLinearResamplerProcessPCMFrames(r.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, r.lib.resultError("ma_linear_resampler_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// SetRate changes the input and output sample rates.
func (r *LinearResampler) SetRate(sampleRateIn, sampleRateOut uint32) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil linear resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_linear_resampler_set_rate", r.lib.bindings.maLinearResamplerSetRate(r.handle, sampleRateIn, sampleRateOut))
}

// SetRateRatio changes the output-to-input rate ratio.
func (r *LinearResampler) SetRateRatio(ratio float64) error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil linear resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_linear_resampler_set_rate_ratio", r.lib.bindings.maLinearResamplerSetRateRatio(r.handle, float32(ratio)))
}

// Reset clears the resampler's internal state.
func (r *LinearResampler) Reset() error {
	if r == nil || r.handle == nil {
		return fmt.Errorf("mago: nil linear resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}
	return r.lib.resultError("ma_linear_resampler_reset", r.lib.bindings.maLinearResamplerReset(r.handle))
}

// RequiredInputFrameCount reports the input frames needed to produce
// outputFrameCount output frames.
func (r *LinearResampler) RequiredInputFrameCount(outputFrameCount uint64) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil linear resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var count uint64
	if result := r.lib.bindings.maLinearResamplerGetRequiredInputFrameCount(r.handle, outputFrameCount, &count); result != Success {
		return 0, r.lib.resultError("ma_linear_resampler_get_required_input_frame_count", result)
	}
	return count, nil
}

// ExpectedOutputFrameCount reports the output frames produced from
// inputFrameCount input frames.
func (r *LinearResampler) ExpectedOutputFrameCount(inputFrameCount uint64) (uint64, error) {
	if r == nil || r.handle == nil {
		return 0, fmt.Errorf("mago: nil linear resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var count uint64
	if result := r.lib.bindings.maLinearResamplerGetExpectedOutputFrameCount(r.handle, inputFrameCount, &count); result != Success {
		return 0, r.lib.resultError("ma_linear_resampler_get_expected_output_frame_count", result)
	}
	return count, nil
}

// Close uninitializes the resampler and frees it.
func (r *LinearResampler) Close() error {
	if r == nil || r.handle == nil {
		return nil
	}
	if err := r.lib.ensureOpen(); err != nil {
		return err
	}

	r.lib.bindings.maLinearResamplerUninit(r.handle, nil)
	r.lib.bindings.magoFree(unsafe.Pointer(r.handle))
	r.handle = nil
	return nil
}

// DataConverterConfig configures a full format, channel and sample-rate
// conversion pipeline.
type DataConverterConfig struct {
	FormatIn                        Format
	FormatOut                       Format
	ChannelsIn                      uint32
	ChannelsOut                     uint32
	SampleRateIn                    uint32
	SampleRateOut                   uint32
	ChannelMapIn                    ChannelMap
	ChannelMapOut                   ChannelMap
	DitherMode                      DitherMode
	ChannelMixMode                  ChannelMixMode
	CalculateLFEFromSpatialChannels bool
	AllowDynamicSampleRate          bool
	LinearLPFOrder                  uint32
}

// DefaultDataConverterConfig returns miniaudio's defaults, which use no dither,
// rectangular channel mixing and a resampler low-pass filter order of 1.
func DefaultDataConverterConfig(formatIn, formatOut Format, channelsIn, channelsOut, sampleRateIn, sampleRateOut uint32) DataConverterConfig {
	return DataConverterConfig{
		FormatIn:       formatIn,
		FormatOut:      formatOut,
		ChannelsIn:     channelsIn,
		ChannelsOut:    channelsOut,
		SampleRateIn:   sampleRateIn,
		SampleRateOut:  sampleRateOut,
		DitherMode:     DitherModeNone,
		ChannelMixMode: ChannelMixModeRectangular,
		LinearLPFOrder: 1,
	}
}

// DataConverter performs format, channel and sample-rate conversion in one pass.
type DataConverter struct {
	lib         *Library
	handle      *dataConverterHandle
	channelsIn  uint32
	channelsOut uint32
	formatIn    Format
	formatOut   Format
}

// NewDataConverter creates a data converter. Custom channel mixing weights are
// rejected.
func (lib *Library) NewDataConverter(config DataConverterConfig) (*DataConverter, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	if config.ChannelMixMode == ChannelMixModeCustomWeights {
		return nil, fmt.Errorf("mago: custom channel mixing weights are not supported")
	}

	native := dataConverterConfigNative{
		FormatIn:                        config.FormatIn,
		FormatOut:                       config.FormatOut,
		ChannelsIn:                      config.ChannelsIn,
		ChannelsOut:                     config.ChannelsOut,
		SampleRateIn:                    config.SampleRateIn,
		SampleRateOut:                   config.SampleRateOut,
		ChannelMapIn:                    channelMapDataPtr(config.ChannelMapIn),
		ChannelMapOut:                   channelMapDataPtr(config.ChannelMapOut),
		DitherMode:                      config.DitherMode,
		ChannelMixMode:                  config.ChannelMixMode,
		CalculateLFEFromSpatialChannels: boolToBool32(config.CalculateLFEFromSpatialChannels),
		AllowDynamicSampleRate:          boolToBool32(config.AllowDynamicSampleRate),
		Resampling: resamplerConfigNative{
			Algorithm: ResampleAlgorithmLinear,
			Linear:    resamplerLinearConfigNative{LPFOrder: config.LinearLPFOrder},
		},
	}

	handle := (*dataConverterHandle)(lib.bindings.magoAlloc(magoObjectDataConverter))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate data converter: out of memory")
	}
	if result := lib.bindings.maDataConverterInit(&native, nil, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_data_converter_init", result)
	}

	return &DataConverter{
		lib:         lib,
		handle:      handle,
		channelsIn:  config.ChannelsIn,
		channelsOut: config.ChannelsOut,
		formatIn:    config.FormatIn,
		formatOut:   config.FormatOut,
	}, nil
}

// Process converts up to the available frames from in into out.
// It returns how many frames were consumed and produced.
func (c *DataConverter) Process(in, out []byte) (framesInRead, framesOutWritten uint64, err error) {
	if c == nil || c.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	bpfIn := BytesPerSample(c.formatIn) * c.channelsIn
	bpfOut := BytesPerSample(c.formatOut) * c.channelsOut
	if bpfIn == 0 || len(in)%int(bpfIn) != 0 || bpfOut == 0 || len(out)%int(bpfOut) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(bpfIn))
	framesOut := uint64(len(out) / int(bpfOut))
	inCount, outCount := framesIn, framesOut
	result := c.lib.bindings.maDataConverterProcessPCMFrames(c.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, c.lib.resultError("ma_data_converter_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// ProcessF32 converts up to the available float32 frames from in into out.
// It returns how many frames were consumed and produced.
func (c *DataConverter) ProcessF32(in, out []float32) (framesInRead, framesOutWritten uint64, err error) {
	if c == nil || c.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	if c.channelsIn == 0 || len(in)%int(c.channelsIn) != 0 || c.channelsOut == 0 || len(out)%int(c.channelsOut) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(c.channelsIn))
	framesOut := uint64(len(out) / int(c.channelsOut))
	inCount, outCount := framesIn, framesOut
	result := c.lib.bindings.maDataConverterProcessPCMFrames(c.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, c.lib.resultError("ma_data_converter_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// ProcessF32ToS16 converts up to the available frames from float32 in into int16 out.
// It returns how many frames were consumed and produced.
func (c *DataConverter) ProcessF32ToS16(in []float32, out []int16) (framesInRead, framesOutWritten uint64, err error) {
	if c == nil || c.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	if c.channelsIn == 0 || len(in)%int(c.channelsIn) != 0 || c.channelsOut == 0 || len(out)%int(c.channelsOut) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(c.channelsIn))
	framesOut := uint64(len(out) / int(c.channelsOut))
	inCount, outCount := framesIn, framesOut
	result := c.lib.bindings.maDataConverterProcessPCMFrames(c.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, c.lib.resultError("ma_data_converter_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// ProcessS16ToF32 converts up to the available frames from int16 in into float32 out.
// It returns how many frames were consumed and produced.
func (c *DataConverter) ProcessS16ToF32(in []int16, out []float32) (framesInRead, framesOutWritten uint64, err error) {
	if c == nil || c.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	if c.channelsIn == 0 || len(in)%int(c.channelsIn) != 0 || c.channelsOut == 0 || len(out)%int(c.channelsOut) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(c.channelsIn))
	framesOut := uint64(len(out) / int(c.channelsOut))
	inCount, outCount := framesIn, framesOut
	result := c.lib.bindings.maDataConverterProcessPCMFrames(c.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, c.lib.resultError("ma_data_converter_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// ProcessS16 converts up to the available int16 frames from in into out.
// It returns how many frames were consumed and produced.
func (c *DataConverter) ProcessS16(in, out []int16) (framesInRead, framesOutWritten uint64, err error) {
	if c == nil || c.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}
	if len(in) == 0 || len(out) == 0 {
		return 0, 0, nil
	}
	if c.channelsIn == 0 || len(in)%int(c.channelsIn) != 0 || c.channelsOut == 0 || len(out)%int(c.channelsOut) != 0 {
		return 0, 0, ErrInvalidSliceLength
	}
	framesIn := uint64(len(in) / int(c.channelsIn))
	framesOut := uint64(len(out) / int(c.channelsOut))
	inCount, outCount := framesIn, framesOut
	result := c.lib.bindings.maDataConverterProcessPCMFrames(c.handle, unsafe.Pointer(&in[0]), &inCount, unsafe.Pointer(&out[0]), &outCount)
	if result != Success {
		return inCount, outCount, c.lib.resultError("ma_data_converter_process_pcm_frames", result)
	}
	return inCount, outCount, nil
}

// SetRate changes the input and output sample rates.
func (c *DataConverter) SetRate(sampleRateIn, sampleRateOut uint32) error {
	if c == nil || c.handle == nil {
		return fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}
	return c.lib.resultError("ma_data_converter_set_rate", c.lib.bindings.maDataConverterSetRate(c.handle, sampleRateIn, sampleRateOut))
}

// SetRateRatio changes the output-to-input rate ratio.
func (c *DataConverter) SetRateRatio(ratio float64) error {
	if c == nil || c.handle == nil {
		return fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}
	return c.lib.resultError("ma_data_converter_set_rate_ratio", c.lib.bindings.maDataConverterSetRateRatio(c.handle, float32(ratio)))
}

// Reset clears the converter's internal state.
func (c *DataConverter) Reset() error {
	if c == nil || c.handle == nil {
		return fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}
	return c.lib.resultError("ma_data_converter_reset", c.lib.bindings.maDataConverterReset(c.handle))
}

// RequiredInputFrameCount reports the input frames needed to produce
// outputFrameCount output frames.
func (c *DataConverter) RequiredInputFrameCount(outputFrameCount uint64) (uint64, error) {
	if c == nil || c.handle == nil {
		return 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var count uint64
	if result := c.lib.bindings.maDataConverterGetRequiredInputFrameCount(c.handle, outputFrameCount, &count); result != Success {
		return 0, c.lib.resultError("ma_data_converter_get_required_input_frame_count", result)
	}
	return count, nil
}

// ExpectedOutputFrameCount reports the output frames produced from
// inputFrameCount input frames.
func (c *DataConverter) ExpectedOutputFrameCount(inputFrameCount uint64) (uint64, error) {
	if c == nil || c.handle == nil {
		return 0, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return 0, err
	}
	var count uint64
	if result := c.lib.bindings.maDataConverterGetExpectedOutputFrameCount(c.handle, inputFrameCount, &count); result != Success {
		return 0, c.lib.resultError("ma_data_converter_get_expected_output_frame_count", result)
	}
	return count, nil
}

// InputChannelMap reports the converter's resolved input channel map.
func (c *DataConverter) InputChannelMap() (ChannelMap, error) {
	if c == nil || c.handle == nil {
		return ChannelMap{}, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return ChannelMap{}, err
	}
	if c.channelsIn == 0 {
		return ChannelMap{}, nil
	}

	m := ChannelMap{lib: c.lib, channels: make([]uint8, c.channelsIn)}
	if result := c.lib.bindings.maDataConverterGetInputChannelMap(c.handle, &m.channels[0], uintptr(c.channelsIn)); result != Success {
		return ChannelMap{}, c.lib.resultError("ma_data_converter_get_input_channel_map", result)
	}
	return m, nil
}

// OutputChannelMap reports the converter's resolved output channel map.
func (c *DataConverter) OutputChannelMap() (ChannelMap, error) {
	if c == nil || c.handle == nil {
		return ChannelMap{}, fmt.Errorf("mago: nil data converter")
	}
	if err := c.lib.ensureOpen(); err != nil {
		return ChannelMap{}, err
	}
	if c.channelsOut == 0 {
		return ChannelMap{}, nil
	}

	m := ChannelMap{lib: c.lib, channels: make([]uint8, c.channelsOut)}
	if result := c.lib.bindings.maDataConverterGetOutputChannelMap(c.handle, &m.channels[0], uintptr(c.channelsOut)); result != Success {
		return ChannelMap{}, c.lib.resultError("ma_data_converter_get_output_channel_map", result)
	}
	return m, nil
}

// Close uninitializes the converter and frees it.
func (c *DataConverter) Close() error {
	if c == nil || c.handle == nil {
		return nil
	}
	if err := c.lib.ensureOpen(); err != nil {
		return err
	}

	c.lib.bindings.maDataConverterUninit(c.handle, nil)
	c.lib.bindings.magoFree(unsafe.Pointer(c.handle))
	c.handle = nil
	return nil
}
