//go:build linux && arm64

package mago

import _ "embed"

//go:embed internal/lib/linux-arm64/libminiaudio.so
var embeddedLibDataLinuxArm64 []byte

func init() {
	embeddedLibData = embeddedLibDataLinuxArm64
	embeddedLibName = "libminiaudio.so"
}
