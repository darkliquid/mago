/*
 * mago native bridge.
 *
 * This file is deliberately tiny. mago is a zero-CGO project: the Go side loads
 * this shared library with purego and binds exported `ma_*` symbols directly.
 * C is only allowed for the three things purego cannot do:
 *
 *   1. Raw allocation (mago_alloc/mago_free). Go cannot call malloc without
 *      cgo, and the sizes of miniaudio's objects are not available to Go.
 *   2. Building a by-value, large config struct. ma_device_config_init returns
 *      ma_device_config by value and that struct is large, backend-specific and
 *      version-unstable, so Go must not mirror it.
 *   3. Callback trampolines for C signatures that carry no user data. The device
 *      data and notification callbacks receive no `pUserData`; user data travels
 *      through `pDevice->pUserData`, which Go cannot read without mirroring the
 *      enormous ma_device struct.
 *
 * Everything else (context init/uninit, device enumeration, logging) is bound
 * directly from Go against the exported `ma_*` symbols. Do not add wrappers for
 * anything that already has a public miniaudio function.
 */

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

/* -------------------------------------------------------------------------
 * 1. Raw allocation
 * The enum is a private ABI shared with the Go constants in types.go. Keep the
 * two in sync when adding a new object type.
 * ---------------------------------------------------------------------- */

enum mago_object_type
{
    MAGO_OBJECT_CONTEXT = 1,
    MAGO_OBJECT_DEVICE  = 2,
    MAGO_OBJECT_LOG     = 3
};

MAGO_API void* mago_alloc(int type)
{
    switch (type)
    {
        case MAGO_OBJECT_CONTEXT: return calloc(1, sizeof(ma_context));
        case MAGO_OBJECT_DEVICE:  return calloc(1, sizeof(ma_device));
        case MAGO_OBJECT_LOG:     return calloc(1, sizeof(ma_log));
        default:                  return NULL;
    }
}

MAGO_API void mago_free(void* p)
{
    free(p);
}

/* -------------------------------------------------------------------------
 * 2. Device config construction and 3. callback trampolines
 * ---------------------------------------------------------------------- */

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
    mago_data_callback dataCallback;
    mago_notification_callback notificationCallback;
    uintptr_t userData;
} mago_device_bridge;

static void mago_on_device_data(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount)
{
    mago_device_bridge* pBridge;

    if (pDevice == NULL)
    {
        return;
    }

    pBridge = (mago_device_bridge*)pDevice->pUserData;
    if (pBridge == NULL || pBridge->dataCallback == NULL)
    {
        return;
    }

    pBridge->dataCallback(pBridge->userData, pOutput, pInput, frameCount);
}

static void mago_on_device_notification(const ma_device_notification* pNotification)
{
    mago_device_bridge* pBridge;

    if (pNotification == NULL || pNotification->pDevice == NULL)
    {
        return;
    }

    pBridge = (mago_device_bridge*)pNotification->pDevice->pUserData;
    if (pBridge == NULL || pBridge->notificationCallback == NULL)
    {
        return;
    }

    pBridge->notificationCallback(pBridge->userData, (ma_uint32)pNotification->type);
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

    if (pMagoConfig == NULL || ppDevice == NULL)
    {
        return MA_INVALID_ARGS;
    }

    *ppDevice = NULL;

    pDevice = (ma_device*)calloc(1, sizeof(ma_device));
    if (pDevice == NULL)
    {
        return MA_OUT_OF_MEMORY;
    }

    pBridge = (mago_device_bridge*)calloc(1, sizeof(mago_device_bridge));
    if (pBridge == NULL)
    {
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

    if (pDevice == NULL)
    {
        return;
    }

    pBridge = (mago_device_bridge*)pDevice->pUserData;
    ma_device_uninit(pDevice);
    free(pBridge);
    free(pDevice);
}

/* -------------------------------------------------------------------------
 * Logging callback registration.
 * ma_log_callback is returned and passed by value, so this is category 2.
 * userData is uintptr_t so Go can pass its token without an
 * unsafe.Pointer(uintptr) conversion.
 * ---------------------------------------------------------------------- */

MAGO_API ma_result mago_log_register_callback(ma_log* pLog, uintptr_t onLog, uintptr_t userData)
{
    ma_log_callback callback = ma_log_callback_init((ma_log_callback_proc)onLog, (void*)userData);
    return ma_log_register_callback(pLog, callback);
}

MAGO_API ma_result mago_log_unregister_callback(ma_log* pLog, uintptr_t onLog, uintptr_t userData)
{
    ma_log_callback callback = ma_log_callback_init((ma_log_callback_proc)onLog, (void*)userData);
    return ma_log_unregister_callback(pLog, callback);
}
