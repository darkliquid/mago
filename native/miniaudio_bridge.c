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
 *      version-unstable, so Go must not mirror it. mago_device_init generalizes
 *      this across all four device types (playback, capture, duplex, loopback),
 *      and the mago_context_config_* helpers do the same for ma_context_config.
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

#ifndef NDEBUG
#define NDEBUG
#endif

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
    MAGO_OBJECT_CONTEXT           = 1,
    MAGO_OBJECT_DEVICE            = 2,
    MAGO_OBJECT_LOG               = 3,
    MAGO_OBJECT_DEVICE_INFO       = 4,
    MAGO_OBJECT_CONTEXT_CONFIG    = 5,
    MAGO_OBJECT_CHANNEL_CONVERTER = 6,
    MAGO_OBJECT_RESAMPLER         = 7,
    MAGO_OBJECT_LINEAR_RESAMPLER  = 8,
    MAGO_OBJECT_DATA_CONVERTER    = 9,
    MAGO_OBJECT_AUDIO_BUFFER      = 10,
    MAGO_OBJECT_AUDIO_BUFFER_REF  = 11,
    MAGO_OBJECT_RB                = 12,
    MAGO_OBJECT_PCM_RB            = 13,
    MAGO_OBJECT_WAVEFORM          = 14,
    MAGO_OBJECT_NOISE             = 15,
    MAGO_OBJECT_BIQUAD            = 16,
    MAGO_OBJECT_LPF1              = 17,
    MAGO_OBJECT_LPF2              = 18,
    MAGO_OBJECT_LPF               = 19,
    MAGO_OBJECT_HPF1              = 20,
    MAGO_OBJECT_HPF2              = 21,
    MAGO_OBJECT_HPF               = 22,
    MAGO_OBJECT_BPF2              = 23,
    MAGO_OBJECT_BPF               = 24,
    MAGO_OBJECT_NOTCH2            = 25,
    MAGO_OBJECT_PEAK2             = 26,
    MAGO_OBJECT_LOSHELF2          = 27,
    MAGO_OBJECT_HISHELF2          = 28,
    MAGO_OBJECT_DELAY             = 29
};

