#include <stddef.h>
#include <stdio.h>

#include "miniaudio.h"

int main(void)
{
    printf("sizeof:ma_device_id %zu\n", sizeof(ma_device_id));
    printf("sizeof:ma_device_info %zu\n", sizeof(ma_device_info));
    printf("offsetof:ma_device_info.name %zu\n", offsetof(ma_device_info, name));
    printf("offsetof:ma_device_info.isDefault %zu\n", offsetof(ma_device_info, isDefault));
    printf("sizeof:ma_device_config %zu\n", sizeof(ma_device_config));
    printf("sizeof:ma_log %zu\n", sizeof(ma_log));
    printf("sizeof:ma_channel %zu\n", sizeof(ma_channel));
    printf("sizeof:ma_channel_converter_config %zu\n", sizeof(ma_channel_converter_config));
    printf("offsetof:ma_channel_converter_config.pChannelMapIn %zu\n", offsetof(ma_channel_converter_config, pChannelMapIn));
    printf("offsetof:ma_channel_converter_config.pChannelMapOut %zu\n", offsetof(ma_channel_converter_config, pChannelMapOut));
    printf("offsetof:ma_channel_converter_config.mixingMode %zu\n", offsetof(ma_channel_converter_config, mixingMode));
    printf("offsetof:ma_channel_converter_config.calculateLFEFromSpatialChannels %zu\n", offsetof(ma_channel_converter_config, calculateLFEFromSpatialChannels));
    printf("offsetof:ma_channel_converter_config.ppWeights %zu\n", offsetof(ma_channel_converter_config, ppWeights));
    printf("sizeof:ma_resampler_config %zu\n", sizeof(ma_resampler_config));
    printf("offsetof:ma_resampler_config.algorithm %zu\n", offsetof(ma_resampler_config, algorithm));
    printf("offsetof:ma_resampler_config.linear.lpfOrder %zu\n", offsetof(ma_resampler_config, linear.lpfOrder));
    printf("sizeof:ma_linear_resampler_config %zu\n", sizeof(ma_linear_resampler_config));
    printf("offsetof:ma_linear_resampler_config.lpfOrder %zu\n", offsetof(ma_linear_resampler_config, lpfOrder));
    printf("offsetof:ma_linear_resampler_config.lpfNyquistFactor %zu\n", offsetof(ma_linear_resampler_config, lpfNyquistFactor));
    printf("sizeof:ma_data_converter_config %zu\n", sizeof(ma_data_converter_config));
    printf("offsetof:ma_data_converter_config.pChannelMapIn %zu\n", offsetof(ma_data_converter_config, pChannelMapIn));
    printf("offsetof:ma_data_converter_config.ditherMode %zu\n", offsetof(ma_data_converter_config, ditherMode));
    printf("offsetof:ma_data_converter_config.channelMixMode %zu\n", offsetof(ma_data_converter_config, channelMixMode));
    printf("offsetof:ma_data_converter_config.calculateLFEFromSpatialChannels %zu\n", offsetof(ma_data_converter_config, calculateLFEFromSpatialChannels));
    printf("offsetof:ma_data_converter_config.ppChannelWeights %zu\n", offsetof(ma_data_converter_config, ppChannelWeights));
    printf("offsetof:ma_data_converter_config.allowDynamicSampleRate %zu\n", offsetof(ma_data_converter_config, allowDynamicSampleRate));
    printf("offsetof:ma_data_converter_config.resampling %zu\n", offsetof(ma_data_converter_config, resampling));
    printf("sizeof:ma_audio_buffer_config %zu\n", sizeof(ma_audio_buffer_config));
    printf("offsetof:ma_audio_buffer_config.sizeInFrames %zu\n", offsetof(ma_audio_buffer_config, sizeInFrames));
    printf("offsetof:ma_audio_buffer_config.pData %zu\n", offsetof(ma_audio_buffer_config, pData));
    printf("offsetof:ma_audio_buffer_config.allocationCallbacks %zu\n", offsetof(ma_audio_buffer_config, allocationCallbacks));
    printf("sizeof:ma_allocation_callbacks %zu\n", sizeof(ma_allocation_callbacks));
    printf("sizeof:ma_waveform_config %zu\n", sizeof(ma_waveform_config));
    printf("offsetof:ma_waveform_config.format %zu\n", offsetof(ma_waveform_config, format));
    printf("offsetof:ma_waveform_config.channels %zu\n", offsetof(ma_waveform_config, channels));
    printf("offsetof:ma_waveform_config.sampleRate %zu\n", offsetof(ma_waveform_config, sampleRate));
    printf("offsetof:ma_waveform_config.type %zu\n", offsetof(ma_waveform_config, type));
    printf("offsetof:ma_waveform_config.amplitude %zu\n", offsetof(ma_waveform_config, amplitude));
    printf("offsetof:ma_waveform_config.frequency %zu\n", offsetof(ma_waveform_config, frequency));
    printf("sizeof:ma_noise_config %zu\n", sizeof(ma_noise_config));
    printf("offsetof:ma_noise_config.format %zu\n", offsetof(ma_noise_config, format));
    printf("offsetof:ma_noise_config.channels %zu\n", offsetof(ma_noise_config, channels));
    printf("offsetof:ma_noise_config.type %zu\n", offsetof(ma_noise_config, type));
    printf("offsetof:ma_noise_config.seed %zu\n", offsetof(ma_noise_config, seed));
    printf("offsetof:ma_noise_config.amplitude %zu\n", offsetof(ma_noise_config, amplitude));
    printf("offsetof:ma_noise_config.duplicateChannels %zu\n", offsetof(ma_noise_config, duplicateChannels));
    printf("sizeof:ma_waveform %zu\n", sizeof(ma_waveform));
    printf("sizeof:ma_noise %zu\n", sizeof(ma_noise));

    printf("sizeof:ma_biquad_config %zu\n", sizeof(ma_biquad_config));
    printf("offsetof:ma_biquad_config.format %zu\n", offsetof(ma_biquad_config, format));
    printf("offsetof:ma_biquad_config.channels %zu\n", offsetof(ma_biquad_config, channels));
    printf("offsetof:ma_biquad_config.b0 %zu\n", offsetof(ma_biquad_config, b0));
    printf("offsetof:ma_biquad_config.b1 %zu\n", offsetof(ma_biquad_config, b1));
    printf("offsetof:ma_biquad_config.b2 %zu\n", offsetof(ma_biquad_config, b2));
    printf("offsetof:ma_biquad_config.a0 %zu\n", offsetof(ma_biquad_config, a0));
    printf("offsetof:ma_biquad_config.a1 %zu\n", offsetof(ma_biquad_config, a1));
    printf("offsetof:ma_biquad_config.a2 %zu\n", offsetof(ma_biquad_config, a2));

    printf("sizeof:ma_lpf1_config %zu\n", sizeof(ma_lpf1_config));
    printf("offsetof:ma_lpf1_config.format %zu\n", offsetof(ma_lpf1_config, format));
    printf("offsetof:ma_lpf1_config.channels %zu\n", offsetof(ma_lpf1_config, channels));
    printf("offsetof:ma_lpf1_config.sampleRate %zu\n", offsetof(ma_lpf1_config, sampleRate));
    printf("offsetof:ma_lpf1_config.cutoffFrequency %zu\n", offsetof(ma_lpf1_config, cutoffFrequency));
    printf("offsetof:ma_lpf1_config.q %zu\n", offsetof(ma_lpf1_config, q));

    printf("sizeof:ma_lpf_config %zu\n", sizeof(ma_lpf_config));
    printf("offsetof:ma_lpf_config.format %zu\n", offsetof(ma_lpf_config, format));
    printf("offsetof:ma_lpf_config.channels %zu\n", offsetof(ma_lpf_config, channels));
    printf("offsetof:ma_lpf_config.sampleRate %zu\n", offsetof(ma_lpf_config, sampleRate));
    printf("offsetof:ma_lpf_config.cutoffFrequency %zu\n", offsetof(ma_lpf_config, cutoffFrequency));
    printf("offsetof:ma_lpf_config.order %zu\n", offsetof(ma_lpf_config, order));

    printf("sizeof:ma_notch2_config %zu\n", sizeof(ma_notch2_config));
    printf("offsetof:ma_notch2_config.format %zu\n", offsetof(ma_notch2_config, format));
    printf("offsetof:ma_notch2_config.channels %zu\n", offsetof(ma_notch2_config, channels));
    printf("offsetof:ma_notch2_config.sampleRate %zu\n", offsetof(ma_notch2_config, sampleRate));
    printf("offsetof:ma_notch2_config.q %zu\n", offsetof(ma_notch2_config, q));
    printf("offsetof:ma_notch2_config.frequency %zu\n", offsetof(ma_notch2_config, frequency));

    printf("sizeof:ma_peak2_config %zu\n", sizeof(ma_peak2_config));
    printf("offsetof:ma_peak2_config.format %zu\n", offsetof(ma_peak2_config, format));
    printf("offsetof:ma_peak2_config.channels %zu\n", offsetof(ma_peak2_config, channels));
    printf("offsetof:ma_peak2_config.sampleRate %zu\n", offsetof(ma_peak2_config, sampleRate));
    printf("offsetof:ma_peak2_config.gainDB %zu\n", offsetof(ma_peak2_config, gainDB));
    printf("offsetof:ma_peak2_config.q %zu\n", offsetof(ma_peak2_config, q));
    printf("offsetof:ma_peak2_config.frequency %zu\n", offsetof(ma_peak2_config, frequency));

    printf("sizeof:ma_loshelf2_config %zu\n", sizeof(ma_loshelf2_config));
    printf("offsetof:ma_loshelf2_config.format %zu\n", offsetof(ma_loshelf2_config, format));
    printf("offsetof:ma_loshelf2_config.channels %zu\n", offsetof(ma_loshelf2_config, channels));
    printf("offsetof:ma_loshelf2_config.sampleRate %zu\n", offsetof(ma_loshelf2_config, sampleRate));
    printf("offsetof:ma_loshelf2_config.gainDB %zu\n", offsetof(ma_loshelf2_config, gainDB));
    printf("offsetof:ma_loshelf2_config.shelfSlope %zu\n", offsetof(ma_loshelf2_config, shelfSlope));
    printf("offsetof:ma_loshelf2_config.frequency %zu\n", offsetof(ma_loshelf2_config, frequency));

    printf("sizeof:ma_delay_config %zu\n", sizeof(ma_delay_config));
    printf("offsetof:ma_delay_config.channels %zu\n", offsetof(ma_delay_config, channels));
    printf("offsetof:ma_delay_config.sampleRate %zu\n", offsetof(ma_delay_config, sampleRate));
    printf("offsetof:ma_delay_config.delayInFrames %zu\n", offsetof(ma_delay_config, delayInFrames));
    printf("offsetof:ma_delay_config.delayStart %zu\n", offsetof(ma_delay_config, delayStart));
    printf("offsetof:ma_delay_config.wet %zu\n", offsetof(ma_delay_config, wet));
    printf("offsetof:ma_delay_config.dry %zu\n", offsetof(ma_delay_config, dry));
    printf("offsetof:ma_delay_config.decay %zu\n", offsetof(ma_delay_config, decay));

    printf("sizeof:ma_biquad %zu\n", sizeof(ma_biquad));
    printf("sizeof:ma_lpf1 %zu\n", sizeof(ma_lpf1));
    printf("sizeof:ma_lpf2 %zu\n", sizeof(ma_lpf2));
    printf("sizeof:ma_lpf %zu\n", sizeof(ma_lpf));
    printf("sizeof:ma_hpf1 %zu\n", sizeof(ma_hpf1));
    printf("sizeof:ma_hpf2 %zu\n", sizeof(ma_hpf2));
    printf("sizeof:ma_hpf %zu\n", sizeof(ma_hpf));
    printf("sizeof:ma_bpf2 %zu\n", sizeof(ma_bpf2));
    printf("sizeof:ma_bpf %zu\n", sizeof(ma_bpf));
    printf("sizeof:ma_notch2 %zu\n", sizeof(ma_notch2));
    printf("sizeof:ma_peak2 %zu\n", sizeof(ma_peak2));
    printf("sizeof:ma_loshelf2 %zu\n", sizeof(ma_loshelf2));
    printf("sizeof:ma_hishelf2 %zu\n", sizeof(ma_hishelf2));
    printf("sizeof:ma_delay %zu\n", sizeof(ma_delay));

    printf("sizeof:ma_decoder_config %zu\n", sizeof(ma_decoder_config));
    printf("offsetof:ma_decoder_config.pChannelMap %zu\n", offsetof(ma_decoder_config, pChannelMap));
    printf("offsetof:ma_decoder_config.channelMixMode %zu\n", offsetof(ma_decoder_config, channelMixMode));
    printf("offsetof:ma_decoder_config.ditherMode %zu\n", offsetof(ma_decoder_config, ditherMode));
    printf("offsetof:ma_decoder_config.resampling %zu\n", offsetof(ma_decoder_config, resampling));
    printf("offsetof:ma_decoder_config.allocationCallbacks %zu\n", offsetof(ma_decoder_config, allocationCallbacks));
    printf("offsetof:ma_decoder_config.encodingFormat %zu\n", offsetof(ma_decoder_config, encodingFormat));
    printf("offsetof:ma_decoder_config.seekPointCount %zu\n", offsetof(ma_decoder_config, seekPointCount));
    printf("offsetof:ma_decoder_config.ppCustomBackendVTables %zu\n", offsetof(ma_decoder_config, ppCustomBackendVTables));
    printf("offsetof:ma_decoder_config.customBackendCount %zu\n", offsetof(ma_decoder_config, customBackendCount));
    printf("offsetof:ma_decoder_config.pCustomBackendUserData %zu\n", offsetof(ma_decoder_config, pCustomBackendUserData));

    printf("sizeof:ma_encoder_config %zu\n", sizeof(ma_encoder_config));
    printf("offsetof:ma_encoder_config.encodingFormat %zu\n", offsetof(ma_encoder_config, encodingFormat));
    printf("offsetof:ma_encoder_config.format %zu\n", offsetof(ma_encoder_config, format));
    printf("offsetof:ma_encoder_config.channels %zu\n", offsetof(ma_encoder_config, channels));
    printf("offsetof:ma_encoder_config.sampleRate %zu\n", offsetof(ma_encoder_config, sampleRate));
    printf("offsetof:ma_encoder_config.allocationCallbacks %zu\n", offsetof(ma_encoder_config, allocationCallbacks));

    printf("sizeof:ma_node_config %zu\n", sizeof(ma_node_config));
    printf("offsetof:ma_node_config.vtable %zu\n", offsetof(ma_node_config, vtable));
    printf("offsetof:ma_node_config.initialState %zu\n", offsetof(ma_node_config, initialState));
    printf("offsetof:ma_node_config.inputBusCount %zu\n", offsetof(ma_node_config, inputBusCount));
    printf("offsetof:ma_node_config.outputBusCount %zu\n", offsetof(ma_node_config, outputBusCount));
    printf("offsetof:ma_node_config.pInputChannels %zu\n", offsetof(ma_node_config, pInputChannels));
    printf("offsetof:ma_node_config.pOutputChannels %zu\n", offsetof(ma_node_config, pOutputChannels));

    printf("sizeof:ma_node_graph_config %zu\n", sizeof(ma_node_graph_config));
    printf("offsetof:ma_node_graph_config.channels %zu\n", offsetof(ma_node_graph_config, channels));
    printf("offsetof:ma_node_graph_config.processingSizeInFrames %zu\n", offsetof(ma_node_graph_config, processingSizeInFrames));
    printf("offsetof:ma_node_graph_config.preMixStackSizeInBytes %zu\n", offsetof(ma_node_graph_config, preMixStackSizeInBytes));

    printf("sizeof:ma_data_source_node_config %zu\n", sizeof(ma_data_source_node_config));
    printf("offsetof:ma_data_source_node_config.pDataSource %zu\n", offsetof(ma_data_source_node_config, pDataSource));

    printf("sizeof:ma_splitter_node_config %zu\n", sizeof(ma_splitter_node_config));
    printf("offsetof:ma_splitter_node_config.channels %zu\n", offsetof(ma_splitter_node_config, channels));
    printf("offsetof:ma_splitter_node_config.outputBusCount %zu\n", offsetof(ma_splitter_node_config, outputBusCount));

    printf("sizeof:ma_biquad_node_config %zu\n", sizeof(ma_biquad_node_config));
    printf("offsetof:ma_biquad_node_config.biquad %zu\n", offsetof(ma_biquad_node_config, biquad));

    printf("sizeof:ma_lpf_node_config %zu\n", sizeof(ma_lpf_node_config));
    printf("offsetof:ma_lpf_node_config.lpf %zu\n", offsetof(ma_lpf_node_config, lpf));
    printf("sizeof:ma_hpf_node_config %zu\n", sizeof(ma_hpf_node_config));
    printf("offsetof:ma_hpf_node_config.hpf %zu\n", offsetof(ma_hpf_node_config, hpf));
    printf("sizeof:ma_bpf_node_config %zu\n", sizeof(ma_bpf_node_config));
    printf("offsetof:ma_bpf_node_config.bpf %zu\n", offsetof(ma_bpf_node_config, bpf));

    printf("sizeof:ma_notch_node_config %zu\n", sizeof(ma_notch_node_config));
    printf("offsetof:ma_notch_node_config.notch %zu\n", offsetof(ma_notch_node_config, notch));
    printf("sizeof:ma_peak_node_config %zu\n", sizeof(ma_peak_node_config));
    printf("offsetof:ma_peak_node_config.peak %zu\n", offsetof(ma_peak_node_config, peak));
    printf("sizeof:ma_loshelf_node_config %zu\n", sizeof(ma_loshelf_node_config));
    printf("offsetof:ma_loshelf_node_config.loshelf %zu\n", offsetof(ma_loshelf_node_config, loshelf));
    printf("sizeof:ma_hishelf_node_config %zu\n", sizeof(ma_hishelf_node_config));
    printf("offsetof:ma_hishelf_node_config.hishelf %zu\n", offsetof(ma_hishelf_node_config, hishelf));

    printf("sizeof:ma_delay_node_config %zu\n", sizeof(ma_delay_node_config));
    printf("offsetof:ma_delay_node_config.delay %zu\n", offsetof(ma_delay_node_config, delay));

    printf("sizeof:ma_node_graph %zu\n", sizeof(ma_node_graph));
    printf("sizeof:ma_data_source_node %zu\n", sizeof(ma_data_source_node));
    printf("sizeof:ma_splitter_node %zu\n", sizeof(ma_splitter_node));
    printf("sizeof:ma_biquad_node %zu\n", sizeof(ma_biquad_node));
    printf("sizeof:ma_lpf_node %zu\n", sizeof(ma_lpf_node));
    printf("sizeof:ma_hpf_node %zu\n", sizeof(ma_hpf_node));
    printf("sizeof:ma_bpf_node %zu\n", sizeof(ma_bpf_node));
    printf("sizeof:ma_notch_node %zu\n", sizeof(ma_notch_node));
    printf("sizeof:ma_peak_node %zu\n", sizeof(ma_peak_node));
    printf("sizeof:ma_loshelf_node %zu\n", sizeof(ma_loshelf_node));
    printf("sizeof:ma_hishelf_node %zu\n", sizeof(ma_hishelf_node));
    printf("sizeof:ma_delay_node %zu\n", sizeof(ma_delay_node));

    printf("sizeof:ma_engine_config %zu\n", sizeof(ma_engine_config));
    printf("offsetof:ma_engine_config.pResourceManager %zu\n", offsetof(ma_engine_config, pResourceManager));
    printf("offsetof:ma_engine_config.pContext %zu\n", offsetof(ma_engine_config, pContext));
    printf("offsetof:ma_engine_config.pDevice %zu\n", offsetof(ma_engine_config, pDevice));
    printf("offsetof:ma_engine_config.pPlaybackDeviceID %zu\n", offsetof(ma_engine_config, pPlaybackDeviceID));
    printf("offsetof:ma_engine_config.dataCallback %zu\n", offsetof(ma_engine_config, dataCallback));
    printf("offsetof:ma_engine_config.notificationCallback %zu\n", offsetof(ma_engine_config, notificationCallback));
    printf("offsetof:ma_engine_config.pLog %zu\n", offsetof(ma_engine_config, pLog));
    printf("offsetof:ma_engine_config.listenerCount %zu\n", offsetof(ma_engine_config, listenerCount));
    printf("offsetof:ma_engine_config.channels %zu\n", offsetof(ma_engine_config, channels));
    printf("offsetof:ma_engine_config.sampleRate %zu\n", offsetof(ma_engine_config, sampleRate));
    printf("offsetof:ma_engine_config.periodSizeInFrames %zu\n", offsetof(ma_engine_config, periodSizeInFrames));
    printf("offsetof:ma_engine_config.periodSizeInMilliseconds %zu\n", offsetof(ma_engine_config, periodSizeInMilliseconds));
    printf("offsetof:ma_engine_config.gainSmoothTimeInFrames %zu\n", offsetof(ma_engine_config, gainSmoothTimeInFrames));
    printf("offsetof:ma_engine_config.gainSmoothTimeInMilliseconds %zu\n", offsetof(ma_engine_config, gainSmoothTimeInMilliseconds));
    printf("offsetof:ma_engine_config.defaultVolumeSmoothTimeInPCMFrames %zu\n", offsetof(ma_engine_config, defaultVolumeSmoothTimeInPCMFrames));
    printf("offsetof:ma_engine_config.preMixStackSizeInBytes %zu\n", offsetof(ma_engine_config, preMixStackSizeInBytes));
    printf("offsetof:ma_engine_config.allocationCallbacks %zu\n", offsetof(ma_engine_config, allocationCallbacks));
    printf("offsetof:ma_engine_config.noAutoStart %zu\n", offsetof(ma_engine_config, noAutoStart));
    printf("offsetof:ma_engine_config.noDevice %zu\n", offsetof(ma_engine_config, noDevice));
    printf("offsetof:ma_engine_config.monoExpansionMode %zu\n", offsetof(ma_engine_config, monoExpansionMode));
    printf("offsetof:ma_engine_config.pResourceManagerVFS %zu\n", offsetof(ma_engine_config, pResourceManagerVFS));
    printf("offsetof:ma_engine_config.onProcess %zu\n", offsetof(ma_engine_config, onProcess));
    printf("offsetof:ma_engine_config.pProcessUserData %zu\n", offsetof(ma_engine_config, pProcessUserData));
    printf("offsetof:ma_engine_config.resourceManagerResampling %zu\n", offsetof(ma_engine_config, resourceManagerResampling));
    printf("offsetof:ma_engine_config.pitchResampling %zu\n", offsetof(ma_engine_config, pitchResampling));

    printf("sizeof:ma_engine %zu\n", sizeof(ma_engine));
    printf("sizeof:ma_sound %zu\n", sizeof(ma_sound));
    printf("sizeof:ma_vec3f %zu\n", sizeof(ma_vec3f));
    return 0;
}
