//go:build freebsd && amd64

package mago

import _ "embed"

//go:embed internal/lib/freebsd-amd64/libminiaudio.so
var embeddedLibDataFreeBSDAmd64 []byte

func init() {
	embeddedLibData = embeddedLibDataFreeBSDAmd64
	embeddedLibName = "libminiaudio.so"
}
