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

	handle := (*contextHandle)(lib.bindings.magoAlloc(magoObjectContext))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate context: out of memory")
	}

	var result Result

	if len(backends) == 0 {
		result = lib.bindings.maContextInit(nil, 0, nil, handle)
	} else {
		backendCount, err := intToUint32(len(backends))
		if err != nil {
			lib.bindings.magoFree(unsafe.Pointer(handle))
			return nil, fmt.Errorf("mago: %w", err)
		}
		result = lib.bindings.maContextInit(&backends[0], backendCount, nil, handle)
		runtime.KeepAlive(backends)
	}
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
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

	ctx.lib.bindings.maContextUninit(ctx.handle)
	ctx.lib.bindings.magoFree(unsafe.Pointer(ctx.handle))
	ctx.handle = nil
	return nil
}

type deviceEnumerator struct {
	playback []DeviceInfo
	capture  []DeviceInfo
}

func (ctx *Context) Devices() ([]DeviceInfo, []DeviceInfo, error) {
	if ctx == nil || ctx.handle == nil {
		return nil, nil, nil
	}
	if err := ctx.lib.ensureOpen(); err != nil {
		return nil, nil, err
	}

	collector := &deviceEnumerator{}
	token := uintptr(callbackSeq.Add(1))
	callbacks.Store(token, collector)
	defer callbacks.Delete(token)

	result := ctx.lib.bindings.maContextEnumerateDevices(ctx.handle, enumerateCallbackPtr, token)
	if result != Success {
		return nil, nil, ctx.lib.resultError("ma_context_enumerate_devices", result)
	}

	return collector.playback, collector.capture, nil
}

func copyDeviceInfo(native *deviceInfoNative) DeviceInfo {
	nameBytes := native.Name[:]
	if idx := bytes.IndexByte(nameBytes, 0); idx >= 0 {
		nameBytes = nameBytes[:idx]
	}
	return DeviceInfo{
		Name:      string(nameBytes),
		IsDefault: native.IsDefault != 0,
	}
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
