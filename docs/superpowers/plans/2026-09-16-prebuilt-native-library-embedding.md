# Prebuilt Native Library Embedding & Cross-Platform CI/CD Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the `mago` miniaudio bridge build system to cross-compile for all 7 supported targets using `zig cc` and `osxcross`, generate Go `embed_*.go` files containing native binary data as `[]byte` literals instead of `//go:embed`, and update CI/CD with verification caching.

**Architecture:** 
1. `internal/buildlib` orchestrates cross-compilation across 7 platforms using `zig cc` with target triples and `osxcross` Docker container for Darwin targets, applying path sanitization and symbol stripping.
2. `internal/gen/embeds` reads compiled binaries and generates `embed_<goos>_<goarch>.go` files containing escaped string literals converted to `[]byte`.
3. `mise.toml` defines `zig 0.16.0` and orchestration tasks (`build-lib`, `build-lib-all`, `generate`).
4. GitHub Actions CI/CD uses hash-based caching on `native/miniaudio_bridge.c` and `miniaudio.h` to skip redundant compilation/generation, running cross-platform verification and pure Go tests.

**Tech Stack:** Go 1.24+, Zig 0.16.0 (`zig cc`), Docker (`dockercross/osxcross`), GitHub Actions, mise.

---

### Task 1: Update mise and git configuration for Zig and library tracking

**Files:**
- Modify: `mise.toml`
- Modify: `.gitignore`

- [ ] **Step 1: Update `mise.toml` to declare zig and build tasks**

Add `zig = "0.16.0"` to `[tools]` and define the `build-lib-all` and updated `generate` tasks.

```toml
[tools]
go = "latest"
zig = "0.16.0"
golangci-lint = "latest"
"go:golang.org/x/vuln/cmd/govulncheck" = "latest"

[env]
CGO_ENABLED = 0
MA_VER = "0.11.25"

[tasks.test]
description = "Run Go tests"
run = "go test ./..."

[tasks.build]
depends = ["generate"]
description = "Build test"
run = "go build -trimpath ./..."
sources = ["**/*.go"]

[tasks.build-lib]
description = "Build the native miniaudio lib for current host or target"
run = "go run ./internal/cmd/buildlib"
sources = ["miniaudio.h", "native/miniaudio_bridge.c"]

[tasks.build-lib-all]
description = "Build the native miniaudio lib for all supported targets"
run = "go run ./internal/cmd/buildlib -all"
sources = ["miniaudio.h", "native/miniaudio_bridge.c"]

[tasks.generate]
hide = true
description = "Run go generate (internal task)"
run = "go generate ./..."
sources = ["**/*.go"]

[tasks.lint]
description = "Run Go linting (golangci-lint + govulncheck)"
run = """
#!/usr/bin/env bash
set -euo pipefail
golangci-lint run ./...
govulncheck ./...
"""
sources = ["**/*.go"]

[tasks.fmt]
description = "Format Go code"
run = "golangci-lint fmt ./..."
sources = ["**/*.go"]
```

- [ ] **Step 2: Update `.gitignore` and untrack 0-byte placeholder files**

Edit `.gitignore` to ignore `internal/lib/` binaries completely, removing the un-ignore exceptions:

```gitignore
# Native runtime artifacts
*.so
*.so.*
*.dylib
*.dll
*.lib
*.exp

# Local build outputs
bin/
dist/

# Test binaries and coverage output
*.test
*.out
coverage.*

# Miniaudio source files (fetched and generated)
miniaudio.c
miniaudio.h

# Native libraries in internal/lib/ are temporary build intermediates
internal/lib/
```

Remove the tracked placeholder files:
```bash
git rm -f internal/lib/*/*
```

- [ ] **Step 3: Verify mise tasks and git status**

Run: `mise tasks`
Expected: Output includes `build-lib`, `build-lib-all`, `generate`, `test`, `build`.

- [ ] **Step 4: Commit**

