// Package mago wraps a miniaudio shared object without using CGO at Go build time.
//
// Build the bundled native library with:
//
//	go run ./internal/cmd/buildlib
//
// The build helper downloads the matching miniaudio header automatically. Use
// `-version x.y.z` if you need to override the default version from the
// generated bindings.
//
// Supported platforms match the PureGo targets used by this package:
// darwin, freebsd, linux, netbsd, and windows.
//
// By default the loader searches for the platform-appropriate runtime library name
// in `native/` under the current working directory, next to the current executable,
// or at the path named by the `MAGO_MINIAUDIO_LIB` environment variable. Opening the
// library performs a strict version check and rejects any loaded `miniaudio` build
// whose reported version does not exactly match the vendored header version used by
// this package.
//
// miniaudio's own logging can be surfaced to Go with Library.NewLog and
// Log.Register.
//
// Above the device callback there are two mixing layers: NodeGraph for routing
// audio through nodes, and Engine for playing Sound and SoundGroup objects with
// groups, fades and 3D spatialization. With EngineConfig.NoDevice an engine is
// driven by Engine.Read instead of a device, so it needs no audio hardware.
package mago
