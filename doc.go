// Package mago wraps a Linux miniaudio shared object without using CGO at Go build time.
//
// Build the bundled native library with:
//
// bash native/build.sh
//
// By default the loader searches for `libminiaudio.so` in `native/` under the current
// working directory, next to the current executable, or at the path named by the
// `MAGO_MINIAUDIO_LIB` environment variable. Opening the library performs a strict
// version check and rejects any loaded `miniaudio` build whose reported version does
// not exactly match the vendored header version used by this package.
package mago
