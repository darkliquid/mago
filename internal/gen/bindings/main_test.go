package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveVersion_FromFlag(t *testing.T) {
	maj, minorVal, rev, err := resolveVersion(t.TempDir(), "1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maj != "1" || minorVal != "2" || rev != "3" {
		t.Errorf("got %s.%s.%s, want 1.2.3", maj, minorVal, rev)
	}
}

func TestResolveVersion_InvalidFlag(t *testing.T) {
	_, _, _, err := resolveVersion(t.TempDir(), "1.2")
	if err == nil {
		t.Fatal("expected error for invalid version flag, got nil")
	}
}

func TestResolveVersion_FromMiniaudioHeader(t *testing.T) {
	dir := t.TempDir()
	headerContent := `
#define MA_VERSION_MAJOR    2
#define MA_VERSION_MINOR    3
#define MA_VERSION_REVISION 4
`
	if err := os.WriteFile(filepath.Join(dir, "miniaudio.h"), []byte(headerContent), 0o644); err != nil {
		t.Fatalf("write miniaudio.h: %v", err)
	}

	maj, minorVal, rev, err := resolveVersion(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maj != "2" || minorVal != "3" || rev != "4" {
		t.Errorf("got %s.%s.%s, want 2.3.4", maj, minorVal, rev)
	}
}

func TestResolveVersion_FromExistingBindings(t *testing.T) {
	dir := t.TempDir()
	bindingsContent := `package mago
const (
	ExpectedMiniaudioVersionMajor uint32 = 3
	ExpectedMiniaudioVersionMinor uint32 = 4
	ExpectedMiniaudioVersionRevision uint32 = 5
)
`
	if err := os.WriteFile(filepath.Join(dir, "zz_generated.bindings.go"), []byte(bindingsContent), 0o644); err != nil {
		t.Fatalf("write zz_generated.bindings.go: %v", err)
	}

	maj, minorVal, rev, err := resolveVersion(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maj != "3" || minorVal != "4" || rev != "5" {
		t.Errorf("got %s.%s.%s, want 3.4.5", maj, minorVal, rev)
	}
}

func TestResolveVersion_DefaultFallback(t *testing.T) {
	maj, minorVal, rev, err := resolveVersion(t.TempDir(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maj != "0" || minorVal != "11" || rev != "25" {
		t.Errorf("got %s.%s.%s, want 0.11.25", maj, minorVal, rev)
	}
}
