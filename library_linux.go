//go:build linux

package mago

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ebitengine/purego"
)

const envLibraryPath = "MAGO_MINIAUDIO_LIB"

type LibraryOption func(*libraryOptions)

type libraryOptions struct {
	path string
}

type Library struct {
	handle uintptr
	path   string

	mu       sync.RWMutex
	closed   bool
	bindings bindingSet
}

func WithLibraryPath(path string) LibraryOption {
	return func(cfg *libraryOptions) {
		cfg.path = path
	}
}

func Open(options ...LibraryOption) (*Library, error) {
	cfg := libraryOptions{}
	for _, option := range options {
		option(&cfg)
	}

	path, err := resolveLibraryPath(cfg.path)
	if err != nil {
		return nil, err
	}

	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("open shared library %q: %w", path, err)
	}

	lib := &Library{handle: handle, path: path}
	if err := lib.register(); err != nil {
		_ = purego.Dlclose(handle)
		return nil, err
	}
	if err := lib.validateLoadedVersion(); err != nil {
		_ = purego.Dlclose(handle)
		return nil, err
	}

	return lib, nil
}

func (lib *Library) register() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("register shared library symbols: %v", r)
		}
	}()

	lib.bindings.register(lib.handle)
	return nil
}

func (lib *Library) Path() string {
	if lib == nil {
		return ""
	}
	return lib.path
}

func (lib *Library) Close() error {
	if lib == nil {
		return nil
	}

	lib.mu.Lock()
	defer lib.mu.Unlock()

	if lib.closed {
		return nil
	}

	if err := purego.Dlclose(lib.handle); err != nil {
		return fmt.Errorf("close shared library %q: %w", lib.path, err)
	}

	lib.closed = true
	return nil
}

func (lib *Library) ensureOpen() error {
	if lib == nil {
		return errors.New("mago: nil library")
	}

	lib.mu.RLock()
	defer lib.mu.RUnlock()

	if lib.closed {
		return errors.New("mago: library is closed")
	}

	return nil
}

func (lib *Library) Version() (Version, error) {
	if err := lib.ensureOpen(); err != nil {
		return Version{}, err
	}

	var major, minor, revision uint32
	lib.bindings.maVersion(&major, &minor, &revision)
	return Version{Major: major, Minor: minor, Revision: revision}, nil
}

func (lib *Library) VersionString() (string, error) {
	if err := lib.ensureOpen(); err != nil {
		return "", err
	}

	return lib.bindings.maVersionString(), nil
}

func (lib *Library) ResultDescription(code Result) (string, error) {
	if err := lib.ensureOpen(); err != nil {
		return "", err
	}

	return lib.bindings.maResultDescription(code), nil
}

func (lib *Library) resultError(op string, code Result) error {
	if code == Success {
		return nil
	}

	description, err := lib.ResultDescription(code)
	if err != nil {
		return &OpError{Op: op, Code: code}
	}

	return &OpError{Op: op, Code: code, Description: description}
}

func (lib *Library) validateLoadedVersion() error {
	version, err := lib.Version()
	if err != nil {
		return fmt.Errorf("read loaded miniaudio version: %w", err)
	}

	versionString, err := lib.VersionString()
	if err != nil {
		return fmt.Errorf("read loaded miniaudio version string: %w", err)
	}

	return validateLoadedVersion(version, versionString)
}

func validateLoadedVersion(actual Version, actualString string) error {
	expected := Version{
		Major:    ExpectedMiniaudioVersionMajor,
		Minor:    ExpectedMiniaudioVersionMinor,
		Revision: ExpectedMiniaudioVersionRevision,
	}
	expectedString := expected.String()

	if actual != expected || actualString != expectedString {
		return &VersionMismatchError{
			Expected:       expected,
			Actual:         actual,
			ExpectedString: expectedString,
			ActualString:   actualString,
		}
	}

	return nil
}

func resolveLibraryPath(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}

	if envPath := os.Getenv(envLibraryPath); envPath != "" {
		return filepath.Abs(envPath)
	}

	var candidates []string
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "native", "libminiaudio.so"),
			filepath.Join(wd, "libminiaudio.so"),
		)
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "native", "libminiaudio.so"),
			filepath.Join(exeDir, "libminiaudio.so"),
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("mago: could not find libminiaudio.so; set %s or use WithLibraryPath (searched %v)", envLibraryPath, candidates)
}
