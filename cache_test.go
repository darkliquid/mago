package mago

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheExtraction(t *testing.T) {
	// Ensure we have embedded data for the current platform
	if len(embeddedLibData) == 0 {
		t.Skip("No embedded library data for this platform")
	}

	// We'll use a custom HOME/XDG_CACHE_HOME to avoid messing with the real one
	tmpDir := t.TempDir()
	oldCache := os.Getenv("XDG_CACHE_HOME")
	os.Setenv("XDG_CACHE_HOME", tmpDir)
	defer os.Setenv("XDG_CACHE_HOME", oldCache)

	// On windows/darwin, UserCacheDir might use different env vars, 
	// but on Linux it uses XDG_CACHE_HOME then .cache in HOME.
	// For simplicity in this test on Linux:
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	path, err := resolveCachedLibrary()
	if err != nil {
		t.Fatalf("resolveCachedLibrary failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cached library not found at %s: %v", path, err)
	}

	// Check if it's in the expected place
	if !filepath.HasPrefix(path, tmpDir) {
		t.Errorf("expected cache path to be under %s, got %s", tmpDir, path)
	}

	// Verify we can Open it
	lib, err := Open(WithLibraryPath(path))
	if err != nil {
		t.Fatalf("failed to open extracted library: %v", err)
	}
	defer lib.Close()

	version, err := lib.Version()
	if err != nil {
		t.Fatalf("failed to get version from extracted library: %v", err)
	}
	t.Logf("Extracted library version: %s", version)
}
