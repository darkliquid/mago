//go:build windows

package mago

import "syscall"

func openLibrary(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	return uintptr(handle), err
}

func closeLibrary(handle uintptr) error {
	return syscall.FreeLibrary(syscall.Handle(handle))
}

func defaultLibraryNames() []string {
	return []string{"miniaudio.dll", "libminiaudio.dll"}
}
