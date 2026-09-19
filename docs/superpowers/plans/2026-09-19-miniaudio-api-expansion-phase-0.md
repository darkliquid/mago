# Phase 0: Minimal-C Bridge Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce `native/miniaudio_bridge.c` to the absolute minimum required to drive miniaudio through purego, enforce zero CGO, and add the per-platform layout validation and lifecycle test infrastructure every later phase depends on.

**Architecture:** Go binds `ma_*` symbols directly and mirrors only the small struct prefixes it reads. C is restricted to three categories: raw allocation, by-value/large config construction, and callback trampolines for signatures that carry no `pUserData`. A test-time C probe validates every Go mirror against the vendored header.

**Tech Stack:** Go 1.26, [`purego`](https://github.com/ebitengine/purego) v0.10.0, `zig cc`, mise tasks, `bd` for tracking.

**Spec:** `docs/superpowers/specs/2026-09-19-miniaudio-api-expansion.md`

## Global Constraints

- **ZERO CGO.** No `import "C"`, no `//export`, no cgo build constraints, no `cgo_*.go` files. `CGO_ENABLED=0` always.
- **Minimal C.** C may only (1) allocate, (2) build a by-value/large config that Go must not mirror, (3) provide callback trampolines for signatures with no `pUserData`. Every other `mago_*` wrapper is forbidden; call `ma_*` directly.
- **Never hand-edit generated files.** `zz_generated.bindings.go` and `embed_*.go` are produced by `go generate`.
- **Native rebuild is a single batched step.** Tasks 1, 2, 4 and 5 do not change C. Only Task 3 does, and it ends with one `mise run build-lib-all && mise run generate` + commit of the `embed_*.go` files.
- **Tests never touch real hardware.** Use `mago.BackendNull` and `internal/testlib`.
- **Mirrors must be validated.** Any Go struct that mirrors miniaudio layout is asserted by the Task 2 probe.
- **Commit style:** conventional commits, imperative subject under 72 chars, body explaining why.

---

## Task 1: Zero-CGO guard

**Files:**
- Create: `internal/cgoguard/cgoguard.go`
- Create: `internal/cgoguard/cgoguard_test.go`
- Create: `internal/cgoguard/testdata/uses_import_c.go.txt`
- Create: `internal/cgoguard/testdata/uses_export.go.txt`
- Create: `internal/cgoguard/testdata/uses_cgo_tag.go.txt`
- Create: `internal/cgoguard/testdata/clean.go.txt`
- Modify: `mise.toml` (add `check-cgo` task)
- Modify: `AGENTS.md` (reference the guard under the hard rules)

**Interfaces:**
- Produces: `cgoguard.Find(root string) ([]string, error)` - returns repo-relative-ish paths (as walked) of Go files that use cgo.
- Produces: `cgoguard.FileUsesCgo(path string) (bool, error)` - used by tests with fixtures.

- [ ] **Step 1: Write the failing test**

```go
// internal/cgoguard/cgoguard_test.go
package cgoguard

import (
	"path/filepath"
	"testing"
)

func TestFileUsesCgo(t *testing.T) {
	cases := map[string]bool{
		"uses_import_c.go.txt": true,
		"uses_export.go.txt":   true,
		"uses_cgo_tag.go.txt":  true,
		"clean.go.txt":         false,
	}
	for name, want := range cases {
		got, err := FileUsesCgo(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestRepositoryHasNoCgo(t *testing.T) {
	found, err := Find(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("cgo usage detected in: %v", found)
	}
}
```

- [ ] **Step 2: Add the fixtures**

`internal/cgoguard/testdata/uses_import_c.go.txt`:

```go
package fixture

/*
#include <stdlib.h>
*/
import "C"

func F() {}
```

`internal/cgoguard/testdata/uses_export.go.txt`:

```go
package fixture

//export FixtureExport
func FixtureExport() {}
```

`internal/cgoguard/testdata/uses_cgo_tag.go.txt`:

```go
//go:build cgo

package fixture
```

`internal/cgoguard/testdata/clean.go.txt`:

```go
package fixture

func F() {}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/cgoguard/ -run TestFileUsesCgo -v`
Expected: FAIL - `undefined: FileUsesCgo`.

- [ ] **Step 4: Implement the guard**

```go
// internal/cgoguard/cgoguard.go
// Package cgoguard detects accidental cgo usage. mago is a zero-CGO project:
// the native library is a prebuilt runtime artifact loaded with purego.
package cgoguard

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	exportRe = regexp.MustCompile(`(?m)^//export\s+\w+`)
	cgoTagRe = regexp.MustCompile(`(?m)^//go:build[^\n]*\bcgo\b`)
)