```bash
git add mise.toml .gitignore
git commit -m "chore: configure zig in mise and untrack internal/lib binaries"
```

---

### Task 2: Refactor `internal/buildlib` to support cross-compilation with Zig and osxcross

**Files:**
- Modify: `internal/buildlib/buildlib.go`
- Modify: `internal/buildlib/buildlib_test.go`
- Modify: `internal/cmd/buildlib/main.go`

- [ ] **Step 1: Write failing unit test for target definitions and compiler arguments**

In `internal/buildlib/buildlib_test.go`:

```go
package buildlib

import (
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
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/buildlib`
Expected: FAIL (`AllTargets`, `zigCompilerArgs` undefined)

- [ ] **Step 3: Implement multi-target compilation in `internal/buildlib/buildlib.go`**

Update `internal/buildlib/buildlib.go` to:
1. Define `Target` struct and `AllTargets() []Target`.
2. Define target resolution by `GOOS/GOARCH` string or host default.
3. Construct hardened `zig cc` flags (`-ffile-prefix-map`, `-fvisibility=hidden`, `-s`, `-fno-asynchronous-unwind-tables`, `-fno-ident`).
4. Implement `buildTargetZig(target, root, outPath, version)` and `buildTargetDarwin(target, root, outPath, version)`:
   - For Darwin: If on Darwin host, use `clang`. If on Linux host, invoke `docker run --rm -v ... dockercross/osxcross` (or configured via `OSXCROSS_IMAGE`), running `o64-clang` or `oa64-clang`.
5. Expose `Build(root, outPath, version string)`, `BuildTarget(root string, target Target, version string) error`, and `BuildAll(root, version string) error`.
6. Expose `DownloadHeader(root, version, dstPath string) error` for downloading `miniaudio.h`.

