# mago

`mago` is a Go wrapper around [`miniaudio.h`](https://miniaud.io/) that avoids Go-side CGO.

Instead of compiling C into your Go package, `mago` expects `miniaudio` to be built as a standalone shared library and then loads that library at runtime with [`purego`](https://github.com/ebitengine/purego).

The Go wrapper currently supports the same platforms this package is wired up to use with `purego`:

- Linux
- macOS
- Windows
- FreeBSD
- NetBSD

OpenBSD and other targets are intentionally skipped here unless `purego` support exists for them.


## License

Like `miniaudio` itself, this project is intended to be available under a **public domain / MIT dual-license model**.

- You may treat it as public domain where that is recognized.
- Where public domain dedication is not recognized or not desired, you may use it under the MIT license instead.

`miniaudio.h` itself follows that same licensing model, and this project is designed to mirror it.


## What this library is

`mago` gives you a way to use `miniaudio` from Go without requiring CGO in your Go build.

The basic model is:

1. Build the native bridge as a shared library.
2. Ship that shared library with your application.
3. Load it from Go with `mago.Open()`.
4. Use the Go wrapper for version checks, context creation, device enumeration, and playback callbacks.

Important runtime rule:

- **Building the shared library is not required at application build time.**
- **Providing the correct shared library artifact at runtime is required.**

In other words, your Go binary does not need CGO, but it does need access to the right dynamic library:

- Linux: `libminiaudio.so`
- macOS: `libminiaudio.dylib`
- Windows: `miniaudio.dll`
- FreeBSD / NetBSD: `libminiaudio.so`


## Strict version matching

`miniaudio` does not promise backward compatibility, so `mago` performs a **strict runtime version check** when opening the library.

The loaded shared library must report the exact `miniaudio` version vendored by this repository. Right now that is:

- `0.11.25`

If the loaded library version does not match exactly, `mago.Open()` fails immediately.


## Current features

The current implementation includes:

- runtime loading through `purego`
- strict `miniaudio` version validation
- context creation
- backend device enumeration
- playback device creation
- callback-based audio output
- a higher-level `audio` subpackage for ergonomic stream playback
- a `speaker` subpackage compatible with `gopxl/beep/speaker`

Examples included in this repo:

- `examples/null-playback` — plays silence through the null backend and proves callback flow
- `examples/list-devices` — enumerates devices for common backends
- `examples/tones` — generates tones and plays them through a real device
- `examples/audio-wav` — uses the higher-level `audio` package with an in-memory WAV clip


## How to use it

### 1. Provide the shared library

By default, `mago` looks for the platform-appropriate runtime library name in one of these places:

- `native/<platform-library-name>` under the current working directory
- `<platform-library-name>` under the current working directory
- `native/<platform-library-name>` next to the executable
- `<platform-library-name>` next to the executable

You can also override the path explicitly:

```go
lib, err := mago.Open(mago.WithLibraryPath("/absolute/path/to/runtime-library"))
```

Or via environment variable:

```bash
export MAGO_MINIAUDIO_LIB=/absolute/path/to/runtime-library
```


### 2. Open the library and create a context

```go
lib, err := mago.Open()
if err != nil {
	panic(err)
}
defer lib.Close()

ctx, err := lib.NewContext(mago.BackendNull)
if err != nil {
	panic(err)
}
defer ctx.Close()
```


### 3. Enumerate devices

```go
playback, capture, err := ctx.Devices()
if err != nil {
	panic(err)
}

for i, device := range playback {
	fmt.Printf("[%d] %s (default=%v)\n", i, device.Name, device.IsDefault)
}
```


### 4. Play audio via callback

```go
config := mago.DefaultPlaybackDeviceConfig()
config.DeviceIndex = 0
config.Channels = 2
config.SampleRate = 48000
config.PeriodSizeInFrames = 256
config.DataCallback = func(device *mago.Device, output unsafe.Pointer, input unsafe.Pointer, frameCount uint32) {
	samples := unsafe.Slice((*float32)(output), int(frameCount*config.Channels))
	for i := range samples {
		samples[i] = 0
	}
}

device, err := ctx.NewPlaybackDevice(config)
if err != nil {
	panic(err)
}
defer device.Close()

if err := device.Start(); err != nil {
	panic(err)
}
```


## Higher-level `audio` package

If you want a more idiomatic playback API, use `github.com/darkliquid/mago/audio`.

The `audio` subpackage currently provides:

- loading WAV streams from `io.Reader` and `io.ReadSeeker`
- stream start / pause / resume / stop / close
- looping
- release of loaded clip buffers
- volume control
- playback speed adjustment
- reverse playback
- software resampling during playback
- crossfades between streams

This makes it the recommended entry point if you want application-facing playback code instead of low-level callback wiring.

Example:

```go
engine, err := audio.Open(audio.DefaultConfig())
if err != nil {
	panic(err)
}
defer engine.Close()

clip, err := engine.Load(reader)
if err != nil {
	panic(err)
}
defer clip.Release()

stream, err := engine.Play(clip, audio.DefaultPlayOptions())
if err != nil {
	panic(err)
}
defer stream.Close()

stream.SetLooping(true)
stream.SetVolume(0.5)
stream.SetSpeed(1.25)
stream.SetReverse(false)
```

Notes:

- `audio.Open()` starts the playback device automatically.
- The initial loader supports WAV streams; higher-level format support can be added on top later.
- The included `examples/audio-wav` demo shows loading a WAV stream from memory and changing speed / direction / fades at runtime.


## `beep`-compatible `speaker` package

If you already have `github.com/gopxl/beep` streamers, use `github.com/darkliquid/mago/speaker`.

It mirrors the `beep/speaker` API closely:

- `speaker.Init`
- `speaker.Play`
- `speaker.PlayAndWait`
- `speaker.Lock` / `speaker.Unlock`
- `speaker.Clear`
- `speaker.Suspend` / `speaker.Resume`
- `speaker.Close`

Example:

```go
if err := speaker.Init(beep.SampleRate(48_000), 512); err != nil {
	panic(err)
}
defer speaker.Close()

speaker.Play(beep.StreamerFunc(func(samples [][2]float64) (int, bool) {
	for i := range samples {
		samples[i][0] = 0
		samples[i][1] = 0
	}
	return len(samples), true
}))
```

Notes:

- Playback is backed by `mago`'s miniaudio callback rather than `oto`.
- `Suspend` and `Resume` map to stopping and starting the underlying `mago.Device`.


## Demos

List devices:

```bash
go run ./examples/list-devices
```

Play tones on a real device:

```bash
go run ./examples/tones
```

Use the higher-level `audio` package demo:

```bash
go run ./examples/audio-wav
```

You can override the backend/device selection. The accepted backend values are platform-dependent:

```bash
go run ./examples/tones --backend pulse --device-index 0 --duration 3s
go run ./examples/tones --backend coreaudio
go run ./examples/tones --backend wasapi
```

Or with environment variables:

```bash
MAGO_BACKEND=pulse MAGO_DEVICE_INDEX=0 MAGO_TONE_DURATION=2s go run ./examples/tones
MAGO_BACKEND=coreaudio go run ./examples/tones
MAGO_BACKEND=wasapi go run ./examples/tones
```

Supported demo overrides:

- `--backend` / `MAGO_BACKEND`
- `--device-index` / `MAGO_DEVICE_INDEX`
- `--device-name` / `MAGO_DEVICE_NAME`
- `--duration` / `MAGO_TONE_DURATION`


## Building the shared library

### Recommended builder

The repository includes a Go-native builder that chooses the right output name and compiler flags for the current host platform:

```bash
go run ./internal/cmd/buildlib
```

There is also a convenience wrapper:

```bash
bash native/build.sh
```

That produces a platform-appropriate runtime library in `native/`, for example:

```text
native/libminiaudio.so
```

Equivalent manual command:

```bash
cc -std=c11 -O2 -fPIC -shared \
  -Wl,-soname,libminiaudio.so \
  -o native/libminiaudio.so \
  native/miniaudio_bridge.c \
  -ldl -lm -lpthread
```


### Linux

Manual Linux command:

```bash
cc -std=c11 -O2 -fPIC -shared \
  -Wl,-soname,libminiaudio.so \
  -o native/libminiaudio.so \
  native/miniaudio_bridge.c \
  -ldl -lm -lpthread
```


### macOS

You can build a `.dylib` with `clang`:

```bash
clang -std=c11 -O2 -fPIC -dynamiclib \
  -o native/libminiaudio.dylib \
  native/miniaudio_bridge.c \
  -framework CoreAudio \
  -framework AudioToolbox \
  -framework AudioUnit \
  -framework Foundation \
  -framework CoreFoundation \
  -framework CoreServices \
  -lm -lpthread
```

### Windows

You can build a `.dll` with MinGW-w64:

```bash
x86_64-w64-mingw32-gcc -std=c11 -O2 -shared \
  -o native/miniaudio.dll \
  native/miniaudio_bridge.c \
  -lwinmm -lole32 -luuid
```

Or from an MSYS2/MinGW shell:

```bash
gcc -std=c11 -O2 -shared \
  -o native/miniaudio.dll \
  native/miniaudio_bridge.c \
  -lwinmm -lole32 -luuid
```

### FreeBSD / NetBSD

Manual BSD command:

```bash
cc -std=c11 -O2 -fPIC -shared \
  -o native/libminiaudio.so \
  native/miniaudio_bridge.c \
  -lm -lpthread
```


### Cross-compiling shared libraries

`mago` itself avoids Go-side CGO, but the shared library is still native code, so cross-compiling the shared library requires an appropriate C toolchain for the target platform.

Typical examples:

- Linux -> Linux: system `cc` or `clang`
- Linux -> Windows: `x86_64-w64-mingw32-gcc`
- macOS -> macOS: `clang`

For Go package cross-compilation, note that `purego` requires a special flag for `freebsd` and `netbsd` when building with `CGO_ENABLED=0`:

```bash
GOOS=freebsd GOARCH=amd64 go build -gcflags=github.com/ebitengine/purego/internal/fakecgo=-std ./...
GOOS=netbsd  GOARCH=amd64 go build -gcflags=github.com/ebitengine/purego/internal/fakecgo=-std ./...
```


## Runtime deployment

The important deployment rule is:

- **Your Go binary can be built without CGO**
- **Your deployment still needs the correct shared library for the target OS/architecture**

For example:

- Linux deployment needs `libminiaudio.so`
- macOS deployment needs `libminiaudio.dylib`
- Windows deployment needs `miniaudio.dll`
- FreeBSD / NetBSD deployment needs `libminiaudio.so`

You can place that artifact next to the executable, under a known app directory, or point `mago` at it explicitly with `WithLibraryPath(...)` or `MAGO_MINIAUDIO_LIB`.


## Development notes

- The bridge source lives in `native/miniaudio_bridge.c`.
- Generated Go symbol bindings are produced with:

```bash
go generate ./...
```

- Tests:

```bash
go test ./...
```


## Status

This project is currently an early cross-platform wrapper focused on:

- playback
- device enumeration
- runtime dynamic loading
- real-device and null-backend demos

There is room to expand the wrapper surface substantially, but the current priority is to keep the ABI boundary explicit and safe.