// skipDirs are directories that never contain shipped Go source.
var skipDirs = map[string]bool{
	".git": true, ".beads": true, ".dolt": true,
	"testdata": true, "internal/lib": true,
}

// FileUsesCgo reports whether the Go file at path uses cgo.
func FileUsesCgo(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, parser.ImportsOnly)
	if err != nil {
		return false, err
	}
	for _, imp := range f.Imports {
		if imp.Path.Value == `"C"` {
			return true, nil
		}
	}

	return exportRe.Match(data) || cgoTagRe.Match(data), nil
}

// Find walks root and returns the paths of all Go files that use cgo.
func Find(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		uses, err := FileUsesCgo(path)
		if err != nil {
			return err
		}
		if uses {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}
```

Note: `skipDirs["internal/lib"]` never matches a single `DirEntry.Name()`; add an explicit path check for `internal/lib` in the directory branch by testing `strings.HasSuffix(filepath.ToSlash(path), "internal/lib")`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/cgoguard/ -v`
Expected: PASS for both tests (the repository currently has no cgo).

- [ ] **Step 6: Wire the guard into lint**

Add to `mise.toml`:

```toml
[tasks.check-cgo]
description = "Fail if any Go source uses cgo"
run = "go test ./internal/cgoguard/ -run TestRepositoryHasNoCgo"
sources = ["**/*.go"]
```

and add `depends = ["check-cgo"]` to the existing `[tasks.lint]` task.

- [ ] **Step 7: Document the rule**

In `AGENTS.md`, under **The ABI boundary**, add one sentence: cgo usage is
rejected by `internal/cgoguard` (run via `mise run check-cgo` / `mise run lint`).

- [ ] **Step 8: Verify and commit**

Run: `go test ./internal/cgoguard/ -v && go build ./...`
Expected: PASS, clean build.

```bash
git add internal/cgoguard mise.toml AGENTS.md
git commit -m "chore: guard against accidental cgo usage"
```

---

## Task 2: Layout probe harness

**Files:**
- Modify: `internal/buildlib/buildlib.go` (export `PrepareIncludeDir`)
- Create: `internal/abi/layout_probe.c`
- Create: `internal/abi/probe.go`
- Create: `internal/abi/probe_test.go`
- Create: `layout_test.go` (root package)

**Interfaces:**
- Consumes: `buildlib.PrepareIncludeDir(root, version string) (dir string, cleanup func(), err error)` - exported rename of the existing unexported helper.
- Produces: `abi.Run(root string) (map[string]uint64, error)` - returns probe keys to values, e.g. `"sizeof:ma_device_id"`, `"offsetof:ma_device_info.name"`.
- Produces: `abi.Parse(output string) (map[string]uint64, error)` - pure parser, unit-testable without a compiler.

- [ ] **Step 1: Export the include-dir helper**

In `internal/buildlib/buildlib.go`, rename `prepareIncludeDir` to
`PrepareIncludeDir` and update the three internal call sites
(`BuildTargetWithOutput`, `BuildAll`, and any test). Keep the signature
unchanged.

Run: `go build ./... && go test ./internal/buildlib/`
Expected: PASS.

- [ ] **Step 2: Add the C probe**

`internal/abi/layout_probe.c`:

```c
#include <stddef.h>
#include <stdio.h>

#include "miniaudio.h"

int main(void)
{
    printf("sizeof:ma_device_id %zu\n", sizeof(ma_device_id));
    printf("sizeof:ma_device_info %zu\n", sizeof(ma_device_info));
    printf("offsetof:ma_device_info.name %zu\n", offsetof(ma_device_info, name));
    printf("offsetof:ma_device_info.isDefault %zu\n", offsetof(ma_device_info, isDefault));
    printf("sizeof:ma_device_config %zu\n", sizeof(ma_device_config));
    printf("sizeof:ma_log %zu\n", sizeof(ma_log));
    return 0;
}
```

This file is test tooling, not a runtime shim. It is compiled against the same
vendored header the embedded library is built from.

- [ ] **Step 3: Write the failing parser test**

```go
// internal/abi/probe_test.go
package abi

import "testing"

func TestParse(t *testing.T) {
	out := "sizeof:ma_device_id 256\n" +
		"offsetof:ma_device_info.name 256\n" +
		"offsetof:ma_device_info.isDefault 512\n"

	got, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got["sizeof:ma_device_id"] != 256 {
		t.Errorf("sizeof ma_device_id = %d, want 256", got["sizeof:ma_device_id"])
	}
	if got["offsetof:ma_device_info.name"] != 256 {
		t.Errorf("offsetof name = %d, want 256", got["offsetof:ma_device_info.name"])
	}
	if got["offsetof:ma_device_info.isDefault"] != 512 {
		t.Errorf("offsetof isDefault = %d, want 512", got["offsetof:ma_device_info.isDefault"])
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	if _, err := Parse("garbage line without value\n"); err == nil {
		t.Fatal("expected error for malformed probe output")
	}
}
```

- [ ] **Step 4: Run to verify it fails**

Run: `go test ./internal/abi/ -run TestParse -v`
Expected: FAIL - `undefined: Parse`.

- [ ] **Step 5: Implement the probe runner**

```go
// internal/abi/probe.go
// Package abi validates that Go's mirrors of miniaudio structs match the
// vendored header, on the current platform. It is test tooling only and is
// never part of the shipped bridge.
package abi

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/darkliquid/mago/internal/buildlib"
)

// compilerOverride, when set, replaces "zig" with the given compiler. Tests use
// it to avoid requiring zig.
var compilerOverride string

// Parse reads probe output of the form "key value" per line.
func Parse(output string) (map[string]uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed probe line %q", line)
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("malformed probe value in %q: %w", line, err)
		}
		values[fields[0]] = value
	}
	return values, nil
}

// Run compiles and executes the layout probe against the vendored miniaudio.h
// in root and returns its measurements.
func Run(root string) (map[string]uint64, error) {
	dir, cleanup, err := buildlib.PrepareIncludeDir(root, "")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	tmp, err := os.MkdirTemp("", "mago-abi-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	source := filepath.Join(root, "internal", "abi", "layout_probe.c")
	probe := filepath.Join(tmp, "probe")
	if runtime.GOOS == "windows" {
		probe += ".exe"
	}

	compiler := "zig"
	if compilerOverride != "" {
		compiler = compilerOverride
	}
	args := []string{"cc", "-std=c11", "-I", dir, "-o", probe, source}
	build := exec.Command(compiler, args...)
	if out, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("compile layout probe: %w\n%s", err, out)
	}

	out, err := exec.Command(probe).Output()
	if err != nil {
		return nil, fmt.Errorf("run layout probe: %w", err)
	}
	return Parse(string(out))
}
```

- [ ] **Step 6: Run the parser tests**

Run: `go test ./internal/abi/ -run TestParse -v`
Expected: PASS.

- [ ] **Step 7: Add the root layout test**

```go
// layout_test.go
package mago

import (
	"testing"
	"unsafe"

	"github.com/darkliquid/mago/internal/abi"
)

func TestMirroredStructLayouts(t *testing.T) {
	probe, err := abi.Run(".")
	if err != nil {
		t.Fatalf("layout probe: %v", err)
	}

	if got, want := uint64(unsafe.Sizeof(deviceIDNative{})), probe["sizeof:ma_device_id"]; got != want {
		t.Errorf("sizeof ma_device_id: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.Name)), probe["offsetof:ma_device_info.name"]; got != want {
		t.Errorf("offsetof ma_device_info.name: mirror %d, header %d", got, want)
	}
	if got, want := uint64(unsafe.Offsetof(deviceInfoNative{}.IsDefault)), probe["offsetof:ma_device_info.isDefault"]; got != want {
		t.Errorf("offsetof ma_device_info.isDefault: mirror %d, header %d", got, want)
	}
}
```

The `deviceInfoNative` prefix mirror is introduced in Task 3. Until then this
test cannot compile, so **do Task 3's mirror definition first if executing
tasks out of order**. Tasks 2 and 3 are interlocked: Task 2 provides the harness,
Task 3 provides the mirror it validates.

- [ ] **Step 8: Run the probe test (requires zig)**

Run: `go test . -run TestMirroredStructLayouts -v`
Expected: PASS once Task 3 defines the mirror; FAIL with a clear diff if the
mirror drifts.

- [ ] **Step 9: Commit**

```bash
git add internal/buildlib/buildlib.go internal/abi layout_test.go
git commit -m "test: validate Go struct mirrors against the vendored miniaudio header"
```

---

## Task 3: Shrink the bridge to the minimal surface (single native rebuild)

**Files:**
- Modify: `native/miniaudio_bridge.c`
- Modify: `internal/gen/bindings/main.go`
- Regenerate: `zz_generated.bindings.go`, `embed_*.go`
- Modify: `library_supported.go`
- Modify: `context_supported.go`
- Modify: `device_supported.go`
- Modify: `types.go`
- Modify: `unsupported.go` (keep the stub API compiling)
- Modify: `layout_test.go` (mirror now exists)

**Interfaces:**
- Produces C: `void* mago_alloc(int type)`, `void mago_free(void* p)`.
- Produces C: unchanged `ma_result mago_device_init_playback(ma_context*, const mago_playback_device_config*, ma_device**)`.
- Produces C: unchanged `void mago_device_uninit_free(ma_device*)` (also frees the callback bridge).
- Removes C: `mago_context_init_default`, `mago_context_init_with_backends`, `mago_context_uninit_free`, `mago_context_get_devices`, `mago_context_free_device_infos`, and the `mago_device_info` struct.
- Produces Go: `deviceIDNative` / `deviceInfoNative` prefix mirrors; `magoObject` constants; `magoAlloc`, `magoFree` bindings; `maContextInit`, `maContextUninit`, `maContextEnumerateDevices`, `maLog*` bindings.

- [ ] **Step 1: Write the C changes**

Target `native/miniaudio_bridge.c` layout:

1. Header comment stating the three allowed categories and classifying each
   remaining function.
2. Object type enum shared with Go:

```c
enum mago_object_type
{
    MAGO_OBJECT_CONTEXT = 1,
    MAGO_OBJECT_DEVICE  = 2,
    MAGO_OBJECT_LOG     = 3
};

MAGO_API void* mago_alloc(int type)
{
    switch (type)
    {
        case MAGO_OBJECT_CONTEXT: return calloc(1, sizeof(ma_context));
        case MAGO_OBJECT_DEVICE:  return calloc(1, sizeof(ma_device));
        case MAGO_OBJECT_LOG:     return calloc(1, sizeof(ma_log));
        default:                  return NULL;
    }
}

MAGO_API void mago_free(void* p)
{
    free(p);
}
```

3. Keep `mago_playback_device_config`, `mago_device_bridge`, the two
   `mago_on_device_*` trampolines, `mago_device_init_playback` and
   `mago_device_uninit_free`, each with a one-line justification comment.
4. Delete everything else (context wrappers and `mago_device_info`).

- [ ] **Step 2: Add the log by-value shims (still Task 3, same rebuild)**

```c
MAGO_API ma_result mago_log_register_callback(ma_log* pLog, uintptr_t onLog, uintptr_t userData)
{
    ma_log_callback callback = ma_log_callback_init((ma_log_callback_proc)onLog, (void*)userData);
    return ma_log_register_callback(pLog, callback);
}

MAGO_API ma_result mago_log_unregister_callback(ma_log* pLog, uintptr_t onLog, uintptr_t userData)
{
    ma_log_callback callback = ma_log_callback_init((ma_log_callback_proc)onLog, (void*)userData);
    return ma_log_unregister_callback(pLog, callback);
}
```

`userData` is `uintptr_t` rather than `void*` so the Go side passes the token
without an `unsafe.Pointer(uintptr)` conversion.

These exist because `ma_log_callback` is returned and passed by value; they are
category (2) in the spec.

- [ ] **Step 3: Extend the bindings generator**

Add to `functions` in `internal/gen/bindings/main.go`:

```go
{FieldName: "maContextInit", Symbol: "ma_context_init", Type: "func(*Backend, uint32, unsafe.Pointer, *contextHandle) Result"},
{FieldName: "maContextUninit", Symbol: "ma_context_uninit", Type: "func(*contextHandle)"},
{FieldName: "maContextEnumerateDevices", Symbol: "ma_context_enumerate_devices", Type: "func(*contextHandle, uintptr, uintptr) Result"},
{FieldName: "maLogInit", Symbol: "ma_log_init", Type: "func(unsafe.Pointer, *logHandle) Result"},
{FieldName: "maLogUninit", Symbol: "ma_log_uninit", Type: "func(*logHandle)"},
{FieldName: "maLogPost", Symbol: "ma_log_post", Type: "func(*logHandle, uint32, string) Result"},
{FieldName: "maLogLevelToString", Symbol: "ma_log_level_to_string", Type: "func(uint32) string"},
{FieldName: "magoAlloc", Symbol: "mago_alloc", Type: "func(int32) unsafe.Pointer"},
{FieldName: "magoFree", Symbol: "mago_free", Type: "func(unsafe.Pointer)"},
{FieldName: "magoLogRegisterCallback", Symbol: "mago_log_register_callback", Type: "func(*logHandle, uintptr, uintptr) Result"},
{FieldName: "magoLogUnregisterCallback", Symbol: "mago_log_unregister_callback", Type: "func(*logHandle, uintptr, uintptr) Result"},
```

Add `func() { var _ unsafe.Pointer }`-style use is already emitted; ensure the
generator still imports `unsafe`.

Run: `go run ./internal/gen/bindings`
Expected: `zz_generated.bindings.go` regenerated.

- [ ] **Step 4: Rewrite the Go call sites**

`types.go` - add the prefix mirror and object constants:

```go
type deviceIDNative [256]byte // ma_device_id union; validated by layout_test.go

type deviceInfoNative struct {
	ID        deviceIDNative
	Name      [256]byte // MA_MAX_DEVICE_NAME_LENGTH + 1
	IsDefault uint32    // ma_bool32
	// Trailing nativeDataFormatCount/nativeDataFormats[64] are intentionally
	// omitted: enumeration pushes one pointer per device, so stride is unused.
	// Offsets are validated by the Task 2 probe.
}

type logHandle struct{}

const (
	magoObjectContext = 1
	magoObjectDevice  = 2
	magoObjectLog     = 3
)
```

`context_supported.go` - allocate in Go, call miniaudio directly:

```go
func (lib *Library) NewContext(backends ...Backend) (*Context, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}

	handle := (*contextHandle)(lib.bindings.magoAlloc(magoObjectContext))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate context: out of memory")
	}

	var result Result
	if len(backends) == 0 {
		result = lib.bindings.maContextInit(nil, 0, nil, handle)
	} else {
		count, err := intToUint32(len(backends))
		if err != nil {
			lib.bindings.magoFree(unsafe.Pointer(handle))
			return nil, fmt.Errorf("mago: %w", err)
		}
		result = lib.bindings.maContextInit(&backends[0], count, nil, handle)
		runtime.KeepAlive(backends)
	}
	if result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_context_init", result)
	}
	return &Context{lib: lib, handle: handle}, nil
}

func (ctx *Context) Close() error {
	if ctx == nil || ctx.handle == nil {
		return nil
	}
	if err := ctx.lib.ensureOpen(); err != nil {
		return err
	}
	ctx.lib.bindings.maContextUninit(ctx.handle)
	ctx.lib.bindings.magoFree(unsafe.Pointer(ctx.handle))
	ctx.handle = nil
	return nil
}
```

`Devices()` - direct enumeration with a purego callback:

```go
type deviceEnumerator struct {
	playback []DeviceInfo
	capture  []DeviceInfo
}

func copyDeviceInfo(native *deviceInfoNative) DeviceInfo {
	name := native.Name[:]
	if idx := bytes.IndexByte(name, 0); idx >= 0 {
		name = name[:idx]
	}
	return DeviceInfo{Name: string(name), IsDefault: native.IsDefault != 0}
}

func (ctx *Context) Devices() ([]DeviceInfo, []DeviceInfo, error) {
	if ctx == nil || ctx.handle == nil {
		return nil, nil, nil
	}
	if err := ctx.lib.ensureOpen(); err != nil {
		return nil, nil, err
	}

	collector := &deviceEnumerator{}
	token := uintptr(callbackSeq.Add(1))
	callbacks.Store(token, collector)
	defer callbacks.Delete(token)

	result := ctx.lib.bindings.maContextEnumerateDevices(ctx.handle, enumerateCallbackPtr, token)
	if result != Success {
		return nil, nil, ctx.lib.resultError("ma_context_enumerate_devices", result)
	}
	return collector.playback, collector.capture, nil
}
```

`device_supported.go` - add the enumeration trampoline next to the existing
device callbacks:

```go
var enumerateCallbackPtr = purego.NewCallback(func(_ uintptr, deviceType uint32, info uintptr, token uintptr) uintptr {
	value, ok := callbacks.Load(token)
	if !ok || info == 0 {
		return 1
	}
	collector := value.(*deviceEnumerator)
	item := copyDeviceInfo((*deviceInfoNative)(unsafe.Pointer(info)))
	switch DeviceType(deviceType) {
	case DeviceTypePlayback:
		collector.playback = append(collector.playback, item)
	case DeviceTypeCapture:
		collector.capture = append(collector.capture, item)
	}
	return 1 // continue enumeration
})
```

`callbacks` and `callbackSeq` are shared with the device callbacks; keep the
single `sync.Map`.

`unsupported.go` - keep stubs compiling; add the new constants/mirrors only if
referenced by stubs. Verify `go build` for a non-supported GOOS:
`GOOS=plan9 go build ./...`.

- [ ] **Step 5: Build and test on the host**

Run: `mise run test`
Expected: the existing `TestVersionAndNullBackendPlayback` and
`TestCacheExtraction` pass unchanged.

- [ ] **Step 6: Rebuild native libraries and regenerate embeds**

Run: `mise run build-lib-all && mise run generate`
Expected: 7 targets build; `embed_*.go` regenerate.

- [ ] **Step 7: Verify no generated drift and commit**

Run: `git status --short`
Expected: modified `zz_generated.bindings.go`, `embed_*.go`, and the Go/C files.

```bash
git add native/miniaudio_bridge.c internal/gen/bindings/main.go zz_generated.bindings.go embed_*.go \
  types.go context_supported.go device_supported.go unsupported.go library_supported.go
git commit -m "refactor: reduce the C bridge to allocation, config and callback shims"
```

---

## Task 4: Expose `ma_log` via direct binding

**Files:**
- Create: `log_supported.go`
- Create: `log_test.go`
- Modify: `unsupported.go` (stub `LogCallback` etc.)
- Modify: `library_supported.go` (optional `Library.Log` accessor if bindings expose it)
- Modify: `doc.go` (mention logging)

**Interfaces:**
- Consumes: `magoAlloc`, `magoFree`, `maLogInit`, `maLogUninit`, `maLogPost`, `maLogLevelToString`, `magoLogRegisterCallback`, `magoLogUnregisterCallback` (Task 3).
- Produces: `type LogLevel uint32` with `LogLevelDebug = 0`, `LogLevelInfo = 1`, `LogLevelWarning = 2`, `LogLevelError = 3`.
- Produces: `type LogCallback func(level LogLevel, message string)`.
- Produces: `func (lib *Library) NewLog() (*Log, error)`.
- Produces: `func (l *Log) Close() error`.
- Produces: `func (l *Log) Register(fn LogCallback) (token uintptr, err error)`.
- Produces: `func (l *Log) Unregister(token uintptr) error`.
- Produces: `func (l *Log) Post(level LogLevel, message string) error`.
- Produces: `func (l *Log) LevelString(level LogLevel) string`.

- [ ] **Step 1: Write the failing test**

```go
// log_test.go
package mago

import (
	"sync"
	"testing"

	"github.com/darkliquid/mago/internal/testlib"
)

func TestLogCallbackReceivesPostedMessages(t *testing.T) {
	libPath := testlib.BuildRuntimeLibrary(t, ".")
	lib, err := Open(WithLibraryPath(libPath))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	defer func() {
		if err := lib.Close(); err != nil {
			t.Fatalf("close library: %v", err)
		}
	}()

	log, err := lib.NewLog()
	if err != nil {
		t.Fatalf("NewLog: %v", err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}()

	var mu sync.Mutex
	var got []string
	token, err := log.Register(func(level LogLevel, message string) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, message)
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	defer func() {
		if err := log.Unregister(token); err != nil {
			t.Errorf("Unregister: %v", err)
		}
	}()

	if err := log.Post(LogLevelInfo, "hello from mago"); err != nil {
		t.Fatalf("Post: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0] != "hello from mago" {
		t.Fatalf("got %v, want one message", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run TestLogCallbackReceivesPostedMessages -v`
Expected: FAIL - `lib.NewLog undefined` (and `undefined: LogLevel`).

- [ ] **Step 3: Implement the log API**

```go
// log_supported.go
//go:build darwin || freebsd || linux || netbsd || windows

package mago

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

type LogLevel uint32

const (
	LogLevelDebug   LogLevel = 0
	LogLevelInfo    LogLevel = 1
	LogLevelWarning LogLevel = 2
	LogLevelError   LogLevel = 3
)

type LogCallback func(level LogLevel, message string)

type logState struct{ fn LogCallback }

var (
	logCallbackSeq atomic.Uint64
	logCallbacks   sync.Map // token -> *logState
)

var logCallbackPtr = purego.NewCallback(func(token uintptr, level uint32, message uintptr) uintptr {
	value, ok := logCallbacks.Load(token)
	if !ok || message == 0 {
		return 0
	}
	if state := value.(*logState); state.fn != nil {
		state.fn(LogLevel(level), goCString(message))
	}
	return 0
})

// goCString copies a NUL-terminated C string into Go. miniaudio's log messages
// are always NUL-terminated.
func goCString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Pointer(ptr + uintptr(n))) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n))
}

type Log struct {
	lib    *Library
	handle *logHandle
}

func (lib *Library) NewLog() (*Log, error) {
	if err := lib.ensureOpen(); err != nil {
		return nil, err
	}
	handle := (*logHandle)(lib.bindings.magoAlloc(magoObjectLog))
	if handle == nil {
		return nil, fmt.Errorf("mago: allocate log: out of memory")
	}
	if result := lib.bindings.maLogInit(nil, handle); result != Success {
		lib.bindings.magoFree(unsafe.Pointer(handle))
		return nil, lib.resultError("ma_log_init", result)
	}
	return &Log{lib: lib, handle: handle}, nil
}

func (l *Log) Register(fn LogCallback) (uintptr, error) {
	if l == nil || l.handle == nil {
		return 0, fmt.Errorf("mago: nil log")
	}
	token := uintptr(logCallbackSeq.Add(1))
	logCallbacks.Store(token, &logState{fn: fn})
	if result := l.lib.bindings.magoLogRegisterCallback(l.handle, logCallbackPtr, token); result != Success {
		logCallbacks.Delete(token)
		return 0, l.lib.resultError("ma_log_register_callback", result)
	}
	return token, nil
}

func (l *Log) Unregister(token uintptr) error {
	if l == nil || l.handle == nil {
		return fmt.Errorf("mago: nil log")
	}
	result := l.lib.bindings.magoLogUnregisterCallback(l.handle, logCallbackPtr, token)
	logCallbacks.Delete(token)
	return l.lib.resultError("ma_log_unregister_callback", result)
}

func (l *Log) Post(level LogLevel, message string) error {
	if l == nil || l.handle == nil {
		return fmt.Errorf("mago: nil log")
	}
	return l.lib.resultError("ma_log_post", l.lib.bindings.maLogPost(l.handle, uint32(level), message))
}

func (l *Log) LevelString(level LogLevel) string {
	if l == nil || l.handle == nil {
		return ""
	}
	return l.lib.bindings.maLogLevelToString(uint32(level))
}

func (l *Log) Close() error {
	if l == nil || l.handle == nil {
		return nil
	}
	if err := l.lib.ensureOpen(); err != nil {
		return err
	}
	l.lib.bindings.maLogUninit(l.handle)
	l.lib.bindings.magoFree(unsafe.Pointer(l.handle))
	l.handle = nil
	return nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test . -run TestLogCallbackReceivesPostedMessages -v`
Expected: PASS.

- [ ] **Step 5: Add the unsupported-platform stubs**

In `unsupported.go`, add `LogLevel` (with the four constants), `LogCallback`,
`type Log struct{}`, and stubs `func (*Library) NewLog() (*Log, error)`,
`func (*Log) Close() error`, `func (*Log) Register(LogCallback) (uintptr, error)`,
`func (*Log) Unregister(uintptr) error`, `func (*Log) Post(LogLevel, string) error`,
and `func (*Log) LevelString(LogLevel) string`, all returning
`errUnsupportedPlatform` (or zero values) so non-supported GOOS still compiles.

Verify: `GOOS=plan9 go build ./...`.

- [ ] **Step 6: Full test and commit**

Run: `mise run test && mise run build`
Expected: PASS.

```bash
git add log_supported.go log_test.go unsupported.go doc.go
git commit -m "feat: surface miniaudio logging through a Go callback API"
```

---

## Task 5: Opaque-object lifecycle harness

**Files:**
- Create: `internal/testlib/lifecycle.go`
- Create: `lifecycle_test.go` (root package)
- Modify: `AGENTS.md` (point at the harness under testing)

**Interfaces:**
- Produces: `testlib.NewNullLibrary(t) *mago.Library` - builds the runtime library, opens it, and registers cleanup.
- Produces: `testlib.NewNullContext(t, lib) *mago.Context` - creates a null-backend context and registers cleanup.

- [ ] **Step 1: Write the failing test**

```go
// lifecycle_test.go
package mago

import (
	"testing"

	"github.com/darkliquid/mago/internal/testlib"
)

func TestDoubleCloseIsSafe(t *testing.T) {
	lib := testlib.NewNullLibrary(t)

	ctx := testlib.NewNullContext(t, lib)
	if err := ctx.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := ctx.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestUseAfterCloseReturnsError(t *testing.T) {
	lib := testlib.NewNullLibrary(t)
	if err := lib.Close(); err != nil {
		t.Fatalf("close library: %v", err)
	}
	if _, err := lib.NewContext(BackendNull); err == nil {
		t.Fatal("expected error creating a context on a closed library")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test . -run 'TestDoubleCloseIsSafe|TestUseAfterCloseReturnsError' -v`
Expected: FAIL - `undefined: testlib.NewNullLibrary`.

- [ ] **Step 3: Implement the helpers**

```go
// internal/testlib/lifecycle.go
package testlib

import (
	"testing"

	"github.com/darkliquid/mago"
)

// NewNullLibrary builds the runtime library, opens it, and closes it when the
// test finishes.
func NewNullLibrary(t testing.TB) *mago.Library {
	t.Helper()
	path := BuildRuntimeLibrary(t, "..") // adjust per package depth
	lib, err := mago.Open(mago.WithLibraryPath(path))
	if err != nil {
		t.Fatalf("open library: %v", err)
	}
	t.Cleanup(func() {
		if err := lib.Close(); err != nil {
			t.Errorf("close library: %v", err)
		}
	})
	return lib
}

// NewNullContext creates a null-backend context and closes it when the test
// finishes.
func NewNullContext(t testing.TB, lib *mago.Library) *mago.Context {
	t.Helper()
	ctx, err := lib.NewContext(mago.BackendNull)
	if err != nil {
		t.Fatalf("new context: %v", err)
	}
	t.Cleanup(func() {
		if err := ctx.Close(); err != nil {
			t.Errorf("close context: %v", err)
		}
	})
	return ctx
}
```

The repo root passed to `BuildRuntimeLibrary` differs by caller package. Give
the helper an explicit `root` parameter (e.g. `NewNullLibrary(t, ".")` from the
root package) rather than guessing with `..`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test . -run 'TestDoubleCloseIsSafe|TestUseAfterCloseReturnsError' -v`
Expected: PASS.

- [ ] **Step 5: Migrate the existing tests**

Update `mago_test.go` and `audio/integration_test.go` to build the library
through the new helper where it reduces duplication, keeping behavior identical.

Run: `mise run test`
Expected: PASS.

- [ ] **Step 6: Document and commit**

In `AGENTS.md`, under **Testing**, note that `internal/testlib` provides
`NewNullLibrary` / `NewNullContext` and that lifecycle behavior
(double-close, use-after-close) is covered by `lifecycle_test.go`.

```bash
git add internal/testlib/lifecycle.go lifecycle_test.go mago_test.go audio/integration_test.go AGENTS.md
git commit -m "test: add null-backend lifecycle helpers and use-after-close coverage"
```

---

## Phase 0 exit criteria

- `mise run check-cgo` (or `mise run lint`) fails if any cgo appears.
- `native/miniaudio_bridge.c` contains only the three justified categories.
- `TestRepositoryHasNoCgo`, `TestMirroredStructLayouts`,
  `TestLogCallbackReceivesPostedMessages`, `TestDoubleCloseIsSafe` and
  `TestUseAfterCloseReturnsError` pass on all three CI test OSes.
- `mise run build-lib-all && mise run generate` leaves `git diff --exit-code`
  clean after committing the regenerated embeds.
- `bd ready` shows Phase 1 (`mago-8a3.2`) unblocked once Phase 0 closes.
