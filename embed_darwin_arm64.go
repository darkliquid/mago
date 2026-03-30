//go:build darwin && arm64

package mago

import _ "embed"

//go:embed internal/lib/darwin-arm64/libminiaudio.dylib
var embeddedLibDataDarwinArm64 []byte

func init() {
	embeddedLibData = embeddedLibDataDarwinArm64
	embeddedLibName = "libminiaudio.dylib"
}
