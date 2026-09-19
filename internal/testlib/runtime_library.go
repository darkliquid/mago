package testlib

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/darkliquid/mago/internal/buildlib"
)

var (
	buildMu    sync.Mutex
	buildPaths = map[string]string{}
)

// BuildRuntimeLibrary compiles the native bridge once per test binary and
// returns the shared library path. Every test in a package uses the same build
// because recompiling the bridge for each test invokes the C compiler dozens of
// times per run, which is slow enough on Windows CI to hit the job timeout.
//
// The compiled library is left in a temporary directory for the life of the test
// process; the operating system reclaims it afterwards.
func BuildRuntimeLibrary(t testing.TB, repoRoot string) string {
	t.Helper()

	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	buildMu.Lock()
	defer buildMu.Unlock()

	if path, ok := buildPaths[absRoot]; ok {
		return path
	}

	dir, err := os.MkdirTemp("", "mago-testlib-*")
	if err != nil {
		t.Fatalf("create temporary directory for shared library: %v", err)
	}

	libPath := filepath.Join(dir, buildlib.DefaultLibraryFilename(runtime.GOOS))
	if err := buildlib.Build(absRoot, libPath, ""); err != nil {
		_ = os.RemoveAll(dir)
		t.Fatalf("build shared library: %v", err)
	}

	buildPaths[absRoot] = libPath
	return libPath
}
