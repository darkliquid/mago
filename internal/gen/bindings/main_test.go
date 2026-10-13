package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolveVersion_FromGeneratedConstants(t *testing.T) {
	dir := t.TempDir()
	content := `package mago
const (
	ExpectedMiniaudioVersionMajor uint32 = 9
	ExpectedMiniaudioVersionMinor uint32 = 8
	ExpectedMiniaudioVersionRevision uint32 = 7
)
`
	if err := os.WriteFile(filepath.Join(dir, constantsFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", constantsFileName, err)
	}

	maj, minorVal, rev, err := resolveVersion(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maj != "9" || minorVal != "8" || rev != "7" {
		t.Errorf("got %s.%s.%s, want 9.8.7", maj, minorVal, rev)
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

func TestConstantsSourceHasNoImports(t *testing.T) {
	source := string(constantsSource([]constSpec{{Name: "Success", Type: "Result", Value: "0"}}))

	if strings.Contains(source, "import") {
		t.Errorf("constants source must not import anything:\n%s", source)
	}
	if strings.Contains(source, "purego") {
		t.Errorf("constants source must not mention purego:\n%s", source)
	}
	if !strings.Contains(source, generatedHeader) {
		t.Errorf("constants source missing generated header:\n%s", source)
	}
	if !strings.Contains(source, "Success Result = 0") {
		t.Errorf("constants source missing constant:\n%s", source)
	}
}

func TestBindingsSourceIsBuildTagged(t *testing.T) {
	source := string(bindingsSource())

	if !strings.HasPrefix(source, platformBuildTag) {
		t.Errorf("bindings source must start with the platform build tag:\n%s", source)
	}
	if !strings.Contains(source, "github.com/ebitengine/purego") {
		t.Errorf("bindings source must import purego:\n%s", source)
	}
	if strings.Contains(source, "ExpectedMiniaudioVersion") {
		t.Errorf("bindings source must not carry version constants:\n%s", source)
	}
}
