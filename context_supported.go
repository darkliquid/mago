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
	owned  bool
}

func (lib *Library) NewContext(backends ...Backend) (*Context, error) {
	return lib.newContextWithConfig(nil, backends...)
}

// NewContextWithLog creates a context that reports miniaudio diagnostics to the
// given log.
func (lib *Library) NewContextWithLog(log *Log, backends ...Backend) (*Context, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	config := lib.bindings.magoAlloc(magoObjectContextConfig)
	if config == nil {
		return nil, fmt.Errorf("mago: allocate context config: out of memory")
	}
	defer lib.bindings.magoFree(config)

	lib.bindings.magoContextConfigInit(config)
	if log != nil {
		lib.bindings.magoContextConfigSetLog(config, log.handle)
	}
	return lib.newContextWithConfig(config, backends...)
}

func (lib *Library) newContextWithConfig(config unsafe.Pointer, backends ...Backend) (*Context, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	handle := (*contextHandle)(lib.bindings.magoAlloc(magoObjectContext))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate context: out of memory")
	}

	var result Result

	if len(backends) == 0 {
		result = lib.bindings.maContextInit(nil, 0, config, handle)
	} else {
		backendCount, err := intToUint32(len(backends))
		if err != nil {
			lib.bindings.magoFree(unsafe.Pointer(handle))
			return nil, fmt.Errorf("mago: %w", err)
		}
		result = lib.bindings.maContextInit(&backends[0], backendCount, config, handle)
		runtime.KeepAlive(backends)
	}
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_context_init", result)
	}

	return &Context{lib: lib, handle: handle, owned: true}, nil
}

func (ctx *Context) Close() error {
	if ctx == nil || ctx.handle == nil {
		return nil
	}
	if !ctx.owned {
		ctx.handle = nil
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

// Log returns the context's own log, or nil when none is attached. The result
// is borrowed: closing it only detaches the handle.
func (ctx *Context) Log() *Log {
	if ctx == nil || ctx.handle == nil {
		return nil
	}
	return borrowedLog(ctx.lib, ctx.lib.bindings.maContextGetLog(ctx.handle))
}

// DeviceInfo reports basic information for a device, or for the default device
// when id is nil.
func (ctx *Context) DeviceInfo(t DeviceType, id *DeviceID) (DeviceInfo, error) {
	if ctx == nil || ctx.handle == nil {
		return DeviceInfo{}, fmt.Errorf("mago: nil context")
	}
	if err := ctx.lib.ensureOpen(); err != nil {
		return DeviceInfo{}, err
	}

	native := (*deviceInfoNative)(ctx.lib.bindings.magoAlloc(magoObjectDeviceInfo))
	if native == nil {
		return DeviceInfo{}, fmt.Errorf("mago: allocate device info: out of memory")
	}
	defer ctx.lib.bindings.magoFree(unsafe.Pointer(native))

	var idPtr unsafe.Pointer
	if id != nil {
		idPtr = unsafe.Pointer(&id[0])
	}

	if result := ctx.lib.bindings.maContextGetDeviceInfo(ctx.handle, t, idPtr, native); result != Success {
		return DeviceInfo{}, ctx.lib.resultError("ma_context_get_device_info", result)
	}
	return copyDeviceInfo(native), nil
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
		ID:        native.ID,
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
