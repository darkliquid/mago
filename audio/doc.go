// Package audio provides a higher-level playback engine on top of the low-level
// mago bindings.
//
// The package is intentionally ergonomic and "Go-shaped": audio clips are loaded
// from standard library interfaces such as io.Reader and io.ReadSeeker, streams are
// controlled through ordinary methods, and mixing is handled internally by the
// engine.
//
// The current design is pure-Go on the mixing side. Audio content is decoded into
// float32 PCM buffers and then mixed in the playback callback exposed by the base
// mago package. This keeps the public API simple while preserving the no-CGO runtime
// model.
//
// A typical workflow looks like this:
//
//  1. Open an Engine with Open(DefaultConfig()).
//  2. Load a clip from an io.Reader or io.ReadSeeker.
//  3. Start playback with Engine.Play().
//  4. Adjust looping, volume, speed, fading, or reverse playback on the returned Stream.
//  5. Release clip memory when the clip is no longer needed and close the engine on shutdown.
//
// Currently supported input format:
//
//   - WAV / RIFF streams
//   - PCM integer samples: 8-bit, 16-bit, 24-bit, 32-bit
//   - 32-bit floating-point WAV
//
// Currently supported playback features:
//
//   - start / pause / resume / stop
//   - looping
//   - release of loaded clip buffers
//   - software resampling
//   - volume control
//   - playback speed changes
//   - reverse playback
//   - stream fades and crossfades
//
// The engine starts the underlying playback device automatically when opened.
package audio
