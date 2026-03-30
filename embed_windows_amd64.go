//go:build windows && amd64

package mago

import _ "embed"

//go:embed internal/lib/windows-amd64/miniaudio.dll
var embeddedLibDataWindowsAmd64 []byte

func init() {
	embeddedLibData = embeddedLibDataWindowsAmd64
	embeddedLibName = "miniaudio.dll"
}
