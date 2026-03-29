#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#define MA_DLL
#define MA_IMPLEMENTATION
#include "miniaudio.h"

#if defined(_WIN32)
#define MAGO_API __declspec(dllexport)
#elif defined(__GNUC__)
#define MAGO_API __attribute__((visibility("default")))
#else
#define MAGO_API
#endif

typedef void (*mago_data_callback)(uintptr_t userData, void* pOutput, const void* pInput, ma_uint32 frameCount);
typedef void (*mago_notification_callback)(uintptr_t userData, ma_uint32 notificationType);

typedef struct
{
    const ma_device_id* pDeviceID;
    ma_int32 deviceIndex;
    ma_format format;
    ma_uint32 channels;
    ma_uint32 sampleRate;
    ma_uint32 periodSizeInFrames;
    ma_uint32 periodSizeInMilliseconds;
    ma_uint32 periods;
    ma_performance_profile performanceProfile;
    ma_share_mode shareMode;
    ma_bool32 noPreSilencedOutputBuffer;
    ma_bool32 noClip;
    ma_bool32 noDisableDenormals;
    ma_bool32 noFixedSizedCallback;
    uintptr_t dataCallback;
    uintptr_t notificationCallback;
    uintptr_t userData;
} mago_playback_device_config;

typedef struct
{
    char name[MA_MAX_DEVICE_NAME_LENGTH + 1];
    ma_bool32 isDefault;
} mago_device_info;

typedef struct
{
    mago_data_callback dataCallback;
    mago_notification_callback notificationCallback;
    uintptr_t userData;
} mago_device_bridge;

static void mago_on_device_data(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount)
{
    mago_device_bridge* pBridge;

    if (pDevice == NULL) {
        return;
    }

    pBridge = (mago_device_bridge*)pDevice->pUserData;
    if (pBridge == NULL || pBridge->dataCallback == NULL) {
        return;
    }

    pBridge->dataCallback(pBridge->userData, pOutput, pInput, frameCount);
}

static void mago_on_device_notification(const ma_device_notification* pNotification)
{
    mago_device_bridge* pBridge;

    if (pNotification == NULL || pNotification->pDevice == NULL) {
        return;
    }

    pBridge = (mago_device_bridge*)pNotification->pDevice->pUserData;
    if (pBridge == NULL || pBridge->notificationCallback == NULL) {
        return;
    }

    pBridge->notificationCallback(pBridge->userData, (ma_uint32)pNotification->type);
}

MAGO_API ma_result mago_context_init_default(ma_context** ppContext)
{
    ma_context* pContext;
    ma_result result;

    if (ppContext == NULL) {
        return MA_INVALID_ARGS;
    }

    *ppContext = NULL;

    pContext = (ma_context*)calloc(1, sizeof(ma_context));
    if (pContext == NULL) {
        return MA_OUT_OF_MEMORY;
    }

    result = ma_context_init(NULL, 0, NULL, pContext);
    if (result != MA_SUCCESS) {
        free(pContext);
        return result;
    }

    *ppContext = pContext;
    return MA_SUCCESS;
}

MAGO_API ma_result mago_context_init_with_backends(const ma_backend* pBackends, ma_uint32 backendCount, ma_context** ppContext)
{
    ma_context* pContext;
    ma_result result;

    if (ppContext == NULL) {
        return MA_INVALID_ARGS;
    }

    *ppContext = NULL;

    pContext = (ma_context*)calloc(1, sizeof(ma_context));
    if (pContext == NULL) {
        return MA_OUT_OF_MEMORY;
    }

    result = ma_context_init(pBackends, backendCount, NULL, pContext);
    if (result != MA_SUCCESS) {
        free(pContext);
        return result;
    }

    *ppContext = pContext;
    return MA_SUCCESS;
}

MAGO_API void mago_context_uninit_free(ma_context* pContext)
{
    if (pContext == NULL) {
        return;
    }

    ma_context_uninit(pContext);
    free(pContext);
}