```go
package buildlib

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Target struct {
	GOOS     string
	GOARCH   string
	Filename string
	Triple   string
}

func (t Target) Key() string {
	return t.GOOS + "-" + t.GOARCH
}

func (t Target) String() string {
	return t.GOOS + "/" + t.GOARCH
}

func AllTargets() []Target {
	return []Target{
		{GOOS: "linux", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-linux-gnu"},
		{GOOS: "linux", GOARCH: "arm64", Filename: "libminiaudio.so", Triple: "aarch64-linux-gnu"},
		{GOOS: "windows", GOARCH: "amd64", Filename: "miniaudio.dll", Triple: "x86_64-windows-gnu"},
		{GOOS: "freebsd", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-freebsd"},
		{GOOS: "netbsd", GOARCH: "amd64", Filename: "libminiaudio.so", Triple: "x86_64-netbsd"},
		{GOOS: "darwin", GOARCH: "amd64", Filename: "libminiaudio.dylib", Triple: "x86_64-apple-darwin"},
		{GOOS: "darwin", GOARCH: "arm64", Filename: "libminiaudio.dylib", Triple: "arm64-apple-darwin"},
	}
}

func FindTarget(goos, goarch string) (Target, error) {
	for _, t := range AllTargets() {
		if t.GOOS == goos && t.GOARCH == goarch {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unsupported platform %s/%s", goos, goarch)
}

func CurrentTarget() (Target, error) {
	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := os.Getenv("GOARCH")
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return FindTarget(goos, goarch)
}

func DefaultLibraryFilename(goos string) string {
	switch goos {
	case "windows":
		return "miniaudio.dll"
	case "darwin":
		return "libminiaudio.dylib"
	default:
		return "libminiaudio.so"
	}
}

func DefaultOutputPath(root string) string {
	target, err := CurrentTarget()
	if err != nil {
		return filepath.Join(root, "internal", "lib", runtime.GOOS+"-"+runtime.GOARCH, DefaultLibraryFilename(runtime.GOOS))
	}
	return filepath.Join(root, "internal", "lib", target.Key(), target.Filename)
}

func ResolveMiniaudioVersion(root, version string) (string, error) {
	if strings.TrimSpace(version) != "" {
		return strings.TrimSpace(version), nil
	}
	return defaultMiniaudioVersion(root)
}

var (
	httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}
	miniaudioHeaderURL = func(version string) string {
		return "https://raw.githubusercontent.com/mackron/miniaudio/" + url.PathEscape(version) + "/miniaudio.h"
	}
	versionConstPattern = regexp.MustCompile(`ExpectedMiniaudioVersion(Major|Minor|Revision)\s+uint32\s*=\s*(\d+)`)
)

func DownloadMiniaudioHeader(version, dstPath string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("miniaudio version must not be empty")
	}

	req, err := http.NewRequest(http.MethodGet, miniaudioHeaderURL(version), nil)
	if err != nil {
		return fmt.Errorf("create miniaudio header request: %w", err)
	}
	req.Header.Set("User-Agent", "mago-buildlib")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download miniaudio.h for version %s: %w", version, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download miniaudio.h for version %s: unexpected status %s", version, resp.Status)
	}

	tmpPath := dstPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temporary miniaudio header file: %w", err)
	}

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write miniaudio.h for version %s: %w", version, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temporary miniaudio header file: %w", err)
	}

	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install miniaudio.h for version %s: %w", version, err)
	}

	return nil
}

func defaultMiniaudioVersion(root string) (string, error) {
	content, err := os.ReadFile(filepath.Join(root, "zz_generated.bindings.go"))
	if err != nil {
		return "", fmt.Errorf("read generated bindings version constants: %w", err)
	}

	parts := map[string]string{}
	for _, match := range versionConstPattern.FindAllStringSubmatch(string(content), -1) {
		parts[match[1]] = match[2]
	}

	major, ok1 := parts["Major"]
	minor, ok2 := parts["Minor"]
	revision, ok3 := parts["Revision"]
	if !ok1 || !ok2 || !ok3 {
		return "", fmt.Errorf("find ExpectedMiniaudioVersion in generated bindings")
	}

	return major + "." + minor + "." + revision, nil
}

func zigCompilerArgs(target Target, outPath, source, includeDir, root string) []string {
	commonFlags := []string{
		"-target", target.Triple,
		"-std=c11", "-O2",
		"-fvisibility=hidden",
		"-fno-asynchronous-unwind-tables",
		"-fno-ident",
		"-ffile-prefix-map=" + root + "=.",
		"-ffile-prefix-map=" + includeDir + "=.",
		"-I", includeDir,
		"-s",
	}

	switch target.GOOS {
	case "linux":
		return append(commonFlags,
			"-fPIC", "-shared",
			"-Wl,-soname," + filepath.Base(outPath),
			"-o", outPath, source,
			"-ldl", "-lm", "-lpthread",
		)
	case "windows":
		return append(commonFlags,
			"-shared",
			"-o", outPath, source,
			"-lwinmm", "-lole32", "-luuid",
		)
	case "freebsd", "netbsd":
		return append(commonFlags,
			"-fPIC", "-shared",
			"-o", outPath, source,
			"-lm", "-lpthread",
		)
	default:
		return nil
	}
}

func BuildTarget(root string, target Target, version string) (err error) {
	resolvedVersion, err := ResolveMiniaudioVersion(root, version)
	if err != nil {
		return err
	}

	outPath := filepath.Join(root, "internal", "lib", target.Key(), target.Filename)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	includeDir, err := os.MkdirTemp("", "mago-buildlib-*")
	if err != nil {
		return fmt.Errorf("create temporary include directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(includeDir)
	}()

	headerPath := filepath.Join(includeDir, "miniaudio.h")
	if err := DownloadMiniaudioHeader(resolvedVersion, headerPath); err != nil {
		return err
	}

	source := filepath.Join(root, "native", "miniaudio_bridge.c")

	if target.GOOS == "darwin" {
		return buildDarwinTarget(target, root, outPath, source, includeDir)
	}

	compiler := os.Getenv("CC")
	var cmd *exec.Cmd
	if compiler != "" && !strings.Contains(compiler, "zig") {
		// Use custom compiler if explicitly specified
		args := []string{"-std=c11", "-O2", "-fvisibility=hidden", "-I", includeDir, "-o", outPath, source}
		cmd = exec.Command(compiler, args...)
	} else {
		// Default to zig cc
		args := append([]string{"cc"}, zigCompilerArgs(target, outPath, source, includeDir, root)...)
		cmd = exec.Command("zig", args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile %s with %s: %w", target, cmd.Path, err)
	}
	return nil
}

func buildDarwinTarget(target Target, root, outPath, source, includeDir string) error {
	if runtime.GOOS == "darwin" {
		// Native macOS compilation
		archFlag := "-arch " + target.GOARCH
		args := []string{
			archFlag, "-std=c11", "-O2", "-fPIC", "-dynamiclib",
			"-fvisibility=hidden",
			"-ffile-prefix-map=" + root + "=.",
			"-ffile-prefix-map=" + includeDir + "=.",
			"-I", includeDir,
			"-Wl,-x",
			"-o", outPath, source,
			"-framework", "CoreAudio",
			"-framework", "AudioToolbox",
			"-framework", "AudioUnit",
			"-framework", "Foundation",
			"-framework", "CoreFoundation",
			"-framework", "CoreServices",
			"-lm",
		}
		cmd := exec.Command("clang", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Cross-compilation on Linux via osxcross container
	image := os.Getenv("OSXCROSS_IMAGE")
	if image == "" {
		image = "dockercross/osxcross"
	}

	compiler := "o64-clang"
	if target.GOARCH == "arm64" {
		compiler = "oa64-clang"
	}

	args := []string{
		"run", "--rm",
		"-v", root + ":/workspace:ro",
		"-v", includeDir + ":/include:ro",
		"-v", filepath.Dir(outPath) + ":/out",
		image,
		compiler,
		"-std=c11", "-O2", "-fPIC", "-dynamiclib",
		"-fvisibility=hidden",
		"-ffile-prefix-map=/workspace=.",
		"-ffile-prefix-map=/include=.",
		"-I", "/include",
		"-Wl,-x",
		"-o", "/out/" + target.Filename,
		"/workspace/native/miniaudio_bridge.c",
		"-framework", "CoreAudio",
		"-framework", "AudioToolbox",
		"-framework", "AudioUnit",
		"-framework", "Foundation",
		"-framework", "CoreFoundation",
		"-framework", "CoreServices",
		"-lm",
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cross-compile darwin %s via docker %s: %w", target.GOARCH, image, err)
	}
	return nil
}

func BuildAll(root, version string) error {
	targets := AllTargets()
	for _, target := range targets {
		fmt.Printf("Building %s (%s)...\n", target, target.Filename)
		if err := BuildTarget(root, target, version); err != nil {
			return fmt.Errorf("build target %s: %w", target, err)
		}
	}
	return nil
}

func Build(root, outPath, version string) error {
	target, err := CurrentTarget()
	if err != nil {
		return err
	}
	if outPath == "" {
		outPath = DefaultOutputPath(root)
	}
	return BuildTarget(root, target, version)
}
```

