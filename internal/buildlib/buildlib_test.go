package buildlib

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSupportedTargets(t *testing.T) {
	targets := AllTargets()
	if len(targets) != 7 {
		t.Fatalf("expected 7 supported targets, got %d", len(targets))
	}

	expected := map[string]string{
		"linux-amd64":   "libminiaudio.so",
		"linux-arm64":   "libminiaudio.so",
		"windows-amd64": "miniaudio.dll",
		"freebsd-amd64": "libminiaudio.so",
		"netbsd-amd64":  "libminiaudio.so",
		"darwin-amd64":  "libminiaudio.dylib",
		"darwin-arm64":  "libminiaudio.dylib",
	}

	for _, target := range targets {
		key := target.GOOS + "-" + target.GOARCH
		expectedFilename, ok := expected[key]
		if !ok {
			t.Errorf("unexpected target: %s", key)
			continue
		}
		if target.Filename != expectedFilename {
			t.Errorf("target %s: expected filename %s, got %s", key, expectedFilename, target.Filename)
		}
	}
}

func TestCompilerArgsHardening(t *testing.T) {
	target := Target{
		GOOS:     "linux",
		GOARCH:   "amd64",
		Filename: "libminiaudio.so",
		Triple:   "x86_64-linux-gnu",
	}

	args := zigCompilerArgs(target, "/repo/out.so", "/repo/native/miniaudio_bridge.c", "/tmp/inc", "/repo")
	joined := strings.Join(args, " ")

	if !strings.Contains(joined, "-ffile-prefix-map=/repo=.") {
		t.Errorf("expected source prefix mapping, got %s", joined)
	}
	if !strings.Contains(joined, "-ffile-prefix-map=/tmp/inc=.") {
		t.Errorf("expected include prefix mapping, got %s", joined)
	}
	if !strings.Contains(joined, "-fvisibility=hidden") {
		t.Errorf("expected -fvisibility=hidden, got %s", joined)
	}
	if !strings.Contains(joined, "-s") {
		t.Errorf("expected -s strip flag, got %s", joined)
	}
	if !strings.Contains(joined, "-target x86_64-linux-gnu") {
		t.Errorf("expected target triple, got %s", joined)
	}
	if !strings.Contains(joined, "-fno-asynchronous-unwind-tables") {
		t.Errorf("expected -fno-asynchronous-unwind-tables, got %s", joined)
	}
	if !strings.Contains(joined, "-fno-ident") {
		t.Errorf("expected -fno-ident, got %s", joined)
	}
}

func TestFindTarget(t *testing.T) {
	tgt, err := FindTarget("linux", "amd64")
	if err != nil {
		t.Fatalf("FindTarget linux/amd64: %v", err)
	}
	if tgt.Triple != "x86_64-linux-gnu" {
		t.Errorf("expected x86_64-linux-gnu, got %s", tgt.Triple)
	}

	_, err = FindTarget("unknown_os", "unknown_arch")
	if err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
}

