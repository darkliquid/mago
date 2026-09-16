package main

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatByteString(t *testing.T) {
	data := []byte{0x7f, 0x45, 0x4c, 0x46, 0x00, 0x01, 0x02}
	formatted := formatByteString(data)
	expected := `"\x7f\x45\x4c\x46\x00\x01\x02"`
	if formatted != expected {
		t.Errorf("expected %s, got %s", expected, formatted)
	}
}

func TestGenerateTargetEmbed(t *testing.T) {
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "internal", "lib", "linux-amd64")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testLib := filepath.Join(libDir, "libminiaudio.so")
	if err := os.WriteFile(testLib, []byte{0x7f, 0x45, 0x4c, 0x46}, 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(tmpDir, "embed_linux_amd64.go")
	target := embedTarget{
		goos:      "linux",
		goarch:    "amd64",
		varSuffix: "LinuxAmd64",
		libName:   "libminiaudio.so",
		fileName:  "embed_linux_amd64.go",
	}

	if err := generateEmbedFile(tmpDir, target); err != nil {
		t.Fatalf("generateEmbedFile failed: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	str := string(content)
	if !strings.Contains(str, "//go:build linux && amd64") {
		t.Errorf("missing build tag: %s", str)
	}
	if !strings.Contains(str, "var embeddedLibDataLinuxAmd64 = []byte(") {
		t.Errorf("missing variable declaration: %s", str)
	}
	if !strings.Contains(str, `embeddedLibName = "libminiaudio.so"`) {
		t.Errorf("missing lib name assignment: %s", str)
	}
	if strings.Contains(str, `//go:embed`) {
		t.Errorf("unexpected go:embed directive in generated file")
	}
	if strings.Contains(str, `import _ "embed"`) {
		t.Errorf("unexpected import _ \"embed\" in generated file")
	}

	if _, err := format.Source(content); err != nil {
		t.Errorf("output Go file does not format cleanly: %v", err)
	}
}

func TestGenerateTargetEmbed_MissingLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	target := embedTarget{
		goos:      "linux",
		goarch:    "amd64",
		varSuffix: "LinuxAmd64",
		libName:   "libminiaudio.so",
		fileName:  "embed_linux_amd64.go",
	}

	err := generateEmbedFile(tmpDir, target)
	if err == nil {
		t.Fatal("expected error for missing library, got nil")
	}
	if !strings.Contains(err.Error(), "run 'mise run build-lib-all' first") {
		t.Errorf("expected error message to suggest running 'mise run build-lib-all', got: %v", err)
	}
}

func TestGenerateTargetEmbed_EmptyLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "internal", "lib", "linux-amd64")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testLib := filepath.Join(libDir, "libminiaudio.so")
	if err := os.WriteFile(testLib, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	target := embedTarget{
		goos:      "linux",
		goarch:    "amd64",
		varSuffix: "LinuxAmd64",
		libName:   "libminiaudio.so",
		fileName:  "embed_linux_amd64.go",
	}

	err := generateEmbedFile(tmpDir, target)
	if err == nil {
		t.Fatal("expected error for empty library, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected error to mention empty library, got: %v", err)
	}
}

func TestGenerateTargetEmbed_MultiChunk(t *testing.T) {
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "internal", "lib", "windows-amd64")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 150 bytes to test >2 chunks of 64 bytes
	testData := make([]byte, 150)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	testLib := filepath.Join(libDir, "miniaudio.dll")
	if err := os.WriteFile(testLib, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(tmpDir, "embed_windows_amd64.go")
	target := embedTarget{
		goos:      "windows",
		goarch:    "amd64",
		varSuffix: "WindowsAmd64",
		libName:   "miniaudio.dll",
		fileName:  "embed_windows_amd64.go",
	}

	if err := generateEmbedFile(tmpDir, target); err != nil {
		t.Fatalf("generateEmbedFile failed: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := format.Source(content); err != nil {
		t.Errorf("output Go file does not format cleanly: %v", err)
	}

	str := string(content)
	if !strings.Contains(str, "+") {
		t.Errorf("expected multi-chunk file to contain '+' concatenation: %s", str)
	}
	if !strings.Contains(str, `embeddedLibName = "miniaudio.dll"`) {
		t.Errorf("expected miniaudio.dll: %s", str)
	}
}
