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
    return 0;
}