func TestResolveMiniaudioVersion(t *testing.T) {
	root := newBuildRoot(t, "1.2.3")
	ver, err := ResolveMiniaudioVersion(root, "")
	if err != nil {
		t.Fatalf("ResolveMiniaudioVersion with empty: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("expected 1.2.3, got %s", ver)
	}

	ver, err = ResolveMiniaudioVersion(root, "4.5.6")
	if err != nil {
		t.Fatalf("ResolveMiniaudioVersion with explicit: %v", err)
	}
	if ver != "4.5.6" {
		t.Errorf("expected 4.5.6, got %s", ver)
	}
}

func TestCompilerArgsPerPlatform(t *testing.T) {
	// Windows
	winTarget := Target{GOOS: "windows", GOARCH: "amd64", Filename: "miniaudio.dll", Triple: "x86_64-windows-gnu"}
	winArgs := strings.Join(zigCompilerArgs(winTarget, "/out.dll", "/src.c", "/inc", "/root"), " ")
	for _, expected := range []string{"-shared", "-lwinmm", "-lole32", "-luuid"} {
		if !strings.Contains(winArgs, expected) {
			t.Errorf("windows args missing %s: %s", expected, winArgs)
		}
	}

	// FreeBSD
	freebsdTarget := Target{GOOS: "freebsd", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-freebsd"}
	freebsdArgs := strings.Join(zigCompilerArgs(freebsdTarget, "/out.so", "/src.c", "/inc", "/root"), " ")
	for _, expected := range []string{"-shared", "-fPIC", "-lm", "-lpthread"} {
		if !strings.Contains(freebsdArgs, expected) {
			t.Errorf("freebsd args missing %s: %s", expected, freebsdArgs)
		}
	}

	// NetBSD
	netbsdTarget := Target{GOOS: "netbsd", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-netbsd"}
	netbsdArgs := strings.Join(zigCompilerArgs(netbsdTarget, "/out.so", "/src.c", "/inc", "/root"), " ")
	for _, expected := range []string{"-shared", "-fPIC", "-lm", "-lpthread"} {
		if !strings.Contains(netbsdArgs, expected) {
			t.Errorf("netbsd args missing %s: %s", expected, netbsdArgs)
		}
	}

	// Darwin (unsupported in zigCompilerArgs, uses buildDarwinTarget)
	darwinTarget := Target{GOOS: "darwin", GOARCH: "amd64", Filename: "libminiaudio.dylib", Triple: "x86_64-apple-darwin"}
	darwinArgs := zigCompilerArgs(darwinTarget, "/out.dylib", "/src.c", "/inc", "/root")
	if darwinArgs != nil {
		t.Errorf("expected nil for darwin zigCompilerArgs, got %v", darwinArgs)
	}
}

func TestCurrentTarget(t *testing.T) {
	t.Setenv("GOOS", "windows")
	t.Setenv("GOARCH", "amd64")
	tgt, err := CurrentTarget()
	if err != nil {
		t.Fatalf("CurrentTarget: %v", err)
	}
	if tgt.Key() != "windows-amd64" || tgt.Filename != "miniaudio.dll" {
		t.Errorf("unexpected current target: %+v", tgt)
	}

	t.Setenv("GOOS", "unsupported")
	t.Setenv("GOARCH", "amd64")
	_, err = CurrentTarget()
	if err == nil {
		t.Error("expected error for unsupported current target, got nil")
	}
}

func TestDefaultLibraryFilenameAndOutputPath(t *testing.T) {
	if got := DefaultLibraryFilename("windows"); got != "miniaudio.dll" {
		t.Errorf("windows: expected miniaudio.dll, got %s", got)
	}
	if got := DefaultLibraryFilename("darwin"); got != "libminiaudio.dylib" {
		t.Errorf("darwin: expected libminiaudio.dylib, got %s", got)
	}
	if got := DefaultLibraryFilename("linux"); got != "libminiaudio.so" {
		t.Errorf("linux: expected libminiaudio.so, got %s", got)
	}

	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "arm64")
	outPath := DefaultOutputPath("/root")
	expected := filepath.Join("/root", "internal", "lib", "linux-arm64", "libminiaudio.so")
	if outPath != expected {
		t.Errorf("expected %s, got %s", expected, outPath)
	}
}

func TestDownloadMiniaudioHeaderValidation(t *testing.T) {
	err := DownloadMiniaudioHeader("  ", "/tmp/miniaudio.h")
	if err == nil {
		t.Fatal("expected error for empty version, got nil")
	}
}

func TestDefaultMiniaudioVersion(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "zz_generated.bindings.go"), []byte(`package mago

const (
	ExpectedMiniaudioVersionMajor    uint32 = 1
	ExpectedMiniaudioVersionMinor    uint32 = 22
	ExpectedMiniaudioVersionRevision uint32 = 333
)
`), 0o644); err != nil {
		t.Fatalf("write generated bindings: %v", err)
	}

	version, err := defaultMiniaudioVersion(root)
	if err != nil {
		t.Fatalf("defaultMiniaudioVersion: %v", err)
	}
	if version != "1.22.333" {
		t.Fatalf("unexpected version: %q", version)
	}
}

