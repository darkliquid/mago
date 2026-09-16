package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildlibCLIUsageAndTargetValidation(t *testing.T) {
	// Build the buildlib binary
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "buildlib")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build buildlib binary: %v\n%s", err, out)
	}

	// Test invalid target format
	cmd := exec.Command(binPath, "-target", "invalid-target-format")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for invalid target format, got nil")
	}
	if !strings.Contains(string(out), "invalid target format") {
		t.Errorf("unexpected output: %s", string(out))
	}

	// Test unsupported target
	cmd = exec.Command(binPath, "-target", "solaris/sparc")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for unsupported target, got nil")
	}
	if !strings.Contains(string(out), "unsupported platform solaris/sparc") {
		t.Errorf("unexpected output: %s", string(out))
	}

	// Test too many arguments
	cmd = exec.Command(binPath, "arg1", "arg2")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for extra arguments, got nil")
	}
	if !strings.Contains(string(out), "usage:") {
		t.Errorf("unexpected output: %s", string(out))
	}
}

func TestBuildlibCLIConflictingFlags(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "buildlib")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build buildlib binary: %v\n%s", err, out)
	}

	// -all and -target
	cmd := exec.Command(binPath, "-all", "-target", "linux/amd64")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for conflicting -all and -target, got nil")
	}
	if !strings.Contains(string(out), "cannot specify more than one") {
		t.Errorf("unexpected output: %s", string(out))
	}

	// -all and -download-only
	cmd = exec.Command(binPath, "-all", "-download-only", "/tmp/miniaudio.h")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for conflicting -all and -download-only, got nil")
	}
	if !strings.Contains(string(out), "cannot specify more than one") {
		t.Errorf("unexpected output: %s", string(out))
	}

	// -target and -download-only
	cmd = exec.Command(binPath, "-target", "linux/amd64", "-download-only", "/tmp/miniaudio.h")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for conflicting -target and -download-only, got nil")
	}
	if !strings.Contains(string(out), "cannot specify more than one") {
		t.Errorf("unexpected output: %s", string(out))
	}

	// -all with positional output path
	cmd = exec.Command(binPath, "-all", "/tmp/out.so")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for -all with positional output path, got nil")
	}
	if !strings.Contains(string(out), "positional output path cannot be used with -all") {
		t.Errorf("unexpected output: %s", string(out))
	}

	// -download-only with positional output path
	cmd = exec.Command(binPath, "-download-only", "/tmp/miniaudio.h", "/tmp/out.so")
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for -download-only with positional output path, got nil")
	}
	if !strings.Contains(string(out), "positional output path cannot be used with -download-only") {
		t.Errorf("unexpected output: %s", string(out))
	}
}

func TestBuildlibDownloadOnly(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "buildlib")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build buildlib binary: %v\n%s", err, out)
	}

	dstPath := filepath.Join(t.TempDir(), "miniaudio.h")
	// Using a dummy version that will fail HTTP download or root without bindings, checking error handling
	cmd := exec.Command(binPath, "-version", "0.0.0-dummy", "-download-only", dstPath)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected download failure for dummy version, got success")
	}
	if !strings.Contains(string(out), "error:") {
		t.Errorf("expected error message, got %s", string(out))
	}
	if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
		t.Errorf("dstPath should not exist on failure")
	}
}

func TestBuildlibTargetWithPositionalOutputPath(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "buildlib")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build buildlib binary: %v\n%s", err, out)
	}

	outPath := filepath.Join(t.TempDir(), "custom-out", "libminiaudio.so")
	cmd := exec.Command(binPath, "-version", "0.0.0-dummy", "-target", "linux/amd64", outPath)
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected failure for dummy version, got success")
	}
	if strings.Contains(string(out), "usage:") {
		t.Errorf("did not expect usage error when passing positional output path with -target: %s", string(out))
	}
}
