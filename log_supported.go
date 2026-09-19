//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// LogLevel mirrors ma_log_level.
type LogLevel uint32

const (
	LogLevelDebug   LogLevel = 0
	LogLevelInfo    LogLevel = 1
	LogLevelWarning LogLevel = 2
	LogLevelError   LogLevel = 3
)

// LogCallback receives miniaudio log messages.
type LogCallback func(level LogLevel, message string)

type logState struct{ fn LogCallback }

var (
	logCallbackSeq atomic.Uint64
	logCallbacks   sync.Map // token -> *logState
)

var logCallbackPtr = purego.NewCallback(func(token uintptr, level uint32, message uintptr) uintptr {
	value, ok := logCallbacks.Load(token)
	if !ok || message == 0 {
		return 0
	}
	if state, ok := value.(*logState); ok && state.fn != nil {
		state.fn(LogLevel(level), goCString(message))
	}
	return 0
})

// goCString copies a NUL-terminated C string into Go.
func goCString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Pointer(ptr + uintptr(n))) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n))
}

// Log wraps a miniaudio ma_log.
type Log struct {
	lib    *Library
	handle *logHandle
}

// NewLog allocates and initializes a log bound to this library.
func (lib *Library) NewLog() (*Log, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	handle := (*logHandle)(lib.bindings.magoAlloc(magoObjectLog))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate log: out of memory")
	}

	if result := lib.bindings.maLogInit(nil, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_log_init", result)
	}

	return &Log{lib: lib, handle: handle}, nil
}

// Register adds a callback and returns a token needed to Unregister it.
func (l *Log) Register(fn LogCallback) (uintptr, error) {
	if l == nil || l.handle == nil {
		return 0, fmt.Errorf("mago: nil log")
	}

	token := uintptr(logCallbackSeq.Add(1))
	logCallbacks.Store(token, &logState{fn: fn})
	if result := l.lib.bindings.magoLogRegisterCallback(l.handle, logCallbackPtr, token); result != Success {
		logCallbacks.Delete(token)
		return 0, l.lib.resultError("ma_log_register_callback", result)
	}
	return token, nil
}

// Unregister removes a previously registered callback by token.
func (l *Log) Unregister(token uintptr) error {
	if l == nil || l.handle == nil {
		return fmt.Errorf("mago: nil log")
	}

	result := l.lib.bindings.magoLogUnregisterCallback(l.handle, logCallbackPtr, token)
	logCallbacks.Delete(token)
	return l.lib.resultError("ma_log_unregister_callback", result)
}

// Post emits a message at the given level.
func (l *Log) Post(level LogLevel, message string) error {
	if l == nil || l.handle == nil {
		return fmt.Errorf("mago: nil log")
	}
	return l.lib.resultError("ma_log_post", l.lib.bindings.maLogPost(l.handle, uint32(level), message))
}

// LevelString returns the printable name of a log level.
func (l *Log) LevelString(level LogLevel) string {
	if l == nil || l.handle == nil {
		return ""
	}
	return l.lib.bindings.maLogLevelToString(uint32(level))
}

// Close uninitializes and frees the log.
func (l *Log) Close() error {
	if l == nil || l.handle == nil {
		return nil
	}
	if err := l.lib.ensureOpen(); err != nil {
		return err
	}

	l.lib.bindings.maLogUninit(l.handle)
	l.lib.bindings.magoFree(unsafe.Pointer(l.handle))
	l.handle = nil
	return nil
}