- [ ] **Step 4: Update `internal/cmd/buildlib/main.go`**

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/mago/internal/buildlib"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		exit(err)
	}

	version := flag.String("version", "", "miniaudio version to download before building (defaults to the vendored version in zz_generated.bindings.go)")
	all := flag.Bool("all", false, "build all supported target platforms")
	targetFlag := flag.String("target", "", "build specific target (e.g. linux/amd64)")
	downloadOnly := flag.String("download-only", "", "download miniaudio.h to target destination path and exit")
	flag.Parse()

	if *downloadOnly != "" {
		ver, err := buildlib.ResolveMiniaudioVersion(root, *version)
		if err != nil {
			exit(err)
		}
		dst := *downloadOnly
		if !filepath.IsAbs(dst) {
			dst = filepath.Join(root, dst)
		}
		if err := buildlib.DownloadMiniaudioHeader(ver, dst); err != nil {
			exit(err)
		}
		fmt.Printf("Downloaded miniaudio.h (version %s) to %s\n", ver, dst)
		return
	}

	if *all {
		if err := buildlib.BuildAll(root, *version); err != nil {
			exit(err)
		}
		return
	}

	if *targetFlag != "" {
		parts := strings.Split(*targetFlag, "/")
		if len(parts) != 2 {
			exit(fmt.Errorf("invalid target format %q, expected os/arch (e.g. linux/amd64)", *targetFlag))
		}
		target, err := buildlib.FindTarget(parts[0], parts[1])
		if err != nil {
			exit(err)
		}
		if err := buildlib.BuildTarget(root, target, *version); err != nil {
			exit(err)
		}
		return
	}

	outPath := ""
	args := flag.Args()
	if len(args) == 1 {
		outPath = args[0]
		if !filepath.IsAbs(outPath) {
			outPath = filepath.Join(root, outPath)
		}
	}

	if err := buildlib.Build(root, outPath, *version); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v ./internal/buildlib`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/buildlib internal/cmd/buildlib
git commit -m "feat: add multi-target cross-compilation with zig and osxcross to buildlib"
```

