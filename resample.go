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
	lib    *Library
	handle *resamplerHandle
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

	return &Resampler{lib: lib, handle: handle}, nil
}

// ProcessPCMFrames resamples up to framesIn input frames into the output buffer.
// It returns how many input frames were consumed and how many output frames were
// produced.
func (r *Resampler) ProcessPCMFrames(in unsafe.Pointer, framesIn uint64, out unsafe.Pointer, framesOut uint64) (uint64, uint64, error) {
	if r == nil || r.handle == nil {
		return 0, 0, fmt.Errorf("mago: nil resampler")
	}
	if err := r.lib.ensureOpen(); err != nil {
		return 0, 0, err
	}

	inCount, outCount := framesIn, framesOut
	result := r.lib.bindings.maResamplerProcessPCMFrames(r.handle, in, &inCount, out, &outCount)
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
