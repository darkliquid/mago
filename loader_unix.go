//go:build darwin || freebsd || linux || netbsd

package mago

import "github.com/ebitengine/purego"

func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func closeLibrary(handle uintptr) error {
	return purego.Dlclose(handle)
}

func defaultLibraryNames() []string {
	if isDarwin() {
		return []string{"libminiaudio.dylib"}
	}
	return []string{"libminiaudio.so"}
}