---

### Task 3: Create the Embed Code Generator (`internal/gen/embeds`)

**Files:**
- Create: `internal/gen/embeds/main.go`
- Create: `internal/gen/embeds/embeds_test.go`
- Modify: `generate.go`

- [ ] **Step 1: Write test for the embed generator**

Create `internal/gen/embeds/embeds_test.go`:

```go
package main

import (
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
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/gen/embeds`
Expected: FAIL (types/functions not yet implemented)

- [ ] **Step 3: Implement `internal/gen/embeds/main.go`**

```go
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
)

type embedTarget struct {
	goos      string
	goarch    string
	varSuffix string
	libName   string
	fileName  string
}

var embedTargets = []embedTarget{
	{goos: "linux", goarch: "amd64", varSuffix: "LinuxAmd64", libName: "libminiaudio.so", fileName: "embed_linux_amd64.go"},
	{goos: "linux", goarch: "arm64", varSuffix: "LinuxArm64", libName: "libminiaudio.so", fileName: "embed_linux_arm64.go"},
	{goos: "windows", goarch: "amd64", varSuffix: "WindowsAmd64", libName: "miniaudio.dll", fileName: "embed_windows_amd64.go"},
	{goos: "freebsd", goarch: "amd64", varSuffix: "FreebsdAmd64", libName: "libminiaudio.so", fileName: "embed_freebsd_amd64.go"},
	{goos: "netbsd", goarch: "amd64", varSuffix: "NetbsdAmd64", libName: "libminiaudio.so", fileName: "embed_netbsd_amd64.go"},
	{goos: "darwin", goarch: "amd64", varSuffix: "DarwinAmd64", libName: "libminiaudio.dylib", fileName: "embed_darwin_amd64.go"},
	{goos: "darwin", goarch: "arm64", varSuffix: "DarwinArm64", libName: "libminiaudio.dylib", fileName: "embed_darwin_arm64.go"},
}

func formatByteString(data []byte) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, byteVal := range data {
		fmt.Fprintf(&b, "\\x%02x", byteVal)
	}
	b.WriteByte('"')
	return b.String()
}

func generateEmbedFile(root string, target embedTarget) error {
	libPath := filepath.Join(root, "internal", "lib", target.goos+"-"+target.goarch, target.libName)
	data, err := os.ReadFile(libPath)
	if err != nil {
		return fmt.Errorf("missing native library for %s/%s at %s (run 'mise run build-lib-all' first): %w", target.goos, target.goarch, libPath, err)
	}

	if len(data) == 0 {
		return fmt.Errorf("native library for %s/%s at %s is empty", target.goos, target.goarch, libPath)
	}

	var buf bytes.Buffer
	buf.WriteString("// Code generated by go generate; DO NOT EDIT.\n\n")
	fmt.Fprintf(&buf, "//go:build %s && %s\n\n", target.goos, target.goarch)
	buf.WriteString("package mago\n\n")

	fmt.Fprintf(&buf, "var embeddedLibData%s = []byte(\n", target.varSuffix)

	const chunkSize = 64
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		buf.WriteString("\t" + formatByteString(chunk))
		if end < len(data) {
			buf.WriteString(" +\n")
		} else {
			buf.WriteString("\n")
		}
	}
	buf.WriteString(")\n\n")

	buf.WriteString("func init() {\n")
	fmt.Fprintf(&buf, "\tembeddedLibData = embeddedLibData%s\n", target.varSuffix)
	fmt.Fprintf(&buf, "\tembeddedLibName = %q\n", target.libName)
	buf.WriteString("}\n")

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format generated Go code for %s: %w", target.fileName, err)
	}

	dstFile := filepath.Join(root, target.fileName)
	tmpFile := dstFile + ".tmp"
	if err := os.WriteFile(tmpFile, formatted, 0o644); err != nil {
		return fmt.Errorf("write temporary embed file: %w", err)
	}

	if err := os.Rename(tmpFile, dstFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename temporary embed file: %w", err)
	}

	fmt.Printf("Generated %s (%d bytes embedded)\n", target.fileName, len(data))
	return nil
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, target := range embedTargets {
		if err := generateEmbedFile(root, target); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}
```

