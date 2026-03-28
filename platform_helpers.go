//go:build darwin || freebsd || linux || netbsd || windows

package mago

import "runtime"

func isDarwin() bool {
	return runtime.GOOS == "darwin"
}
