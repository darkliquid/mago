//go:build linux && amd64

package mago

import _ "embed"

//go:embed internal/lib/linux-amd64/libminiaudio.so
var embeddedLibDataLinuxAmd64 []byte

func init() {
	embeddedLibData = embeddedLibDataLinuxAmd64
	embeddedLibName = "libminiaudio.so"
}