- [ ] **Step 4: Update `generate.go` to invoke `internal/gen/embeds`**

```go
package mago

//go:generate go run ./internal/gen/bindings
//go:generate go run ./internal/gen/embeds
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v ./internal/gen/embeds`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/gen/embeds generate.go
git commit -m "feat: add embed code generator for platform byte slices"
```

---

### Task 4: Generate all platform embed files and update root embed files

**Files:**
- Modify: `embed_darwin_amd64.go`
- Modify: `embed_darwin_arm64.go`
- Modify: `embed_freebsd_amd64.go`
- Modify: `embed_linux_amd64.go`
- Modify: `embed_linux_arm64.go`
- Modify: `embed_netbsd_amd64.go`
- Modify: `embed_windows_amd64.go`

- [ ] **Step 1: Cross-compile non-Darwin native libraries and test generation**

Run:
```bash
go run ./internal/cmd/buildlib -target linux/amd64
go run ./internal/cmd/buildlib -target linux/arm64
go run ./internal/cmd/buildlib -target windows/amd64
go run ./internal/cmd/buildlib -target freebsd/amd64
go run ./internal/cmd/buildlib -target netbsd/amd64
```
Expected: All 5 `.so` and `.dll` files compiled successfully with `zig cc`.

- [ ] **Step 2: Cross-compile Darwin targets with osxcross (or docker/local)**

Run:
```bash
go run ./internal/cmd/buildlib -target darwin/amd64
go run ./internal/cmd/buildlib -target darwin/arm64
```
Expected: Both `.dylib` files compiled successfully into `internal/lib/darwin-*/`.

- [ ] **Step 3: Run `go run ./internal/gen/embeds` to update all 7 embed files**

Run: `go run ./internal/gen/embeds`
Expected: Output indicates `Generated embed_<goos>_<goarch>.go (NNN bytes embedded)` for all 7 platforms.

- [ ] **Step 4: Run Go tests and build examples**

Run:
```bash
go test -v ./...
go build -trimpath ./examples/...
```
Expected: PASS. The cache extraction and runtime tests pass using the newly embedded in-memory byte slices.

- [ ] **Step 5: Clean up `internal/lib` and verify build still succeeds**

Run:
```bash
rm -rf internal/lib
go test -v ./...
go build -trimpath ./examples/...
```
Expected: PASS. Verifies that `mago` compiles and tests with zero dependency on `internal/lib/` or any C compilers!

- [ ] **Step 6: Commit**

```bash
git add embed_*.go
git commit -m "feat: embed prebuilt native libraries as byte slices across all platforms"
```

---

### Task 5: Redesign GitHub Actions CI/CD with verification caching

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.github/workflows/update-lib.yml`