func TestBuildDownloadsRequestedHeader(t *testing.T) {
	root := newBuildRoot(t, "0.11.25")
	compilerDir := newFakeCompiler(t)

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_, _ = io.WriteString(w, "/* requested header */\n")
	}))
	defer server.Close()

	restoreURLBuilder := miniaudioHeaderURL
	restoreClient := httpClient
	miniaudioHeaderURL = func(version string) string {
		return server.URL + "/" + version + "/miniaudio.h"
	}
	httpClient = server.Client()
	defer func() {
		miniaudioHeaderURL = restoreURLBuilder
		httpClient = restoreClient
	}()

	t.Setenv("CC", "cc")
	t.Setenv("PATH", compilerDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	outPath := filepath.Join(t.TempDir(), "libminiaudio.so")
	if err := Build(root, outPath, "9.8.7"); err != nil {
		t.Fatalf("Build: %v", err)
	}

	if requestedPath != "/9.8.7/miniaudio.h" {
		t.Fatalf("unexpected download path: %q", requestedPath)
	}

	headerCopy, err := os.ReadFile(outPath + ".header")
	if err != nil {
		t.Fatalf("read copied header: %v", err)
	}
	if string(headerCopy) != "/* requested header */\n" {
		t.Fatalf("unexpected downloaded header contents: %q", string(headerCopy))
	}
}

func TestBuildUsesGeneratedVersionWhenUnset(t *testing.T) {
	root := newBuildRoot(t, "3.4.5")
	compilerDir := newFakeCompiler(t)

	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_, _ = io.WriteString(w, "/* default header */\n")
	}))
	defer server.Close()

	restoreURLBuilder := miniaudioHeaderURL
	restoreClient := httpClient
	miniaudioHeaderURL = func(version string) string {
		return server.URL + "/" + version + "/miniaudio.h"
	}
	httpClient = server.Client()
	defer func() {
		miniaudioHeaderURL = restoreURLBuilder
		httpClient = restoreClient
	}()

	t.Setenv("CC", "cc")
	t.Setenv("PATH", compilerDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	outPath := filepath.Join(t.TempDir(), "libminiaudio.so")
	if err := Build(root, outPath, ""); err != nil {
		t.Fatalf("Build: %v", err)
	}

	if requestedPath != "/3.4.5/miniaudio.h" {
		t.Fatalf("unexpected default download path: %q", requestedPath)
	}
}

func newBuildRoot(t *testing.T, version string) string {
	t.Helper()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "native"), 0o755); err != nil {
		t.Fatalf("create native directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "native", "miniaudio_bridge.c"), []byte(`#include "miniaudio.h"`+"\n"), 0o644); err != nil {
		t.Fatalf("write bridge source: %v", err)
	}

	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		t.Fatalf("version %q must have three dot-separated parts", version)
	}
	content := `package mago

const (
	ExpectedMiniaudioVersionMajor    uint32 = ` + parts[0] + `
	ExpectedMiniaudioVersionMinor    uint32 = ` + parts[1] + `
	ExpectedMiniaudioVersionRevision uint32 = ` + parts[2] + `
)
`
	if err := os.WriteFile(filepath.Join(root, "zz_generated.bindings.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write generated bindings: %v", err)
	}

	return root
}

func newFakeCompiler(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binPath := filepath.Join(dir, "cc"+ext)

	sourcePath := filepath.Join(dir, "main.go")
	source := `package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func run(args []string) error {
	var outPath string
	var includeDir string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				return errors.New("missing value for -o")
			}
			outPath = args[i+1]
			i++
		case "-I":
			if i+1 >= len(args) {
				return errors.New("missing value for -I")
			}
			includeDir = args[i+1]
			i++
		}
	}

	if outPath == "" {
		return errors.New("missing -o")
	}
	if includeDir == "" {
		return errors.New("missing -I")
	}

	headerPath := filepath.Join(includeDir, "miniaudio.h")
	header, err := os.Open(headerPath)
	if err != nil {
		return err
	}
	defer header.Close()

	headerCopy, err := os.Create(outPath + ".header")
	if err != nil {
		return err
	}
	if _, err := io.Copy(headerCopy, header); err != nil {
		_ = headerCopy.Close()
		return err
	}
	if err := headerCopy.Close(); err != nil {
		return err
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	return outFile.Close()
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write fake compiler source: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", binPath, sourcePath)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake compiler: %v\n%s", err, output)
	}

	return dir
}
