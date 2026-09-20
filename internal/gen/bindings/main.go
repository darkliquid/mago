package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type functionSpec struct {
	FieldName string
	Symbol    string
	Type      string
}

type constSpec struct {
	Name  string
	Type  string
	Value string
}

var functions = []functionSpec{
	{FieldName: "maVersion", Symbol: "ma_version", Type: "func(*uint32, *uint32, *uint32)"},
	{FieldName: "maVersionString", Symbol: "ma_version_string", Type: "func() string"},
	{FieldName: "maResultDescription", Symbol: "ma_result_description", Type: "func(Result) string"},
	{FieldName: "maContextInit", Symbol: "ma_context_init", Type: "func(*Backend, uint32, unsafe.Pointer, *contextHandle) Result"},
	{FieldName: "maContextUninit", Symbol: "ma_context_uninit", Type: "func(*contextHandle)"},
	{FieldName: "maContextEnumerateDevices", Symbol: "ma_context_enumerate_devices", Type: "func(*contextHandle, uintptr, uintptr) Result"},
	{FieldName: "magoDeviceInit", Symbol: "mago_device_init", Type: "func(*contextHandle, *deviceConfigNative, **deviceHandle) Result"},
	{FieldName: "magoDeviceUninitFree", Symbol: "mago_device_uninit_free", Type: "func(*deviceHandle)"},
	{FieldName: "maDeviceStart", Symbol: "ma_device_start", Type: "func(*deviceHandle) Result"},
	{FieldName: "maDeviceStop", Symbol: "ma_device_stop", Type: "func(*deviceHandle) Result"},
	{FieldName: "maDeviceGetState", Symbol: "ma_device_get_state", Type: "func(*deviceHandle) DeviceState"},
	{FieldName: "maDeviceGetName", Symbol: "ma_device_get_name", Type: "func(*deviceHandle, DeviceType, *byte, uintptr, *uintptr) Result"},
	{FieldName: "maDeviceGetInfo", Symbol: "ma_device_get_info", Type: "func(*deviceHandle, DeviceType, *deviceInfoNative) Result"},
	{FieldName: "maDeviceGetLog", Symbol: "ma_device_get_log", Type: "func(*deviceHandle) *logHandle"},
	{FieldName: "maDeviceGetContext", Symbol: "ma_device_get_context", Type: "func(*deviceHandle) *contextHandle"},
	{FieldName: "maDeviceSetMasterVolume", Symbol: "ma_device_set_master_volume", Type: "func(*deviceHandle, float32) Result"},
	{FieldName: "maDeviceGetMasterVolume", Symbol: "ma_device_get_master_volume", Type: "func(*deviceHandle, *float32) Result"},
	{FieldName: "maDeviceSetMasterVolumeDB", Symbol: "ma_device_set_master_volume_db", Type: "func(*deviceHandle, float32) Result"},
	{FieldName: "maDeviceGetMasterVolumeDB", Symbol: "ma_device_get_master_volume_db", Type: "func(*deviceHandle, *float32) Result"},
	{FieldName: "maContextGetLog", Symbol: "ma_context_get_log", Type: "func(*contextHandle) *logHandle"},
	{FieldName: "maContextGetDeviceInfo", Symbol: "ma_context_get_device_info", Type: "func(*contextHandle, DeviceType, unsafe.Pointer, *deviceInfoNative) Result"},
	{FieldName: "magoAlloc", Symbol: "mago_alloc", Type: "func(int32) unsafe.Pointer"},
	{FieldName: "magoFree", Symbol: "mago_free", Type: "func(unsafe.Pointer)"},
	{FieldName: "maLogInit", Symbol: "ma_log_init", Type: "func(unsafe.Pointer, *logHandle) Result"},
	{FieldName: "maLogUninit", Symbol: "ma_log_uninit", Type: "func(*logHandle)"},
	{FieldName: "maLogPost", Symbol: "ma_log_post", Type: "func(*logHandle, uint32, string) Result"},
	{FieldName: "maLogLevelToString", Symbol: "ma_log_level_to_string", Type: "func(uint32) string"},
	{FieldName: "magoLogRegisterCallback", Symbol: "mago_log_register_callback", Type: "func(*logHandle, uintptr, uintptr) Result"},
	{FieldName: "magoLogUnregisterCallback", Symbol: "mago_log_unregister_callback", Type: "func(*logHandle, uintptr, uintptr) Result"},
	{FieldName: "magoContextConfigInit", Symbol: "mago_context_config_init", Type: "func(unsafe.Pointer)"},
	{FieldName: "magoContextConfigSetLog", Symbol: "mago_context_config_set_log", Type: "func(unsafe.Pointer, *logHandle)"},
	{FieldName: "maPCMConvert", Symbol: "ma_pcm_convert", Type: "func(unsafe.Pointer, Format, unsafe.Pointer, Format, uint64, DitherMode)"},
	{FieldName: "maConvertPCMFramesFormat", Symbol: "ma_convert_pcm_frames_format", Type: "func(unsafe.Pointer, Format, unsafe.Pointer, Format, uint64, uint32, DitherMode)"},
	{FieldName: "maConvertFrames", Symbol: "ma_convert_frames", Type: "func(unsafe.Pointer, uint64, Format, uint32, uint32, unsafe.Pointer, uint64, Format, uint32, uint32) uint64"},
	{FieldName: "maChannelMapInitStandard", Symbol: "ma_channel_map_init_standard", Type: "func(StandardChannelMap, *uint8, uintptr, uint32)"},
	{FieldName: "maChannelMapInitBlank", Symbol: "ma_channel_map_init_blank", Type: "func(*uint8, uint32)"},
	{FieldName: "maChannelMapCopy", Symbol: "ma_channel_map_copy", Type: "func(*uint8, *uint8, uint32)"},
	{FieldName: "maChannelMapCopyOrDefault", Symbol: "ma_channel_map_copy_or_default", Type: "func(*uint8, uintptr, *uint8, uint32)"},
	{FieldName: "maChannelMapGetChannel", Symbol: "ma_channel_map_get_channel", Type: "func(*uint8, uint32, uint32) Channel"},
	{FieldName: "maChannelMapToString", Symbol: "ma_channel_map_to_string", Type: "func(*uint8, uint32, *byte, uintptr) uintptr"},
	{FieldName: "maChannelConverterInit", Symbol: "ma_channel_converter_init", Type: "func(*channelConverterConfigNative, unsafe.Pointer, *channelConverterHandle) Result"},
	{FieldName: "maChannelConverterUninit", Symbol: "ma_channel_converter_uninit", Type: "func(*channelConverterHandle, unsafe.Pointer)"},
	{FieldName: "maChannelConverterProcessPCMFrames", Symbol: "ma_channel_converter_process_pcm_frames", Type: "func(*channelConverterHandle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maChannelConverterGetInputChannelMap", Symbol: "ma_channel_converter_get_input_channel_map", Type: "func(*channelConverterHandle, *uint8, uintptr) Result"},
	{FieldName: "maChannelConverterGetOutputChannelMap", Symbol: "ma_channel_converter_get_output_channel_map", Type: "func(*channelConverterHandle, *uint8, uintptr) Result"},
	{FieldName: "maResamplerInit", Symbol: "ma_resampler_init", Type: "func(*resamplerConfigNative, unsafe.Pointer, *resamplerHandle) Result"},
	{FieldName: "maResamplerUninit", Symbol: "ma_resampler_uninit", Type: "func(*resamplerHandle, unsafe.Pointer)"},
	{FieldName: "maResamplerProcessPCMFrames", Symbol: "ma_resampler_process_pcm_frames", Type: "func(*resamplerHandle, unsafe.Pointer, *uint64, unsafe.Pointer, *uint64) Result"},
	{FieldName: "maResamplerSetRate", Symbol: "ma_resampler_set_rate", Type: "func(*resamplerHandle, uint32, uint32) Result"},
	{FieldName: "maResamplerSetRateRatio", Symbol: "ma_resampler_set_rate_ratio", Type: "func(*resamplerHandle, float32) Result"},
	{FieldName: "maResamplerReset", Symbol: "ma_resampler_reset", Type: "func(*resamplerHandle) Result"},
	{FieldName: "maResamplerGetRequiredInputFrameCount", Symbol: "ma_resampler_get_required_input_frame_count", Type: "func(*resamplerHandle, uint64, *uint64) Result"},
	{FieldName: "maResamplerGetExpectedOutputFrameCount", Symbol: "ma_resampler_get_expected_output_frame_count", Type: "func(*resamplerHandle, uint64, *uint64) Result"},
	{FieldName: "maLinearResamplerInit", Symbol: "ma_linear_resampler_init", Type: "func(*linearResamplerConfigNative, unsafe.Pointer, *linearResamplerHandle) Result"},
	{FieldName: "maLinearResamplerUninit", Symbol: "ma_linear_resampler_uninit", Type: "func(*linearResamplerHandle, unsafe.Pointer)"},
	{FieldName: "maLinearResamplerProcessPCMFrames", Symbol: "ma_linear_resampler_process_pcm_frames", Type: "func(*linearResamplerHandle, unsafe.Pointer, *uint64, unsafe.Pointer, *uint64) Result"},
	{FieldName: "maLinearResamplerSetRate", Symbol: "ma_linear_resampler_set_rate", Type: "func(*linearResamplerHandle, uint32, uint32) Result"},
	{FieldName: "maLinearResamplerSetRateRatio", Symbol: "ma_linear_resampler_set_rate_ratio", Type: "func(*linearResamplerHandle, float32) Result"},
	{FieldName: "maLinearResamplerReset", Symbol: "ma_linear_resampler_reset", Type: "func(*linearResamplerHandle) Result"},
	{FieldName: "maLinearResamplerGetRequiredInputFrameCount", Symbol: "ma_linear_resampler_get_required_input_frame_count", Type: "func(*linearResamplerHandle, uint64, *uint64) Result"},
	{FieldName: "maLinearResamplerGetExpectedOutputFrameCount", Symbol: "ma_linear_resampler_get_expected_output_frame_count", Type: "func(*linearResamplerHandle, uint64, *uint64) Result"},
	{FieldName: "maDataConverterInit", Symbol: "ma_data_converter_init", Type: "func(*dataConverterConfigNative, unsafe.Pointer, *dataConverterHandle) Result"},
	{FieldName: "maDataConverterUninit", Symbol: "ma_data_converter_uninit", Type: "func(*dataConverterHandle, unsafe.Pointer)"},
	{FieldName: "maDataConverterProcessPCMFrames", Symbol: "ma_data_converter_process_pcm_frames", Type: "func(*dataConverterHandle, unsafe.Pointer, *uint64, unsafe.Pointer, *uint64) Result"},
	{FieldName: "maDataConverterSetRate", Symbol: "ma_data_converter_set_rate", Type: "func(*dataConverterHandle, uint32, uint32) Result"},
	{FieldName: "maDataConverterSetRateRatio", Symbol: "ma_data_converter_set_rate_ratio", Type: "func(*dataConverterHandle, float32) Result"},
	{FieldName: "maDataConverterReset", Symbol: "ma_data_converter_reset", Type: "func(*dataConverterHandle) Result"},
	{FieldName: "maDataConverterGetRequiredInputFrameCount", Symbol: "ma_data_converter_get_required_input_frame_count", Type: "func(*dataConverterHandle, uint64, *uint64) Result"},
	{FieldName: "maDataConverterGetExpectedOutputFrameCount", Symbol: "ma_data_converter_get_expected_output_frame_count", Type: "func(*dataConverterHandle, uint64, *uint64) Result"},
	{FieldName: "maDataConverterGetInputChannelMap", Symbol: "ma_data_converter_get_input_channel_map", Type: "func(*dataConverterHandle, *uint8, uintptr) Result"},
	{FieldName: "maDataConverterGetOutputChannelMap", Symbol: "ma_data_converter_get_output_channel_map", Type: "func(*dataConverterHandle, *uint8, uintptr) Result"},
	{FieldName: "maAudioBufferInit", Symbol: "ma_audio_buffer_init", Type: "func(*audioBufferConfigNative, *audioBufferHandle) Result"},
	{FieldName: "maAudioBufferInitCopy", Symbol: "ma_audio_buffer_init_copy", Type: "func(*audioBufferConfigNative, *audioBufferHandle) Result"},
	{FieldName: "maAudioBufferUninit", Symbol: "ma_audio_buffer_uninit", Type: "func(*audioBufferHandle)"},
	{FieldName: "maAudioBufferReadPCMFrames", Symbol: "ma_audio_buffer_read_pcm_frames", Type: "func(*audioBufferHandle, unsafe.Pointer, uint64, uint32) uint64"},
	{FieldName: "maAudioBufferSeekToPCMFrame", Symbol: "ma_audio_buffer_seek_to_pcm_frame", Type: "func(*audioBufferHandle, uint64) Result"},
	{FieldName: "maAudioBufferMap", Symbol: "ma_audio_buffer_map", Type: "func(*audioBufferHandle, *unsafe.Pointer, *uint64) Result"},
	{FieldName: "maAudioBufferUnmap", Symbol: "ma_audio_buffer_unmap", Type: "func(*audioBufferHandle, uint64) Result"},
	{FieldName: "maAudioBufferGetCursorInPCMFrames", Symbol: "ma_audio_buffer_get_cursor_in_pcm_frames", Type: "func(*audioBufferHandle, *uint64) Result"},
	{FieldName: "maAudioBufferGetLengthInPCMFrames", Symbol: "ma_audio_buffer_get_length_in_pcm_frames", Type: "func(*audioBufferHandle, *uint64) Result"},
	{FieldName: "maAudioBufferGetAvailableFrames", Symbol: "ma_audio_buffer_get_available_frames", Type: "func(*audioBufferHandle, *uint64) Result"},
	{FieldName: "maAudioBufferRefInit", Symbol: "ma_audio_buffer_ref_init", Type: "func(Format, uint32, unsafe.Pointer, uint64, *audioBufferRefHandle) Result"},
	{FieldName: "maAudioBufferRefUninit", Symbol: "ma_audio_buffer_ref_uninit", Type: "func(*audioBufferRefHandle)"},
	{FieldName: "maAudioBufferRefSetData", Symbol: "ma_audio_buffer_ref_set_data", Type: "func(*audioBufferRefHandle, unsafe.Pointer, uint64) Result"},
	{FieldName: "maAudioBufferRefReadPCMFrames", Symbol: "ma_audio_buffer_ref_read_pcm_frames", Type: "func(*audioBufferRefHandle, unsafe.Pointer, uint64, uint32) uint64"},
	{FieldName: "maAudioBufferRefSeekToPCMFrame", Symbol: "ma_audio_buffer_ref_seek_to_pcm_frame", Type: "func(*audioBufferRefHandle, uint64) Result"},
	{FieldName: "maAudioBufferRefMap", Symbol: "ma_audio_buffer_ref_map", Type: "func(*audioBufferRefHandle, *unsafe.Pointer, *uint64) Result"},
	{FieldName: "maAudioBufferRefUnmap", Symbol: "ma_audio_buffer_ref_unmap", Type: "func(*audioBufferRefHandle, uint64) Result"},
	{FieldName: "maAudioBufferRefAtEnd", Symbol: "ma_audio_buffer_ref_at_end", Type: "func(*audioBufferRefHandle) uint32"},
	{FieldName: "maAudioBufferRefGetCursorInPCMFrames", Symbol: "ma_audio_buffer_ref_get_cursor_in_pcm_frames", Type: "func(*audioBufferRefHandle, *uint64) Result"},
	{FieldName: "maAudioBufferRefGetLengthInPCMFrames", Symbol: "ma_audio_buffer_ref_get_length_in_pcm_frames", Type: "func(*audioBufferRefHandle, *uint64) Result"},
	{FieldName: "maAudioBufferRefGetAvailableFrames", Symbol: "ma_audio_buffer_ref_get_available_frames", Type: "func(*audioBufferRefHandle, *uint64) Result"},
	{FieldName: "maRBInit", Symbol: "ma_rb_init", Type: "func(uintptr, unsafe.Pointer, unsafe.Pointer, *ringBufferHandle) Result"},
	{FieldName: "maRBInitEx", Symbol: "ma_rb_init_ex", Type: "func(uintptr, uintptr, uintptr, unsafe.Pointer, unsafe.Pointer, *ringBufferHandle) Result"},
	{FieldName: "maRBUninit", Symbol: "ma_rb_uninit", Type: "func(*ringBufferHandle)"},
	{FieldName: "maRBReset", Symbol: "ma_rb_reset", Type: "func(*ringBufferHandle)"},
	{FieldName: "maRBAcquireRead", Symbol: "ma_rb_acquire_read", Type: "func(*ringBufferHandle, *uintptr, *unsafe.Pointer) Result"},
	{FieldName: "maRBCommitRead", Symbol: "ma_rb_commit_read", Type: "func(*ringBufferHandle, uintptr) Result"},
	{FieldName: "maRBAcquireWrite", Symbol: "ma_rb_acquire_write", Type: "func(*ringBufferHandle, *uintptr, *unsafe.Pointer) Result"},
	{FieldName: "maRBCommitWrite", Symbol: "ma_rb_commit_write", Type: "func(*ringBufferHandle, uintptr) Result"},
	{FieldName: "maRBSeekRead", Symbol: "ma_rb_seek_read", Type: "func(*ringBufferHandle, uintptr) Result"},
	{FieldName: "maRBSeekWrite", Symbol: "ma_rb_seek_write", Type: "func(*ringBufferHandle, uintptr) Result"},
	{FieldName: "maRBPointerDistance", Symbol: "ma_rb_pointer_distance", Type: "func(*ringBufferHandle) int32"},
	{FieldName: "maRBAvailableRead", Symbol: "ma_rb_available_read", Type: "func(*ringBufferHandle) uint32"},
	{FieldName: "maRBAvailableWrite", Symbol: "ma_rb_available_write", Type: "func(*ringBufferHandle) uint32"},
	{FieldName: "maRBGetSubbufferSize", Symbol: "ma_rb_get_subbuffer_size", Type: "func(*ringBufferHandle) uintptr"},
	{FieldName: "maRBGetSubbufferStride", Symbol: "ma_rb_get_subbuffer_stride", Type: "func(*ringBufferHandle) uintptr"},
	{FieldName: "maRBGetSubbufferOffset", Symbol: "ma_rb_get_subbuffer_offset", Type: "func(*ringBufferHandle, uintptr) uintptr"},
	{FieldName: "maRBGetSubbufferPtr", Symbol: "ma_rb_get_subbuffer_ptr", Type: "func(*ringBufferHandle, uintptr, unsafe.Pointer) unsafe.Pointer"},
	{FieldName: "maPCMRBInit", Symbol: "ma_pcm_rb_init", Type: "func(Format, uint32, uint32, unsafe.Pointer, unsafe.Pointer, *pcmRingBufferHandle) Result"},
	{FieldName: "maPCMRBInitEx", Symbol: "ma_pcm_rb_init_ex", Type: "func(Format, uint32, uint32, uint32, uint32, unsafe.Pointer, unsafe.Pointer, *pcmRingBufferHandle) Result"},
	{FieldName: "maPCMRBUninit", Symbol: "ma_pcm_rb_uninit", Type: "func(*pcmRingBufferHandle)"},
	{FieldName: "maPCMRBReset", Symbol: "ma_pcm_rb_reset", Type: "func(*pcmRingBufferHandle)"},
	{FieldName: "maPCMRBAcquireRead", Symbol: "ma_pcm_rb_acquire_read", Type: "func(*pcmRingBufferHandle, *uint32, *unsafe.Pointer) Result"},
	{FieldName: "maPCMRBCommitRead", Symbol: "ma_pcm_rb_commit_read", Type: "func(*pcmRingBufferHandle, uint32) Result"},
	{FieldName: "maPCMRBAcquireWrite", Symbol: "ma_pcm_rb_acquire_write", Type: "func(*pcmRingBufferHandle, *uint32, *unsafe.Pointer) Result"},
	{FieldName: "maPCMRBCommitWrite", Symbol: "ma_pcm_rb_commit_write", Type: "func(*pcmRingBufferHandle, uint32) Result"},
	{FieldName: "maPCMRBSeekRead", Symbol: "ma_pcm_rb_seek_read", Type: "func(*pcmRingBufferHandle, uint32) Result"},
	{FieldName: "maPCMRBSeekWrite", Symbol: "ma_pcm_rb_seek_write", Type: "func(*pcmRingBufferHandle, uint32) Result"},
	{FieldName: "maPCMRBPointerDistance", Symbol: "ma_pcm_rb_pointer_distance", Type: "func(*pcmRingBufferHandle) int32"},
	{FieldName: "maPCMRBAvailableRead", Symbol: "ma_pcm_rb_available_read", Type: "func(*pcmRingBufferHandle) uint32"},
	{FieldName: "maPCMRBAvailableWrite", Symbol: "ma_pcm_rb_available_write", Type: "func(*pcmRingBufferHandle) uint32"},
	{FieldName: "maPCMRBGetFormat", Symbol: "ma_pcm_rb_get_format", Type: "func(*pcmRingBufferHandle) Format"},
	{FieldName: "maPCMRBGetChannels", Symbol: "ma_pcm_rb_get_channels", Type: "func(*pcmRingBufferHandle) uint32"},
	{FieldName: "maPCMRBGetSampleRate", Symbol: "ma_pcm_rb_get_sample_rate", Type: "func(*pcmRingBufferHandle) uint32"},
	{FieldName: "maPCMRBGetSubbufferSize", Symbol: "ma_pcm_rb_get_subbuffer_size", Type: "func(*pcmRingBufferHandle) uint32"},
	{FieldName: "maPCMRBGetSubbufferStride", Symbol: "ma_pcm_rb_get_subbuffer_stride", Type: "func(*pcmRingBufferHandle) uint32"},
	{FieldName: "maPCMRBGetSubbufferOffset", Symbol: "ma_pcm_rb_get_subbuffer_offset", Type: "func(*pcmRingBufferHandle, uint32) uint32"},
	{FieldName: "maPCMRBGetSubbufferPtr", Symbol: "ma_pcm_rb_get_subbuffer_ptr", Type: "func(*pcmRingBufferHandle, uint32, unsafe.Pointer) unsafe.Pointer"},
	{FieldName: "maWaveformInit", Symbol: "ma_waveform_init", Type: "func(*waveformConfigNative, *waveformHandle) Result"},
	{FieldName: "maWaveformUninit", Symbol: "ma_waveform_uninit", Type: "func(*waveformHandle)"},
	{FieldName: "maWaveformReadPCMFrames", Symbol: "ma_waveform_read_pcm_frames", Type: "func(*waveformHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "maWaveformSeekToPCMFrame", Symbol: "ma_waveform_seek_to_pcm_frame", Type: "func(*waveformHandle, uint64) Result"},
	{FieldName: "maWaveformSetAmplitude", Symbol: "ma_waveform_set_amplitude", Type: "func(*waveformHandle, float64) Result"},
	{FieldName: "maWaveformSetFrequency", Symbol: "ma_waveform_set_frequency", Type: "func(*waveformHandle, float64) Result"},
	{FieldName: "maWaveformSetType", Symbol: "ma_waveform_set_type", Type: "func(*waveformHandle, WaveformType) Result"},
	{FieldName: "maWaveformSetSampleRate", Symbol: "ma_waveform_set_sample_rate", Type: "func(*waveformHandle, uint32) Result"},
	{FieldName: "maNoiseInit", Symbol: "ma_noise_init", Type: "func(*noiseConfigNative, unsafe.Pointer, *noiseHandle) Result"},
	{FieldName: "maNoiseUninit", Symbol: "ma_noise_uninit", Type: "func(*noiseHandle, unsafe.Pointer)"},
	{FieldName: "maNoiseReadPCMFrames", Symbol: "ma_noise_read_pcm_frames", Type: "func(*noiseHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "maNoiseSetAmplitude", Symbol: "ma_noise_set_amplitude", Type: "func(*noiseHandle, float64) Result"},
	{FieldName: "maNoiseSetSeed", Symbol: "ma_noise_set_seed", Type: "func(*noiseHandle, int32) Result"},
	{FieldName: "maNoiseSetType", Symbol: "ma_noise_set_type", Type: "func(*noiseHandle, NoiseType) Result"},
	{FieldName: "maBiquadInit", Symbol: "ma_biquad_init", Type: "func(*biquadConfigNative, unsafe.Pointer, *biquadHandle) Result"},
	{FieldName: "maBiquadUninit", Symbol: "ma_biquad_uninit", Type: "func(*biquadHandle, unsafe.Pointer)"},
	{FieldName: "maBiquadReinit", Symbol: "ma_biquad_reinit", Type: "func(*biquadConfigNative, *biquadHandle) Result"},
	{FieldName: "maBiquadClearCache", Symbol: "ma_biquad_clear_cache", Type: "func(*biquadHandle) Result"},
	{FieldName: "maBiquadProcessPCMFrames", Symbol: "ma_biquad_process_pcm_frames", Type: "func(*biquadHandle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maBiquadGetLatency", Symbol: "ma_biquad_get_latency", Type: "func(*biquadHandle) uint32"},
	{FieldName: "maLPF1Init", Symbol: "ma_lpf1_init", Type: "func(*lpf1ConfigNative, unsafe.Pointer, *lpf1Handle) Result"},
	{FieldName: "maLPF1Uninit", Symbol: "ma_lpf1_uninit", Type: "func(*lpf1Handle, unsafe.Pointer)"},
	{FieldName: "maLPF1Reinit", Symbol: "ma_lpf1_reinit", Type: "func(*lpf1ConfigNative, *lpf1Handle) Result"},
	{FieldName: "maLPF1ClearCache", Symbol: "ma_lpf1_clear_cache", Type: "func(*lpf1Handle) Result"},
	{FieldName: "maLPF1ProcessPCMFrames", Symbol: "ma_lpf1_process_pcm_frames", Type: "func(*lpf1Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maLPF1GetLatency", Symbol: "ma_lpf1_get_latency", Type: "func(*lpf1Handle) uint32"},
	{FieldName: "maLPF2Init", Symbol: "ma_lpf2_init", Type: "func(*lpf2ConfigNative, unsafe.Pointer, *lpf2Handle) Result"},
	{FieldName: "maLPF2Uninit", Symbol: "ma_lpf2_uninit", Type: "func(*lpf2Handle, unsafe.Pointer)"},
	{FieldName: "maLPF2Reinit", Symbol: "ma_lpf2_reinit", Type: "func(*lpf2ConfigNative, *lpf2Handle) Result"},
	{FieldName: "maLPF2ClearCache", Symbol: "ma_lpf2_clear_cache", Type: "func(*lpf2Handle) Result"},
	{FieldName: "maLPF2ProcessPCMFrames", Symbol: "ma_lpf2_process_pcm_frames", Type: "func(*lpf2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maLPF2GetLatency", Symbol: "ma_lpf2_get_latency", Type: "func(*lpf2Handle) uint32"},
	{FieldName: "maLPFInit", Symbol: "ma_lpf_init", Type: "func(*lpfConfigNative, unsafe.Pointer, *lpfHandle) Result"},
	{FieldName: "maLPFUninit", Symbol: "ma_lpf_uninit", Type: "func(*lpfHandle, unsafe.Pointer)"},
	{FieldName: "maLPFReinit", Symbol: "ma_lpf_reinit", Type: "func(*lpfConfigNative, *lpfHandle) Result"},
	{FieldName: "maLPFClearCache", Symbol: "ma_lpf_clear_cache", Type: "func(*lpfHandle) Result"},
	{FieldName: "maLPFProcessPCMFrames", Symbol: "ma_lpf_process_pcm_frames", Type: "func(*lpfHandle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maLPFGetLatency", Symbol: "ma_lpf_get_latency", Type: "func(*lpfHandle) uint32"},
	{FieldName: "maHPF1Init", Symbol: "ma_hpf1_init", Type: "func(*hpf1ConfigNative, unsafe.Pointer, *hpf1Handle) Result"},
	{FieldName: "maHPF1Uninit", Symbol: "ma_hpf1_uninit", Type: "func(*hpf1Handle, unsafe.Pointer)"},
	{FieldName: "maHPF1Reinit", Symbol: "ma_hpf1_reinit", Type: "func(*hpf1ConfigNative, *hpf1Handle) Result"},
	{FieldName: "maHPF1ProcessPCMFrames", Symbol: "ma_hpf1_process_pcm_frames", Type: "func(*hpf1Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maHPF1GetLatency", Symbol: "ma_hpf1_get_latency", Type: "func(*hpf1Handle) uint32"},
	{FieldName: "maHPF2Init", Symbol: "ma_hpf2_init", Type: "func(*hpf2ConfigNative, unsafe.Pointer, *hpf2Handle) Result"},
	{FieldName: "maHPF2Uninit", Symbol: "ma_hpf2_uninit", Type: "func(*hpf2Handle, unsafe.Pointer)"},
	{FieldName: "maHPF2Reinit", Symbol: "ma_hpf2_reinit", Type: "func(*hpf2ConfigNative, *hpf2Handle) Result"},
	{FieldName: "maHPF2ProcessPCMFrames", Symbol: "ma_hpf2_process_pcm_frames", Type: "func(*hpf2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maHPF2GetLatency", Symbol: "ma_hpf2_get_latency", Type: "func(*hpf2Handle) uint32"},
	{FieldName: "maHPFInit", Symbol: "ma_hpf_init", Type: "func(*hpfConfigNative, unsafe.Pointer, *hpfHandle) Result"},
	{FieldName: "maHPFUninit", Symbol: "ma_hpf_uninit", Type: "func(*hpfHandle, unsafe.Pointer)"},
	{FieldName: "maHPFReinit", Symbol: "ma_hpf_reinit", Type: "func(*hpfConfigNative, *hpfHandle) Result"},
	{FieldName: "maHPFProcessPCMFrames", Symbol: "ma_hpf_process_pcm_frames", Type: "func(*hpfHandle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maHPFGetLatency", Symbol: "ma_hpf_get_latency", Type: "func(*hpfHandle) uint32"},
	{FieldName: "maBPF2Init", Symbol: "ma_bpf2_init", Type: "func(*bpf2ConfigNative, unsafe.Pointer, *bpf2Handle) Result"},
	{FieldName: "maBPF2Uninit", Symbol: "ma_bpf2_uninit", Type: "func(*bpf2Handle, unsafe.Pointer)"},
	{FieldName: "maBPF2Reinit", Symbol: "ma_bpf2_reinit", Type: "func(*bpf2ConfigNative, *bpf2Handle) Result"},
	{FieldName: "maBPF2ProcessPCMFrames", Symbol: "ma_bpf2_process_pcm_frames", Type: "func(*bpf2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maBPF2GetLatency", Symbol: "ma_bpf2_get_latency", Type: "func(*bpf2Handle) uint32"},
	{FieldName: "maBPFInit", Symbol: "ma_bpf_init", Type: "func(*bpfConfigNative, unsafe.Pointer, *bpfHandle) Result"},
	{FieldName: "maBPFUninit", Symbol: "ma_bpf_uninit", Type: "func(*bpfHandle, unsafe.Pointer)"},
	{FieldName: "maBPFReinit", Symbol: "ma_bpf_reinit", Type: "func(*bpfConfigNative, *bpfHandle) Result"},
	{FieldName: "maBPFProcessPCMFrames", Symbol: "ma_bpf_process_pcm_frames", Type: "func(*bpfHandle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maBPFGetLatency", Symbol: "ma_bpf_get_latency", Type: "func(*bpfHandle) uint32"},
	{FieldName: "maNotch2Init", Symbol: "ma_notch2_init", Type: "func(*notch2ConfigNative, unsafe.Pointer, *notch2Handle) Result"},
	{FieldName: "maNotch2Uninit", Symbol: "ma_notch2_uninit", Type: "func(*notch2Handle, unsafe.Pointer)"},
	{FieldName: "maNotch2Reinit", Symbol: "ma_notch2_reinit", Type: "func(*notch2ConfigNative, *notch2Handle) Result"},
	{FieldName: "maNotch2ProcessPCMFrames", Symbol: "ma_notch2_process_pcm_frames", Type: "func(*notch2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maNotch2GetLatency", Symbol: "ma_notch2_get_latency", Type: "func(*notch2Handle) uint32"},
	{FieldName: "maPeak2Init", Symbol: "ma_peak2_init", Type: "func(*peak2ConfigNative, unsafe.Pointer, *peak2Handle) Result"},
	{FieldName: "maPeak2Uninit", Symbol: "ma_peak2_uninit", Type: "func(*peak2Handle, unsafe.Pointer)"},
	{FieldName: "maPeak2Reinit", Symbol: "ma_peak2_reinit", Type: "func(*peak2ConfigNative, *peak2Handle) Result"},
	{FieldName: "maPeak2ProcessPCMFrames", Symbol: "ma_peak2_process_pcm_frames", Type: "func(*peak2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maPeak2GetLatency", Symbol: "ma_peak2_get_latency", Type: "func(*peak2Handle) uint32"},
	{FieldName: "maLoShelf2Init", Symbol: "ma_loshelf2_init", Type: "func(*loshelf2ConfigNative, unsafe.Pointer, *loshelf2Handle) Result"},
	{FieldName: "maLoShelf2Uninit", Symbol: "ma_loshelf2_uninit", Type: "func(*loshelf2Handle, unsafe.Pointer)"},
	{FieldName: "maLoShelf2Reinit", Symbol: "ma_loshelf2_reinit", Type: "func(*loshelf2ConfigNative, *loshelf2Handle) Result"},
	{FieldName: "maLoShelf2ProcessPCMFrames", Symbol: "ma_loshelf2_process_pcm_frames", Type: "func(*loshelf2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maLoShelf2GetLatency", Symbol: "ma_loshelf2_get_latency", Type: "func(*loshelf2Handle) uint32"},
	{FieldName: "maHiShelf2Init", Symbol: "ma_hishelf2_init", Type: "func(*hishelf2ConfigNative, unsafe.Pointer, *hishelf2Handle) Result"},
	{FieldName: "maHiShelf2Uninit", Symbol: "ma_hishelf2_uninit", Type: "func(*hishelf2Handle, unsafe.Pointer)"},
	{FieldName: "maHiShelf2Reinit", Symbol: "ma_hishelf2_reinit", Type: "func(*hishelf2ConfigNative, *hishelf2Handle) Result"},
	{FieldName: "maHiShelf2ProcessPCMFrames", Symbol: "ma_hishelf2_process_pcm_frames", Type: "func(*hishelf2Handle, unsafe.Pointer, unsafe.Pointer, uint64) Result"},
	{FieldName: "maHiShelf2GetLatency", Symbol: "ma_hishelf2_get_latency", Type: "func(*hishelf2Handle) uint32"},
	{FieldName: "maDelayInit", Symbol: "ma_delay_init", Type: "func(*delayConfigNative, unsafe.Pointer, *delayHandle) Result"},
	{FieldName: "maDelayUninit", Symbol: "ma_delay_uninit", Type: "func(*delayHandle, unsafe.Pointer)"},
	{FieldName: "maDelayProcessPCMFrames", Symbol: "ma_delay_process_pcm_frames", Type: "func(*delayHandle, unsafe.Pointer, unsafe.Pointer, uint32) Result"},
	{FieldName: "maDelaySetWet", Symbol: "ma_delay_set_wet", Type: "func(*delayHandle, float32)"},
	{FieldName: "maDelayGetWet", Symbol: "ma_delay_get_wet", Type: "func(*delayHandle) float32"},
	{FieldName: "maDelaySetDry", Symbol: "ma_delay_set_dry", Type: "func(*delayHandle, float32)"},
	{FieldName: "maDelayGetDry", Symbol: "ma_delay_get_dry", Type: "func(*delayHandle) float32"},
	{FieldName: "maDelaySetDecay", Symbol: "ma_delay_set_decay", Type: "func(*delayHandle, float32)"},
	{FieldName: "maDelayGetDecay", Symbol: "ma_delay_get_decay", Type: "func(*delayHandle) float32"},
	{FieldName: "maDecoderInitMemory", Symbol: "ma_decoder_init_memory", Type: "func(unsafe.Pointer, uintptr, *decoderConfigNative, *decoderHandle) Result"},
	{FieldName: "maDecoderInitFile", Symbol: "ma_decoder_init_file", Type: "func(string, *decoderConfigNative, *decoderHandle) Result"},
	{FieldName: "maDecoderUninit", Symbol: "ma_decoder_uninit", Type: "func(*decoderHandle)"},
	{FieldName: "maDecoderReadPCMFrames", Symbol: "ma_decoder_read_pcm_frames", Type: "func(*decoderHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "maDecoderSeekToPCMFrame", Symbol: "ma_decoder_seek_to_pcm_frame", Type: "func(*decoderHandle, uint64) Result"},
	{FieldName: "maDecoderGetDataFormat", Symbol: "ma_decoder_get_data_format", Type: "func(*decoderHandle, *Format, *uint32, *uint32, *uint8, uintptr) Result"},
	{FieldName: "maDecoderGetCursorInPCMFrames", Symbol: "ma_decoder_get_cursor_in_pcm_frames", Type: "func(*decoderHandle, *uint64) Result"},
	{FieldName: "maDecoderGetLengthInPCMFrames", Symbol: "ma_decoder_get_length_in_pcm_frames", Type: "func(*decoderHandle, *uint64) Result"},
	{FieldName: "maDecoderGetAvailableFrames", Symbol: "ma_decoder_get_available_frames", Type: "func(*decoderHandle, *uint64) Result"},
	{FieldName: "maEncoderInitFile", Symbol: "ma_encoder_init_file", Type: "func(string, *encoderConfigNative, *encoderHandle) Result"},
	{FieldName: "magoEncoderInit", Symbol: "mago_encoder_init", Type: "func(*encoderHandle, *encoderConfigNative, uintptr, uintptr, uintptr, **encoderBridgeNative) Result"},
	{FieldName: "maEncoderUninit", Symbol: "ma_encoder_uninit", Type: "func(*encoderHandle)"},
	{FieldName: "maEncoderWritePCMFrames", Symbol: "ma_encoder_write_pcm_frames", Type: "func(*encoderHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "magoDataSourceInit", Symbol: "mago_data_source_init", Type: "func(uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, uintptr, **dataSourceHandle) Result"},
	{FieldName: "magoDataSourceUninit", Symbol: "mago_data_source_uninit", Type: "func(*dataSourceHandle)"},
	{FieldName: "maDataSourceReadPCMFrames", Symbol: "ma_data_source_read_pcm_frames", Type: "func(*dataSourceHandle, unsafe.Pointer, uint64, *uint64) Result"},
	{FieldName: "maDataSourceSeekToPCMFrame", Symbol: "ma_data_source_seek_to_pcm_frame", Type: "func(*dataSourceHandle, uint64) Result"},
	{FieldName: "maDataSourceGetDataFormat", Symbol: "ma_data_source_get_data_format", Type: "func(*dataSourceHandle, *Format, *uint32, *uint32, *uint8, uintptr) Result"},
	{FieldName: "maDataSourceGetCursorInPCMFrames", Symbol: "ma_data_source_get_cursor_in_pcm_frames", Type: "func(*dataSourceHandle, *uint64) Result"},
	{FieldName: "maDataSourceGetLengthInPCMFrames", Symbol: "ma_data_source_get_length_in_pcm_frames", Type: "func(*dataSourceHandle, *uint64) Result"},
	{FieldName: "maDataSourceSetLooping", Symbol: "ma_data_source_set_looping", Type: "func(*dataSourceHandle, uint32) Result"},
}

var baseConstants = []constSpec{
	{Name: "Success", Type: "Result", Value: "0"},
	{Name: "Error", Type: "Result", Value: "-1"},
	{Name: "InvalidArgs", Type: "Result", Value: "-2"},
	{Name: "InvalidOperation", Type: "Result", Value: "-3"},
	{Name: "OutOfMemory", Type: "Result", Value: "-4"},
	{Name: "NoBackend", Type: "Result", Value: "-203"},
	{Name: "NoDevice", Type: "Result", Value: "-204"},
	{Name: "InvalidDeviceConfig", Type: "Result", Value: "-206"},
	{Name: "DeviceNotInitialized", Type: "Result", Value: "-300"},
	{Name: "DeviceAlreadyInitialized", Type: "Result", Value: "-301"},
	{Name: "DeviceNotStarted", Type: "Result", Value: "-302"},
	{Name: "DeviceNotStopped", Type: "Result", Value: "-303"},
	{Name: "AtEnd", Type: "Result", Value: "-17"},
	{Name: "BackendWASAPI", Type: "Backend", Value: "0"},
	{Name: "BackendDSound", Type: "Backend", Value: "1"},
	{Name: "BackendWinMM", Type: "Backend", Value: "2"},
	{Name: "BackendCoreAudio", Type: "Backend", Value: "3"},
	{Name: "BackendSndIO", Type: "Backend", Value: "4"},
	{Name: "BackendAudio4", Type: "Backend", Value: "5"},
	{Name: "BackendOSS", Type: "Backend", Value: "6"},
	{Name: "BackendPulseAudio", Type: "Backend", Value: "7"},
	{Name: "BackendALSA", Type: "Backend", Value: "8"},
	{Name: "BackendJACK", Type: "Backend", Value: "9"},
	{Name: "BackendAAudio", Type: "Backend", Value: "10"},
	{Name: "BackendOpenSL", Type: "Backend", Value: "11"},
	{Name: "BackendWebAudio", Type: "Backend", Value: "12"},
	{Name: "BackendCustom", Type: "Backend", Value: "13"},
	{Name: "BackendNull", Type: "Backend", Value: "14"},
	{Name: "DeviceTypePlayback", Type: "DeviceType", Value: "1"},
	{Name: "DeviceTypeCapture", Type: "DeviceType", Value: "2"},
	{Name: "DeviceTypeDuplex", Type: "DeviceType", Value: "3"},
	{Name: "DeviceTypeLoopback", Type: "DeviceType", Value: "4"},
	{Name: "ShareModeShared", Type: "ShareMode", Value: "0"},
	{Name: "ShareModeExclusive", Type: "ShareMode", Value: "1"},
	{Name: "PerformanceProfileLowLatency", Type: "PerformanceProfile", Value: "0"},
	{Name: "PerformanceProfileConservative", Type: "PerformanceProfile", Value: "1"},
	{Name: "FormatUnknown", Type: "Format", Value: "0"},
	{Name: "FormatU8", Type: "Format", Value: "1"},
	{Name: "FormatS16", Type: "Format", Value: "2"},
	{Name: "FormatS24", Type: "Format", Value: "3"},
	{Name: "FormatS32", Type: "Format", Value: "4"},
	{Name: "FormatF32", Type: "Format", Value: "5"},
	{Name: "NotificationStarted", Type: "NotificationType", Value: "0"},
	{Name: "NotificationStopped", Type: "NotificationType", Value: "1"},
	{Name: "NotificationRerouted", Type: "NotificationType", Value: "2"},
	{Name: "NotificationInterruptionBegan", Type: "NotificationType", Value: "3"},
	{Name: "NotificationInterruptionEnded", Type: "NotificationType", Value: "4"},
	{Name: "NotificationUnlocked", Type: "NotificationType", Value: "5"},
	{Name: "WaveformTypeSine", Type: "WaveformType", Value: "0"},
	{Name: "WaveformTypeSquare", Type: "WaveformType", Value: "1"},
	{Name: "WaveformTypeTriangle", Type: "WaveformType", Value: "2"},
	{Name: "WaveformTypeSawtooth", Type: "WaveformType", Value: "3"},
	{Name: "NoiseTypeWhite", Type: "NoiseType", Value: "0"},
	{Name: "NoiseTypePink", Type: "NoiseType", Value: "1"},
	{Name: "NoiseTypeBrownian", Type: "NoiseType", Value: "2"},
}

func resolveVersion(root, versionFlag string) (major, minor, revision string, err error) {
	if strings.TrimSpace(versionFlag) != "" {
		parts := strings.Split(strings.TrimSpace(versionFlag), ".")
		if len(parts) != 3 {
			return "", "", "", fmt.Errorf("invalid version format %q, expected x.y.z", versionFlag)
		}
		return parts[0], parts[1], parts[2], nil
	}

	headerPath := filepath.Join(root, "miniaudio.h")
	if data, err := os.ReadFile(headerPath); err == nil {
		majorRe := regexp.MustCompile(`#define\s+MA_VERSION_MAJOR\s+(\d+)`)
		minorRe := regexp.MustCompile(`#define\s+MA_VERSION_MINOR\s+(\d+)`)
		revRe := regexp.MustCompile(`#define\s+MA_VERSION_REVISION\s+(\d+)`)

		majorMatch := majorRe.FindSubmatch(data)
		minorMatch := minorRe.FindSubmatch(data)
		revMatch := revRe.FindSubmatch(data)

		if len(majorMatch) > 1 && len(minorMatch) > 1 && len(revMatch) > 1 {
			return string(majorMatch[1]), string(minorMatch[1]), string(revMatch[1]), nil
		}
	}

	bindingsPath := filepath.Join(root, "zz_generated.bindings.go")
	if data, err := os.ReadFile(bindingsPath); err == nil {
		pattern := regexp.MustCompile(`ExpectedMiniaudioVersion(Major|Minor|Revision)\s+uint32\s*=\s*(\d+)`)
		parts := map[string]string{}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			parts[match[1]] = match[2]
		}
		if maj, ok1 := parts["Major"]; ok1 {
			if minorVal, ok2 := parts["Minor"]; ok2 {
				if rev, ok3 := parts["Revision"]; ok3 {
					return maj, minorVal, rev, nil
				}
			}
		}
	}

	return "0", "11", "25", nil
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	versionFlag := flag.String("version", "", "miniaudio version (e.g. 0.11.26)")
	flag.Parse()

	major, minor, revision, err := resolveVersion(root, *versionFlag)
	if err != nil {
		panic(err)
	}

	constants := append([]constSpec{
		{Name: "ExpectedMiniaudioVersionMajor", Type: "uint32", Value: major},
		{Name: "ExpectedMiniaudioVersionMinor", Type: "uint32", Value: minor},
		{Name: "ExpectedMiniaudioVersionRevision", Type: "uint32", Value: revision},
	}, baseConstants...)

	outPath := filepath.Join(root, "zz_generated.bindings.go")
	var buf bytes.Buffer

	buf.WriteString("// Code generated by go generate; DO NOT EDIT.\n")
	buf.WriteString("\n")
	buf.WriteString("package mago\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"unsafe\"\n\n")
	buf.WriteString("\t\"github.com/ebitengine/purego\"\n")
	buf.WriteString(")\n\n")

	buf.WriteString("const (\n")
	for _, c := range constants {
		fmt.Fprintf(&buf, "\t%s %s = %s\n", c.Name, c.Type, c.Value)
	}
	buf.WriteString(")\n\n")

	buf.WriteString("type bindingSet struct {\n")
	for _, fn := range functions {
		fmt.Fprintf(&buf, "\t%s %s\n", fn.FieldName, fn.Type)
	}
	buf.WriteString("}\n\n")

	buf.WriteString("func (b *bindingSet) register(handle uintptr) {\n")
	for _, fn := range functions {
		fmt.Fprintf(&buf, "\tpurego.RegisterLibFunc(&b.%s, handle, %q)\n", fn.FieldName, fn.Symbol)
	}
	buf.WriteString("}\n\n")
	buf.WriteString("var _ unsafe.Pointer\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		panic(fmt.Errorf("format generated bindings: %w\n%s", err, buf.String()))
	}

	if err := os.WriteFile(outPath, formatted, 0o600); err != nil {
		panic(err)
	}
}
