package mago

import (
	"testing"
	"unsafe"

	"github.com/darkliquid/mago/internal/abi"
)

func TestMirroredStructLayouts(t *testing.T) {
	probe, err := abi.Run(".")
	if err != nil {
		t.Fatalf("layout probe: %v", err)
	}

	if got, want := uint64(unsafe.Sizeof(deviceIDNative{})), probe["sizeof:ma_device_id"]; got != want {
		t.Errorf("sizeof ma_device_id: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.Name)), probe["offsetof:ma_device_info.name"]; got != want {
		t.Errorf("offsetof ma_device_info.name: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.IsDefault)), probe["offsetof:ma_device_info.isDefault"]; got != want {
		t.Errorf("offsetof ma_device_info.isDefault: mirror %d, header %d", got, want)
	}

	if got, want := uint64(unsafe.Sizeof(Channel(0))), probe["sizeof:ma_channel"]; got != want {
		t.Errorf("sizeof ma_channel: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(channelConverterConfigNative{})), probe["sizeof:ma_channel_converter_config"]; got != want {
		t.Errorf("sizeof channelConverterConfigNative: mirror %d, header %d", got, want)
	}

	mirror := channelConverterConfigNative{}
	offsets := map[string]struct {
		got uintptr
		key string
	}{
		"pChannelMapIn":                   {unsafe.Offsetof(mirror.ChannelMapIn), "offsetof:ma_channel_converter_config.pChannelMapIn"},
		"pChannelMapOut":                  {unsafe.Offsetof(mirror.ChannelMapOut), "offsetof:ma_channel_converter_config.pChannelMapOut"},
		"mixingMode":                      {unsafe.Offsetof(mirror.MixingMode), "offsetof:ma_channel_converter_config.mixingMode"},
		"calculateLFEFromSpatialChannels": {unsafe.Offsetof(mirror.CalculateLFEFromSpatialChannels), "offsetof:ma_channel_converter_config.calculateLFEFromSpatialChannels"},
		"ppWeights":                       {unsafe.Offsetof(mirror.Weights), "offsetof:ma_channel_converter_config.ppWeights"},
	}
	for name, check := range offsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_channel_converter_config.%s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(resamplerConfigNative{})), probe["sizeof:ma_resampler_config"]; got != want {
		t.Errorf("sizeof resamplerConfigNative: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(linearResamplerConfigNative{})), probe["sizeof:ma_linear_resampler_config"]; got != want {
		t.Errorf("sizeof linearResamplerConfigNative: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(dataConverterConfigNative{})), probe["sizeof:ma_data_converter_config"]; got != want {
		t.Errorf("sizeof dataConverterConfigNative: mirror %d, header %d", got, want)
	}

	resampler := resamplerConfigNative{}
	linear := linearResamplerConfigNative{}
	converter := dataConverterConfigNative{}
	resampleOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"resampler.algorithm":        {unsafe.Offsetof(resampler.Algorithm), "offsetof:ma_resampler_config.algorithm"},
		"resampler.linear.lpfOrder":  {unsafe.Offsetof(resampler.Linear) + unsafe.Offsetof(resamplerLinearConfigNative{}.LPFOrder), "offsetof:ma_resampler_config.linear.lpfOrder"},
		"linear.lpfOrder":            {unsafe.Offsetof(linear.LPFOrder), "offsetof:ma_linear_resampler_config.lpfOrder"},
		"linear.lpfNyquistFactor":    {unsafe.Offsetof(linear.LPFNyquistFactor), "offsetof:ma_linear_resampler_config.lpfNyquistFactor"},
		"converter.pChannelMapIn":    {unsafe.Offsetof(converter.ChannelMapIn), "offsetof:ma_data_converter_config.pChannelMapIn"},
		"converter.ditherMode":       {unsafe.Offsetof(converter.DitherMode), "offsetof:ma_data_converter_config.ditherMode"},
		"converter.channelMixMode":   {unsafe.Offsetof(converter.ChannelMixMode), "offsetof:ma_data_converter_config.channelMixMode"},
		"converter.lfeFromSpatial":   {unsafe.Offsetof(converter.CalculateLFEFromSpatialChannels), "offsetof:ma_data_converter_config.calculateLFEFromSpatialChannels"},
		"converter.ppChannelWeights": {unsafe.Offsetof(converter.ChannelWeights), "offsetof:ma_data_converter_config.ppChannelWeights"},
		"converter.allowDynamicRate": {unsafe.Offsetof(converter.AllowDynamicSampleRate), "offsetof:ma_data_converter_config.allowDynamicSampleRate"},
		"converter.resampling":       {unsafe.Offsetof(converter.Resampling), "offsetof:ma_data_converter_config.resampling"},
	}
	for name, check := range resampleOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof %s: mirror %d, header %d", name, got, want)
		}
	}

	if got, want := uint64(unsafe.Sizeof(allocationCallbacksNative{})), probe["sizeof:ma_allocation_callbacks"]; got != want {
		t.Errorf("sizeof allocationCallbacksNative: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Sizeof(audioBufferConfigNative{})), probe["sizeof:ma_audio_buffer_config"]; got != want {
		t.Errorf("sizeof audioBufferConfigNative: mirror %d, header %d", got, want)
	}

	audioBuffer := audioBufferConfigNative{}
	bufferOffsets := map[string]struct {
		got uintptr
		key string
	}{
		"sizeInFrames":        {unsafe.Offsetof(audioBuffer.SizeInFrames), "offsetof:ma_audio_buffer_config.sizeInFrames"},
		"pData":               {unsafe.Offsetof(audioBuffer.Data), "offsetof:ma_audio_buffer_config.pData"},
		"allocationCallbacks": {unsafe.Offsetof(audioBuffer.AllocationCallbacks), "offsetof:ma_audio_buffer_config.allocationCallbacks"},
	}
	for name, check := range bufferOffsets {
		if got, want := uint64(check.got), probe[check.key]; got != want {
			t.Errorf("offsetof ma_audio_buffer_config.%s: mirror %d, header %d", name, got, want)
		}
	}
}
