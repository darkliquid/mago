//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"unsafe"
)

type Context struct {
	lib    *Library
	handle *contextHandle
}

func (lib *Library) NewContext(backends ...Backend) (*Context, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	var handle *contextHandle
	var result Result

	if len(backends) == 0 {
		result = lib.bindings.magoContextInitDefault(&handle)
	} else {
		backendCount, err := intToUint32(len(backends))
		if err != nil {
			return nil, fmt.Errorf("mago: %w", err)
		}
		result = lib.bindings.magoContextInitWithBackends(&backends[0], backendCount, &handle)
		runtime.KeepAlive(backends)
	}
	if result != Success {
		return nil, lib.resultError("ma_context_init", result)
	}

	return &Context{lib: lib, handle: handle}, nil
}

func (ctx *Context) Close() error {
	if ctx == nil || ctx.handle == nil {
		return nil
	}

	if err := ctx.lib.ensureOpen(); err != nil {
		return err
	}

	ctx.lib.bindings.magoContextUninitFree(ctx.handle)
	ctx.handle = nil
	return nil
}

func (ctx *Context) Devices() ([]DeviceInfo, []DeviceInfo, error) {
	if ctx == nil || ctx.handle == nil {
		return nil, nil, nil
	}
	if err := ctx.lib.ensureOpen(); err != nil {
		return nil, nil, err
	}

	var playbackNative *deviceInfoNative
	var playbackCount uint32
	var captureNative *deviceInfoNative
	var captureCount uint32

	result := ctx.lib.bindings.magoContextGetDevices(ctx.handle, &playbackNative, &playbackCount, &captureNative, &captureCount)
	if result != Success {
		return nil, nil, ctx.lib.resultError("ma_context_get_devices", result)
	}
	defer func() {
		if playbackNative != nil {
			ctx.lib.bindings.magoContextFreeDeviceInfos(playbackNative)
		}
		if captureNative != nil {
			ctx.lib.bindings.magoContextFreeDeviceInfos(captureNative)
		}
	}()

	playback := copyDeviceInfos(playbackNative, playbackCount)
	capture := copyDeviceInfos(captureNative, captureCount)
	return playback, capture, nil
}

func copyDeviceInfos(native *deviceInfoNative, count uint32) []DeviceInfo {
	if native == nil || count == 0 {
		return nil
	}

	items := unsafe.Slice(native, count)
	out := make([]DeviceInfo, 0, count)
	for _, item := range items {
		nameBytes := item.Name[:]
		if idx := bytes.IndexByte(nameBytes, 0); idx >= 0 {
			nameBytes = nameBytes[:idx]
		}
		out = append(out, DeviceInfo{
			Name:      string(nameBytes),
			IsDefault: item.IsDefault != 0,
		})
	}

	return out
}

func intToUint32(v int) (uint32, error) {
	if v < 0 {
		return 0, fmt.Errorf("value %d exceeds uint32 range", v)
	}
	var out uint32
	if _, err := fmt.Sscan(strconv.Itoa(v), &out); err != nil {
		return 0, fmt.Errorf("value %d exceeds uint32 range", v)
	}
	return out, nil
}
