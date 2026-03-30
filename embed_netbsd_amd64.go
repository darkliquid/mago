//go:build netbsd && amd64

package mago

import _ "embed"

//go:embed internal/lib/netbsd-amd64/libminiaudio.so
var embeddedLibDataNetBSDAmd64 []byte

func init() {
	embeddedLibData = embeddedLibDataNetBSDAmd64
	embeddedLibName = "libminiaudio.so"
}
