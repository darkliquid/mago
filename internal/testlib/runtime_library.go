package testlib

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/darkliquid/mago/internal/buildlib"
)

func BuildRuntimeLibrary(t testing.TB, repoRoot string) string {
	t.Helper()

	libPath := filepath.Join(t.TempDir(), buildlib.DefaultLibraryFilename(runtime.GOOS))
	if err := buildlib.Build(repoRoot, libPath); err != nil {
		t.Fatalf("build shared library: %v", err)
	}

	return libPath
}
