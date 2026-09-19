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
    return 0;
}