MAGO_API ma_result mago_context_get_devices(
    ma_context* pContext,
    mago_device_info** ppPlaybackInfos,
    ma_uint32* pPlaybackCount,
    mago_device_info** ppCaptureInfos,
    ma_uint32* pCaptureCount)
{
    ma_device_info* pPlaybackDeviceInfos;
    ma_device_info* pCaptureDeviceInfos;
    mago_device_info* pPlaybackInfos = NULL;
    mago_device_info* pCaptureInfos = NULL;
    ma_uint32 playbackCount;
    ma_uint32 captureCount;
    ma_uint32 i;
    ma_result result;

    if (pContext == NULL || ppPlaybackInfos == NULL || pPlaybackCount == NULL || ppCaptureInfos == NULL || pCaptureCount == NULL) {
        return MA_INVALID_ARGS;
    }

    *ppPlaybackInfos = NULL;
    *pPlaybackCount = 0;
    *ppCaptureInfos = NULL;
    *pCaptureCount = 0;

    result = ma_context_get_devices(pContext, &pPlaybackDeviceInfos, &playbackCount, &pCaptureDeviceInfos, &captureCount);
    if (result != MA_SUCCESS) {
        return result;
    }

    if (playbackCount > 0) {
        pPlaybackInfos = (mago_device_info*)calloc(playbackCount, sizeof(mago_device_info));
        if (pPlaybackInfos == NULL) {
            return MA_OUT_OF_MEMORY;
        }

        for (i = 0; i < playbackCount; ++i) {
            memcpy(pPlaybackInfos[i].name, pPlaybackDeviceInfos[i].name, sizeof(pPlaybackInfos[i].name));
            pPlaybackInfos[i].isDefault = pPlaybackDeviceInfos[i].isDefault;
        }
    }

    if (captureCount > 0) {
        pCaptureInfos = (mago_device_info*)calloc(captureCount, sizeof(mago_device_info));
        if (pCaptureInfos == NULL) {
            free(pPlaybackInfos);
            return MA_OUT_OF_MEMORY;
        }

        for (i = 0; i < captureCount; ++i) {
            memcpy(pCaptureInfos[i].name, pCaptureDeviceInfos[i].name, sizeof(pCaptureInfos[i].name));
            pCaptureInfos[i].isDefault = pCaptureDeviceInfos[i].isDefault;
        }
    }

    *ppPlaybackInfos = pPlaybackInfos;
    *pPlaybackCount = playbackCount;
    *ppCaptureInfos = pCaptureInfos;
    *pCaptureCount = captureCount;
    return MA_SUCCESS;
}

MAGO_API void mago_context_free_device_infos(mago_device_info* pInfos)
{
    free(pInfos);
}

MAGO_API ma_result mago_device_init_playback(
    ma_context* pContext,
    const mago_playback_device_config* pMagoConfig,
    ma_device** ppDevice)
{
    ma_device_config config;
    ma_device* pDevice;
    mago_device_bridge* pBridge;
    const ma_device_id* pDeviceID;
    ma_result result;

    if (pMagoConfig == NULL || ppDevice == NULL) {
        return MA_INVALID_ARGS;
    }

    *ppDevice = NULL;

    pDevice = (ma_device*)calloc(1, sizeof(ma_device));
    if (pDevice == NULL) {
        return MA_OUT_OF_MEMORY;
    }

    pBridge = (mago_device_bridge*)calloc(1, sizeof(mago_device_bridge));
    if (pBridge == NULL) {
        free(pDevice);
        return MA_OUT_OF_MEMORY;
    }

    pBridge->dataCallback = (mago_data_callback)pMagoConfig->dataCallback;
    pBridge->notificationCallback = (mago_notification_callback)pMagoConfig->notificationCallback;
    pBridge->userData = pMagoConfig->userData;
    pDeviceID = pMagoConfig->pDeviceID;

    if (pMagoConfig->deviceIndex >= 0) {
        ma_device_info* pPlaybackDeviceInfos;
        ma_uint32 playbackDeviceCount;

        if (pContext == NULL) {
            free(pBridge);
            free(pDevice);
            return MA_INVALID_ARGS;
        }

        result = ma_context_get_devices(pContext, &pPlaybackDeviceInfos, &playbackDeviceCount, NULL, NULL);
        if (result != MA_SUCCESS) {
            free(pBridge);
            free(pDevice);
            return result;
        }

        if ((ma_uint32)pMagoConfig->deviceIndex >= playbackDeviceCount) {
            free(pBridge);
            free(pDevice);
            return MA_NO_DEVICE;
        }

        pDeviceID = &pPlaybackDeviceInfos[pMagoConfig->deviceIndex].id;
    }

    config = ma_device_config_init(ma_device_type_playback);
    config.playback.pDeviceID = pDeviceID;
    config.playback.format = pMagoConfig->format;
    config.playback.channels = pMagoConfig->channels;
    config.playback.shareMode = pMagoConfig->shareMode;
    config.sampleRate = pMagoConfig->sampleRate;
    config.periodSizeInFrames = pMagoConfig->periodSizeInFrames;
    config.periodSizeInMilliseconds = pMagoConfig->periodSizeInMilliseconds;
    config.periods = pMagoConfig->periods;
    config.performanceProfile = pMagoConfig->performanceProfile;
    config.noPreSilencedOutputBuffer = (ma_bool8)pMagoConfig->noPreSilencedOutputBuffer;
    config.noClip = (ma_bool8)pMagoConfig->noClip;
    config.noDisableDenormals = (ma_bool8)pMagoConfig->noDisableDenormals;
    config.noFixedSizedCallback = (ma_bool8)pMagoConfig->noFixedSizedCallback;
    config.dataCallback = pBridge->dataCallback != NULL ? mago_on_device_data : NULL;
    config.notificationCallback = pBridge->notificationCallback != NULL ? mago_on_device_notification : NULL;
    config.pUserData = pBridge;

    result = ma_device_init(pContext, &config, pDevice);
    if (result != MA_SUCCESS) {
        free(pBridge);
        free(pDevice);
        return result;
    }

    *ppDevice = pDevice;
    return MA_SUCCESS;
}

MAGO_API void mago_device_uninit_free(ma_device* pDevice)
{
    mago_device_bridge* pBridge;

    if (pDevice == NULL) {
        return;
    }

    pBridge = (mago_device_bridge*)pDevice->pUserData;
    ma_device_uninit(pDevice);
    free(pBridge);
    free(pDevice);
}
