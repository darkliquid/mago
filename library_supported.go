//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

	handle, err := openLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("open shared library %q: %w", path, err)
	}

	lib := &Library{handle: handle, path: path}
	if err := lib.register(); err != nil {
		_ = closeLibrary(handle)
		return nil, err
	}
	if err := lib.validateLoadedVersion(); err != nil {
		_ = closeLibrary(handle)
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

	if err := closeLibrary(lib.handle); err != nil {
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

var (
	embeddedLibData []byte
	embeddedLibName string
)

func resolveLibraryPath(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}

	if envPath := os.Getenv(envLibraryPath); envPath != "" {
		return filepath.Abs(envPath)
	}

	// Try to find the library in the cache if it's embedded
	if len(embeddedLibData) > 0 {
		if cachePath, err := resolveCachedLibrary(); err == nil {
			return cachePath, nil
		}
	}

	var candidates []string
	names := defaultLibraryNames()
	if wd, err := os.Getwd(); err == nil {
		for _, name := range names {
			candidates = append(candidates,
				filepath.Join(wd, "native", name),
				filepath.Join(wd, name),
			)
		}
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		for _, name := range names {
			candidates = append(candidates,
				filepath.Join(exeDir, "native", name),
				filepath.Join(exeDir, name),
			)
		}
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("mago: could not find a runtime miniaudio library (%v); set %s or use WithLibraryPath (searched %v)", names, envLibraryPath, candidates)
}

func resolveCachedLibrary() (string, error) {
	if len(embeddedLibData) == 0 || embeddedLibName == "" {
		return "", errors.New("mago: no embedded library")
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("mago: could not determine user cache directory: %w", err)
	}

	// We include the version in the path to avoid conflicts between different mago versions
	magoCacheDir := filepath.Join(cacheDir, "mago", fmt.Sprintf("%d.%d.%d", ExpectedMiniaudioVersionMajor, ExpectedMiniaudioVersionMinor, ExpectedMiniaudioVersionRevision))
	if err := os.MkdirAll(magoCacheDir, 0o755); err != nil {
		return "", fmt.Errorf("mago: could not create cache directory: %w", err)
	}

	cachePath := filepath.Join(magoCacheDir, embeddedLibName)
	if _, err := os.Stat(cachePath); err == nil {
		// Cache hit!
		return cachePath, nil
	}

	// Cache miss, extract the library
	if err := extractLibraryToCache(cachePath); err != nil {
		return "", err
	}

	return cachePath, nil
}

func extractLibraryToCache(path string) error {
	// Use a temporary file to avoid race conditions with other processes
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, embeddedLibData, 0o600); err != nil {
		return fmt.Errorf("mago: could not write library to cache: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("mago: could not move library to final cache location: %w", err)
	}

	return nil
}