- [ ] **Step 1: Redesign `.github/workflows/ci.yml`**

Update `.github/workflows/ci.yml`:
1. Add `verify-embeds` job on `ubuntu-latest`:
   - Downloads `miniaudio.h` via `go run ./internal/cmd/buildlib -download-only miniaudio.h`.
   - Checks `actions/cache` keyed on `embeds-v1-${{ hashFiles('native/miniaudio_bridge.c', 'miniaudio.h') }}`.
   - If cache miss, sets up `mise` (Go + Zig), runs `mise run build-lib-all`, runs `mise run generate`, and runs `git diff --exit-code`.
   - Writes cache marker on success.
2. Update `test` matrix across `ubuntu-latest`, `macos-latest`, `windows-latest`:
   - Requires `verify-embeds` job.
   - Pure Go environment (no MSYS2, no local C compilers).
   - Runs `go test -v ./...` and `go build -trimpath ./examples/...`.

```yaml
name: CI

on:
  pull_request:
  push:
    branches:
      - main

permissions:
  contents: read

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up mise
        uses: jdx/mise-action@v4
        with:
          cache: true

      - name: Run lint checks
        run: mise run lint

  verify-embeds:
    name: Verify Native Libraries & Codegen
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up mise (Go)
        uses: jdx/mise-action@v4
        with:
          cache: true

      - name: Download miniaudio header for checksum
        run: go run ./internal/cmd/buildlib -download-only miniaudio.h

      - name: Check embed cache
        id: embed-cache
        uses: actions/cache@v4
        with:
          path: .buildlib-cache-marker
          key: embeds-v1-${{ hashFiles('native/miniaudio_bridge.c', 'miniaudio.h') }}

      - name: Cross-compile all libraries and verify embeds
        if: steps.embed-cache.outputs.cache-hit != 'true'
        run: |
          mise run build-lib-all
          mise run generate
          git diff --exit-code
          touch .buildlib-cache-marker

  test:
    name: Test (${{ matrix.os }})
    needs: [verify-embeds]
    runs-on: ${{ matrix.os }}
    timeout-minutes: 10
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up mise
        uses: jdx/mise-action@v4
        with:
          cache: true

      - name: Run tests (Pure Go consumer build)
        run: go test -v ./...

      - name: Build examples
        run: go build -trimpath ./examples/...
```

- [ ] **Step 2: Create `.github/workflows/update-lib.yml` for automated version bumps**

```yaml
name: Update Miniaudio

on:
  workflow_dispatch:
    inputs:
      version:
        description: "Miniaudio version to update to (e.g. 0.11.26)"
        required: true
        type: string

permissions:
  contents: write
  pull-requests: write

jobs:
  update-miniaudio:
    name: Update Miniaudio & Regenerate Embeds
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up mise
        uses: jdx/mise-action@v4
        with:
          cache: true

      - name: Build all native libraries
        run: mise run build-lib-all -version ${{ inputs.version }}

      - name: Generate bindings and embeds
        run: mise run generate

      - name: Create Pull Request
        uses: peter-evans/create-pull-request@v6
        with:
          commit-message: "chore: update miniaudio to ${{ inputs.version }}"
          title: "chore: update miniaudio to ${{ inputs.version }}"
          body: |
            Automated update of miniaudio to version `${{ inputs.version }}`.
            Rebuilt and re-embedded precompiled native libraries for all supported platforms.
          branch: "chore/update-miniaudio-${{ inputs.version }}"
          delete-branch: true
```

- [ ] **Step 3: Verify local test suite and lint**

Run:
```bash
mise run fmt
mise run lint
```
Expected: Clean formatting and no lint errors.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/update-lib.yml
git commit -m "ci: add cached embed verification and pure Go consumer testing"
```
