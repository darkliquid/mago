# AGENTS.md

Guidance for agents working in the `mago` repository.

## What this project is

`mago` is a pure-Go wrapper around `miniaudio.h` that deliberately avoids CGO at
Go build time. The C library is compiled into a standalone shared object (with
`zig cc`, or `clang`/`osxcross` for Darwin), that binary is embedded into the Go
package as a byte slice, extracted to a per-user cache directory at runtime, and
loaded with [`purego`](https://github.com/ebitengine/purego). Go code never links
C; it only resolves symbols at runtime.

Three layers:

1. **Root `mago` package** - low-level, explicit ABI: `Open()`/`Library`,
   `NewContext`, `Devices`, `NewPlaybackDevice`, `Device.Start/Stop/Close`, and
   callback registration. Platform-specific files: `*_supported.go` (build-tagged
   for the supported OSes) and `unsupported.go` (stub API for everything else so
   the package still compiles).
2. **`audio` subpackage** - higher-level software mixer/engine: WAV decoding,
   `Engine`/`Clip`/`Stream`, looping, volume, speed, reverse, fades, crossfades.
3. **`speaker` subpackage** - `gopxl/beep/speaker`-compatible API.

`examples/` contains runnable demos (`list-devices`, `tones`, `null-playback`,
`audio-wav`). `native/miniaudio_bridge.c` is the C ABI boundary. `internal/`
holds build/codegen tooling and the test helper, never part of the public API.

## Essential commands

Toolchain is managed by `mise` (`mise.toml`), which also exports
`CGO_ENABLED=0`:

```bash
mise run test        # go test ./...
mise run build       # depends on generate; go build -trimpath ./...
mise run lint        # golangci-lint run ./... && govulncheck ./...
mise run fmt         # golangci-lint fmt ./...
mise run generate    # go generate ./...  (bindings + embed generation)
mise run build-lib       # native lib for the current host
mise run build-lib-all   # cross-compile all 7 targets into internal/lib/
```

Direct Go equivalents work too (the important thing is `CGO_ENABLED=0`).

Run a single test, e.g. `go test -run TestVersionAndNullBackendPlayback ./`.

## Native library build pipeline

`go run ./internal/cmd/buildlib [flags]`:

- `-version x.y.z` overrides the miniaudio version; otherwise it is derived from
  `zz_generated.bindings.go` (or `miniaudio.h` if present).
- `-all` builds all targets; `-target os/arch` builds one; `-download-only path`
  only fetches `miniaudio.h`.
- Outputs go to `internal/lib/<goos>-<goarch>/<lib>` (gitignored).
- Non-Darwin targets are compiled with `zig cc` using `-target <triple>`.
- Darwin on macOS uses `clang`; Darwin on Linux/CI uses an osxcross Docker image
  (override with `OSXCROSS_IMAGE`, default `dockercross/osxcross`).

Supported targets (7): `linux/{amd64,arm64}`, `windows/amd64`,
`freebsd/amd64`, `netbsd/amd64`, `darwin/{amd64,arm64}`.

### Generated files and the CI embed check

`go generate ./...` runs two generators, both writing files at the repo root
with `DO NOT EDIT` headers:

- `internal/gen/bindings` → `zz_generated.bindings.go` (version constants,
  `Result`/`Backend`/... constant values, and the `bindingSet` struct whose
  `register` method binds each `purego.RegisterLibFunc` symbol).
- `internal/gen/embeds` → the tracked `embed_<goos>_<goarch>.go` files. It
  requires every built library under `internal/lib/` to exist and be non-empty,
  and fails with an actionable error telling you to run `mise run build-lib-all`.

The embed files are committed. CI (`verify-embeds`) rebuilds all targets,
regenerates, and runs `git diff --exit-code`, so **any change to
`native/miniaudio_bridge.c` or the miniaudio version must be followed by
`mise run build-lib-all && mise run generate` and committing the updated
`embed_*.go` files.** Editing generated files by hand is pointless; generated
code is excluded from linting via `generated: strict`.

## The ABI boundary

**Hard rules:** there is **absolutely no CGO** anywhere (no `import "C"`, no
cgo files, no `//export`; `CGO_ENABLED=0` always). cgo usage is rejected by
`internal/cgoguard` (`mise run check-cgo`, run as part of `mise run lint`).

`native/miniaudio_bridge.c` is the **absolute minimum** C required to drive
miniaudio through purego. C may only: (1) allocate raw memory Go cannot size
(`mago_alloc`/`mago_free`); (2) build a by-value, large config Go must not
mirror (the `ma_device_config` setter); (3) provide callback trampolines for C
signatures that carry no `pUserData` (device data/notification). No other
wrappers: bind `ma_*` symbols directly; every `mago_*` shim must be justified.

`native/miniaudio_bridge.c` defines `mago_*` wrapper structs and functions for
everything the Go side needs, and forwards device data/notification callbacks
through `uintptr_t` user data. Go mirrors the C structs exactly in `types.go`
(`playbackDeviceConfigNative`, `deviceInfoNative`); field order and widths must
match the C struct. Callbacks are implemented once with `purego.NewCallback` in
`device_supported.go`, looked up in a `sync.Map` by a monotonically increasing
token passed as `UserData`.

When adding a bridge symbol: add it to `functions` in
`internal/gen/bindings/main.go`, then re-run `mise run generate`; do not edit
`zz_generated.bindings.go` directly.

## Conventions and patterns

- **Platform tags**: shared implementations use
  `//go:build darwin || freebsd || linux || netbsd || windows`; each supported
  file has a matching stub branch in `unsupported.go` behind
  `//go:build !darwin && !freebsd && !linux && !netbsd && !windows` so the
  package still typechecks on unsupported platforms. `loader_unix.go` uses
  `purego.Dlopen`; `loader_windows.go` uses `syscall.LoadLibrary`.
- **Errors**: wrap miniaudio result codes in `OpError` via
  `lib.resultError(op, code)`; version mismatches use `VersionMismatchError`.
  All errors are prefixed `mago:`.
- **Lifetime safety**: methods call `lib.ensureOpen()` (RWMutex-guarded
  `closed` flag) before touching the handle, and nil receivers return clean
  errors rather than panicking.
- **Strict version check**: `Open()` rejects any loaded library whose
  `ma_version`/`ma_version_string` differs from the vendored
  `ExpectedMiniaudioVersion*` constants. This is intentional; miniaudio does not
  guarantee ABI compatibility.
- **Runtime library resolution order** (`resolveLibraryPath`): explicit
  `WithLibraryPath`, then `MAGO_MINIAUDIO_LIB`, then the embedded library
  extracted to `os.UserCacheDir()/mago/<version>/`, then a search for the
  platform library name under `./native`, `./`, and next to the executable.
- **Lint config** (`.golangci.yml`) enables `gosec`, `revive`, etc. `device_supported.go`
  is exempted from the `unsafe.Pointer` govet rule; test files are exempt from
  `gosec`. `goimports` formatting is enforced (tabs, stdlib-first import grouping).

## Testing

Tests that touch audio build a fresh native library at runtime via
`internal/testlib.BuildRuntimeLibrary(t, root)`, which calls `buildlib.Build`
into a temp dir. **This means the ordinary test suite requires `zig` on `PATH`
(or `clang` on macOS) to compile the C bridge**, even though the package itself
needs no CGO. The root test passes `.`; `audio` tests pass `..`.

- Prefer the `BackendNull` backend so tests are deterministic and need no real
  audio hardware.
- Callback delivery is asserted with buffered channels and `time.After`
  timeouts (see `mago_test.go`); don't add sleeps expecting synchronous
  delivery.
- `cache_test.go` redirects the cache via `XDG_CACHE_HOME`/`HOME`/`LOCALAPPDATA`
  and skips if the platform has no embedded data.
- Pure unit tests for the generators/buildlib live beside them (`internal/**`)
  and use a `compilerOverride` hook so they never invoke `zig`/Docker.

## Gotchas

- `miniaudio.h` is downloaded on demand and gitignored; it is usually **absent**
  from a fresh checkout. LSP/clangd diagnostics on
  `native/miniaudio_bridge.c` about unknown `ma_*` types are expected and not a
  real problem.
- `README.md` mentions `bash native/build.sh`; that script does **not** exist in
  this tree. Use `go run ./internal/cmd/buildlib` or `mise run build-lib`.
- `internal/lib/` and all compiled `*.so`/`*.dylib`/`*.dll` are gitignored.
  Only the generated embed `.go` files are tracked.
- Cross-compiling the Go package for FreeBSD/NetBSD with `CGO_ENABLED=0`
  requires extra gcflags for purego's fakecgo:
  `go build -gcflags=github.com/ebitengine/purego/internal/fakecgo=-std ./...`.
- `audio.Open()` starts the playback device automatically; the engine owns the
  device and mixer. Streams from the same `Clip` can be mixed concurrently.
- The miniaudio version lives in the generated bindings; bump it through
  `mise run build-lib-all -version x.y.z` + `mise run generate` (which is what
  `.github/workflows/update-lib.yml` automates, opening a PR). `MA_VER` in
  `mise.toml` is not read by the Go tooling.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:46cd31e7 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   bd dolt push
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->

<!-- BEGIN BEADS CODEX SETUP: generated by bd setup codex -->
## Beads Issue Tracker

Use Beads (`bd`) for durable task tracking in repositories that include it. Use the `beads` skill at `.agents/skills/beads/SKILL.md` (project install) or `~/.agents/skills/beads/SKILL.md` (global install) for Beads workflow guidance, then use the `bd` CLI for issue operations.

### Quick Reference

```bash
bd ready                # Find available work
bd show <id>            # View issue details
bd update <id> --claim  # Claim work
bd close <id>           # Complete work
bd prime                # Refresh Beads context
```

### Rules

- Use `bd` for all task tracking; do not create markdown TODO lists.
- Run `bd prime` when Beads context is missing or stale. Codex 0.129.0+ can load Beads context automatically through native hooks; use `/hooks` to inspect or toggle them.
- Keep persistent project memory in Beads via `bd remember`; do not create ad hoc memory files.

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/core-concepts/sync-concepts.md for details and anti-patterns.
<!-- END BEADS CODEX SETUP -->
