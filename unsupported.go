//go:build !darwin && !freebsd && !linux && !netbsd && !windows

package mago

import "errors"

var errUnsupportedPlatform = errors.New("mago: this package is currently supported on darwin, freebsd, linux, netbsd, and windows")

type Library struct{}
type Context struct{}
type Device struct{}

type LibraryOption func(*struct{})
type DataCallback func(*Device, any, any, uint32)
type NotificationCallback func(*Device, NotificationType)
type PlaybackDeviceConfig struct{}

type OpError struct {
	Op          string
	Code        Result
	Description string
}

func (e *OpError) Error() string                        { return errUnsupportedPlatform.Error() }
func WithLibraryPath(string) LibraryOption              { return func(*struct{}) {} }
func Open(...LibraryOption) (*Library, error)           { return nil, errUnsupportedPlatform }
func DefaultPlaybackDeviceConfig() PlaybackDeviceConfig { return PlaybackDeviceConfig{} }

type LogLevel uint32

const (
	LogLevelDebug   LogLevel = 0
	LogLevelInfo    LogLevel = 1
	LogLevelWarning LogLevel = 2
	LogLevelError   LogLevel = 3
)

type LogCallback func(level LogLevel, message string)

type Log struct{}

func (lib *Library) NewLog() (*Log, error)           { return nil, errUnsupportedPlatform }
func (l *Log) Close() error                          { return errUnsupportedPlatform }
func (l *Log) Register(LogCallback) (uintptr, error) { return 0, errUnsupportedPlatform }
func (l *Log) Unregister(uintptr) error              { return errUnsupportedPlatform }
func (l *Log) Post(LogLevel, string) error           { return errUnsupportedPlatform }
func (l *Log) LevelString(LogLevel) string           { return "" }
