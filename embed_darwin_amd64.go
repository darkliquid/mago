//go:build darwin && amd64

package mago

import _ "embed"

//go:embed internal/lib/darwin-amd64/libminiaudio.dylib
var embeddedLibDataDarwinAmd64 []byte

func init() {
	embeddedLibData = embeddedLibDataDarwinAmd64
	embeddedLibName = "libminiaudio.dylib"
}
