# Prebuilt Native Library Embedding & Cross-Platform CI/CD Design

## Context & Motivation
`mago` currently uses Go's `//go:embed` directive in platform-specific files (`embed_<goos>_<goarch>.go`) pointing to shared libraries located in `internal/lib/<platform>/`. In the original scheme:
1. Compilation only embedded the native library for the specific platform being built in CI/CD.
2. 0-byte dummy placeholder files were checked into git for other platforms to prevent `go build` from failing with missing file patterns.
3. Consequently, end users downloading the package on platforms other than the one built received empty binary data, causing dynamic loading to fail unless they cross-compiled the bridge library manually.

This design transitions `mago` to pre-generate Go source files (`embed_<goos>_<goarch>.go`) containing the compiled native libraries directly embedded as `[]byte` slice literals. All supported platform libraries are cross-compiled in CI/CD using `zig cc` and an `osxcross` Docker container, completely eliminating the need for `//go:embed` directives or for downstream users to possess any C compilers or cross-compilation toolchains.

---

## Architecture & Supported Targets

### Target Platforms
The build system supports 7 target platforms:

| Target Platform (`GOOS/GOARCH`) | Output Library | Toolchain / Compiler | Target Arguments / Settings |
| :--- | :--- | :--- | :--- |
| `linux/amd64` | `internal/lib/linux-amd64/libminiaudio.so` | `zig cc` | `-target x86_64-linux-gnu` |
| `linux/arm64` | `internal/lib/linux-arm64/libminiaudio.so` | `zig cc` | `-target aarch64-linux-gnu` |
| `windows/amd64` | `internal/lib/windows-amd64/miniaudio.dll` | `zig cc` | `-target x86_64-windows-gnu` |
| `freebsd/amd64` | `internal/lib/freebsd-amd64/libminiaudio.so` | `zig cc` | `-target x86_64-freebsd` |
| `netbsd/amd64` | `internal/lib/netbsd-amd64/libminiaudio.so` | `zig cc` | `-target x86_64-netbsd` |
| `darwin/amd64` | `internal/lib/darwin-amd64/libminiaudio.dylib` | `osxcross` (Docker / native) | `x86_64-apple-darwin` + CoreAudio/AudioToolbox frameworks |
| `darwin/arm64` | `internal/lib/darwin-arm64/libminiaudio.dylib` | `osxcross` (Docker / native) | `arm64-apple-darwin` + CoreAudio/AudioToolbox frameworks |

### Tooling Management (`mise.toml`)
`mise.toml` manages the development and CI dependencies:
* `go = "latest"`
* `zig = "0.16.0"`
* `golangci-lint = "latest"`
* `"go:golang.org/x/vuln/cmd/govulncheck" = "latest"`

Tasks in `mise.toml`:
* `build-lib`: Builds the native library for the current host (or platform targeted by `GOOS`/`GOARCH`).
* `build-lib-all`: Cross-compiles the native bridge for all 7 platforms into `internal/lib/`.
* `generate`: Runs `go generate ./...`, invoking bindings generation and embed generation.
* `build`: Runs `go build -trimpath ./...`.
* `test`: Runs `go test ./...`.

### Repository File Tracking
* `internal/lib/` is gitignored (`*.so`, `*.dylib`, `*.dll`). The 0-byte dummy placeholder files are removed from version control.
* Only the generated `embed_<goos>_<goarch>.go` files containing embedded byte slices are tracked in git.

---

## Component Details

### 1. Native Build Tool (`internal/buildlib` & `internal/cmd/buildlib`)
The `internal/buildlib` package is refactored to orchestrate compilation across all supported targets:

1. **Compiler Selection & Validation:**
   * For Linux, Windows, FreeBSD, and NetBSD: Discovers `zig` from `PATH` (or `$CC`) and executes `zig cc`.
   * For Darwin:
     * If running on macOS host: Uses native `clang` with Apple frameworks.
     * If running on Linux/CI: Executes an `osxcross` container (e.g. `dockercross/osxcross` or configured via `OSXCROSS_IMAGE`), mounting the workspace and include directory to invoke `o64-clang` and `oa64-clang`.
2. **Build Hardening & Stripping:**
   * Sanitizes all source and include file paths: `-ffile-prefix-map=<workspace>=.` and `-ffile-prefix-map=<includeDir>=.`.
   * Hides all symbols by default (`-fvisibility=hidden`), exposing only the bridge API marked with `MAGO_API`.
   * Strips all debug info, compiler identities, and symbols:
     * Zig / ELF / PE: `-s`, `-Wl,-s`, `-fno-asynchronous-unwind-tables`, `-fno-ident`.
     * Mach-O: `-Wl,-x` (strips non-global symbols, preserving dynamic exports for `dlsym`).
3. **CLI Interface (`internal/cmd/buildlib`):**
   * Flags:
     * `-target <os/arch>`: Compile single target.
     * `-all`: Compile all 7 targets sequentially into `internal/lib/<platform>/`.
     * `-version <x.y.z>`: Override miniaudio version (defaults to version constant in bindings).

### 2. Embed Code Generator (`internal/gen/embeds`)
A dedicated generator tool responsible for reading the compiled binaries and generating `embed_<goos>_<goarch>.go`:

1. **Validation:**
   * Ensures all 7 target binaries exist in `internal/lib/` and are non-empty (> 0 bytes). If missing, exits with an actionable error directing the developer/CI to run `mise run build-lib-all`.
2. **Code Generation Format:**
   * Replaces `//go:embed` directives and `_ "embed"` import.
   * Generates byte slices using formatted multi-line hex-escaped string literals (`[]byte("\x7f\x45...")` wrapped in ~64-byte chunks per line) to keep parser overhead minimal and binary ASTs clean.
   * Emits proper `//go:build <goos> && <goarch>` tags, `init()` hooks, and `embeddedLibName`.
   * Formats the generated source code with `go/format`.
3. **Invocation via `generate.go`:**
   ```go
   package mago

   //go:generate go run ./internal/gen/bindings
   //go:generate go run ./internal/gen/embeds
   ```

---

## CI/CD Pipeline (`.github/workflows/ci.yml`)

The GitHub Actions workflow is redesigned around two distinct phases:

### Phase 1: Build & Verification Job (`ubuntu-latest`)
1. Checks out repository.
2. Sets up `mise` with Go and Zig 0.16.0.
3. Runs `mise run build-lib-all` (using `zig cc` for 5 platforms and `osxcross` Docker container for Darwin).
4. Runs `mise run generate` (re-generating bindings and all 7 `embed_*.go` files).
5. Runs `git diff --exit-code`:
   * Fails the build if any committed `embed_*.go` file is missing, outdated, or desynchronized with the bridge source or miniaudio version.

### Phase 2: Consumer Validation Matrix (`ubuntu-latest`, `macos-latest`, `windows-latest`)
1. Checks out repository.
2. Sets up `mise` (Go only; no C compilers, MinGW, or MSYS2 needed).
3. Executes:
   * `go test -v ./...`
   * `go build -trimpath ./examples/...`
4. Validates that runtime extraction and dynamic symbol binding succeed out-of-the-box on every consumer operating system using pure Go.

---

## Maintenance Automation (`.github/workflows/update-lib.yml`)
A dispatch and scheduled maintenance workflow to manage miniaudio version bumps:
1. Accepts target miniaudio version.
2. Runs `mise run build-lib-all -version <ver>`.
3. Runs `mise run generate`.
4. Uses `peter-evans/create-pull-request` to open a PR with the updated bindings and embedded platform binaries.
