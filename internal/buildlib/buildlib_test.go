package buildlib

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	scriptPath := filepath.Join(dir, "cc")
	script := `#!/bin/sh
set -eu

outfile=""
includedir=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		-o)
			outfile="$2"
			shift 2
			;;
		-I)
			includedir="$2"
			shift 2
			;;
		*)
			shift
			;;
	esac
done

test -n "$outfile"
test -n "$includedir"
test -f "$includedir/miniaudio.h"

cp "$includedir/miniaudio.h" "$outfile.header"
: > "$outfile"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake compiler: %v", err)
	}

	return dir
}