MAGO_API void* mago_alloc(int type)
{
    switch (type)
    {
        case MAGO_OBJECT_CONTEXT:           return calloc(1, sizeof(ma_context));
        case MAGO_OBJECT_DEVICE:            return calloc(1, sizeof(ma_device));
        case MAGO_OBJECT_LOG:               return calloc(1, sizeof(ma_log));
        case MAGO_OBJECT_DEVICE_INFO:       return calloc(1, sizeof(ma_device_info));
        case MAGO_OBJECT_CONTEXT_CONFIG:    return calloc(1, sizeof(ma_context_config));
        case MAGO_OBJECT_CHANNEL_CONVERTER: return calloc(1, sizeof(ma_channel_converter));
        case MAGO_OBJECT_RESAMPLER:         return calloc(1, sizeof(ma_resampler));
        case MAGO_OBJECT_LINEAR_RESAMPLER:  return calloc(1, sizeof(ma_linear_resampler));
        case MAGO_OBJECT_DATA_CONVERTER:    return calloc(1, sizeof(ma_data_converter));
        case MAGO_OBJECT_AUDIO_BUFFER:      return calloc(1, sizeof(ma_audio_buffer));
        case MAGO_OBJECT_AUDIO_BUFFER_REF:  return calloc(1, sizeof(ma_audio_buffer_ref));
        case MAGO_OBJECT_RB:                return calloc(1, sizeof(ma_rb));
        case MAGO_OBJECT_PCM_RB:            return calloc(1, sizeof(ma_pcm_rb));
        case MAGO_OBJECT_WAVEFORM:          return calloc(1, sizeof(ma_waveform));
        case MAGO_OBJECT_NOISE:             return calloc(1, sizeof(ma_noise));
        case MAGO_OBJECT_BIQUAD:            return calloc(1, sizeof(ma_biquad));
        case MAGO_OBJECT_LPF1:              return calloc(1, sizeof(ma_lpf1));
        case MAGO_OBJECT_LPF2:              return calloc(1, sizeof(ma_lpf2));
        case MAGO_OBJECT_LPF:               return calloc(1, sizeof(ma_lpf));
        case MAGO_OBJECT_HPF1:              return calloc(1, sizeof(ma_hpf1));
        case MAGO_OBJECT_HPF2:              return calloc(1, sizeof(ma_hpf2));
        case MAGO_OBJECT_HPF:               return calloc(1, sizeof(ma_hpf));
        case MAGO_OBJECT_BPF2:              return calloc(1, sizeof(ma_bpf2));
        case MAGO_OBJECT_BPF:               return calloc(1, sizeof(ma_bpf));
        case MAGO_OBJECT_NOTCH2:            return calloc(1, sizeof(ma_notch2));
        case MAGO_OBJECT_PEAK2:             return calloc(1, sizeof(ma_peak2));
        case MAGO_OBJECT_LOSHELF2:          return calloc(1, sizeof(ma_loshelf2));
        case MAGO_OBJECT_HISHELF2:          return calloc(1, sizeof(ma_hishelf2));
        case MAGO_OBJECT_DELAY:             return calloc(1, sizeof(ma_delay));
        default:                            return NULL;
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
} mago_stream_config;

typedef struct
{
    ma_uint32 deviceType;
    const mago_stream_config* pPlayback;
    const mago_stream_config* pCapture;
    uintptr_t dataCallback;
    uintptr_t notificationCallback;
    uintptr_t userData;
} mago_device_config;

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

static ma_result mago_resolve_device_id(
    ma_context* pContext,
    ma_device_type type,
    const mago_stream_config* pStream,
    const ma_device_id** ppDeviceID)
{
    ma_device_info* pInfos;
    ma_uint32 count;

    *ppDeviceID = pStream->pDeviceID;
    if (pStream->deviceIndex < 0)
    {
        return MA_SUCCESS;
    }

    if (pContext == NULL)
    {
        return MA_INVALID_ARGS;
    }

    if (type == ma_device_type_capture || type == ma_device_type_loopback)
    {
        if (ma_context_get_devices(pContext, NULL, NULL, &pInfos, &count) != MA_SUCCESS)
        {
            return MA_NO_DEVICE;
        }
    }
    else
    {
        if (ma_context_get_devices(pContext, &pInfos, &count, NULL, NULL) != MA_SUCCESS)
        {
            return MA_NO_DEVICE;
        }
    }

    if ((ma_uint32)pStream->deviceIndex >= count)
    {
        return MA_NO_DEVICE;
    }

    *ppDeviceID = &pInfos[pStream->deviceIndex].id;
    return MA_SUCCESS;
}

static void mago_apply_stream_config(
    ma_device_config* pConfig,
    ma_device_type type,
    const mago_stream_config* pStream,
    const ma_device_id* pDeviceID)
{
    if (type == ma_device_type_capture || type == ma_device_type_loopback)
    {
        pConfig->capture.pDeviceID = pDeviceID;
        pConfig->capture.format = pStream->format;
        pConfig->capture.channels = pStream->channels;
        pConfig->capture.shareMode = pStream->shareMode;
    }
    else
    {
        pConfig->playback.pDeviceID = pDeviceID;
        pConfig->playback.format = pStream->format;
        pConfig->playback.channels = pStream->channels;
        pConfig->playback.shareMode = pStream->shareMode;
    }
}

MAGO_API ma_result mago_device_init(
    ma_context* pContext,
    const mago_device_config* pMagoConfig,
    ma_device** ppDevice)
{
    ma_device_config config;
    ma_device* pDevice;
    mago_device_bridge* pBridge;
    ma_device_type type;
    ma_result result;

    if (pMagoConfig == NULL || ppDevice == NULL)
    {
        return MA_INVALID_ARGS;
    }

    *ppDevice = NULL;
    type = (ma_device_type)pMagoConfig->deviceType;

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

    config = ma_device_config_init(type);

    if (type == ma_device_type_playback || type == ma_device_type_duplex)
    {
        const ma_device_id* pDeviceID;

        if (pMagoConfig->pPlayback == NULL)
        {
            free(pBridge);
            free(pDevice);
            return MA_INVALID_ARGS;
        }

        result = mago_resolve_device_id(pContext, ma_device_type_playback, pMagoConfig->pPlayback, &pDeviceID);
        if (result != MA_SUCCESS)
        {
            free(pBridge);
            free(pDevice);
            return result;
        }

        mago_apply_stream_config(&config, ma_device_type_playback, pMagoConfig->pPlayback, pDeviceID);
        config.sampleRate = pMagoConfig->pPlayback->sampleRate;
        config.periodSizeInFrames = pMagoConfig->pPlayback->periodSizeInFrames;
        config.periodSizeInMilliseconds = pMagoConfig->pPlayback->periodSizeInMilliseconds;
        config.periods = pMagoConfig->pPlayback->periods;
        config.performanceProfile = pMagoConfig->pPlayback->performanceProfile;
        config.noPreSilencedOutputBuffer = (ma_bool8)pMagoConfig->pPlayback->noPreSilencedOutputBuffer;
        config.noClip = (ma_bool8)pMagoConfig->pPlayback->noClip;
        config.noDisableDenormals = (ma_bool8)pMagoConfig->pPlayback->noDisableDenormals;
        config.noFixedSizedCallback = (ma_bool8)pMagoConfig->pPlayback->noFixedSizedCallback;
    }

    if (type == ma_device_type_capture || type == ma_device_type_duplex || type == ma_device_type_loopback)
    {
        const ma_device_id* pDeviceID;

        if (pMagoConfig->pCapture == NULL)
        {
            free(pBridge);
            free(pDevice);
            return MA_INVALID_ARGS;
        }

        result = mago_resolve_device_id(pContext, type, pMagoConfig->pCapture, &pDeviceID);
        if (result != MA_SUCCESS)
        {
            free(pBridge);
            free(pDevice);
            return result;
        }

        mago_apply_stream_config(&config, type, pMagoConfig->pCapture, pDeviceID);
        if (type != ma_device_type_duplex)
        {
            config.sampleRate = pMagoConfig->pCapture->sampleRate;
            config.periodSizeInFrames = pMagoConfig->pCapture->periodSizeInFrames;
            config.periodSizeInMilliseconds = pMagoConfig->pCapture->periodSizeInMilliseconds;
            config.periods = pMagoConfig->pCapture->periods;
            config.performanceProfile = pMagoConfig->pCapture->performanceProfile;
            config.noFixedSizedCallback = (ma_bool8)pMagoConfig->pCapture->noFixedSizedCallback;
        }
    }

    config.dataCallback = pBridge->dataCallback != NULL ? mago_on_device_data : NULL;
    config.notificationCallback = pBridge->notificationCallback != NULL ? mago_on_device_notification : NULL;
    config.pUserData = pBridge;

    result = ma_device_init(pContext, &config, pDevice);
    if (result != MA_SUCCESS)
    {
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

/* -------------------------------------------------------------------------
 * Context config helpers.
 * ma_context_config is 240 bytes of backend-specific state returned by value,
 * so Go treats it as an opaque buffer and only sets the log pointer.
 * ---------------------------------------------------------------------- */

MAGO_API void mago_context_config_init(void* pOut)
{
    if (pOut != NULL)
    {
        *(ma_context_config*)pOut = ma_context_config_init();
    }
}

MAGO_API void mago_context_config_set_log(void* pConfig, ma_log* pLog)
{
    if (pConfig != NULL)
    {
        ((ma_context_config*)pConfig)->pLog = pLog;
    }
}
