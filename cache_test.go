package mago

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheExtraction(t *testing.T) {
	// Ensure we have embedded data for the current platform
	if len(embeddedLibData) == 0 {
		t.Skip("No embedded library data for this platform")
	}

	// We'll use a custom environment to avoid messing with the real user cache.
	// On windows, os.UserCacheDir uses LOCALAPPDATA.
	// On darwin, it uses HOME/Library/Caches.
	// On other unix, it uses XDG_CACHE_HOME then HOME/.cache.
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	path, err := resolveCachedLibrary()
	if err != nil {
		t.Fatalf("resolveCachedLibrary failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cached library not found at %s: %v", path, err)
	}

	// Check if it's in the expected place
	rel, err := filepath.Rel(tmpDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Errorf("expected cache path to be under %s, got %s", tmpDir, path)
	}

	// Verify we can Open it
	lib, err := Open(WithLibraryPath(path))
	if err != nil {
		t.Fatalf("failed to open extracted library: %v", err)
	}
	defer func() {
		if err := lib.Close(); err != nil {
			t.Errorf("lib.Close failed: %v", err)
		}
	}()

	version, err := lib.Version()
	if err != nil {
		t.Fatalf("failed to get version from extracted library: %v", err)
	}
	t.Logf("Extracted library version: %s", version)
}
