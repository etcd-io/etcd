//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !aix && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!aix,!windows

package uv

import "os"

func openTTY() (*os.File, *os.File, error) {
	return nil, nil, ErrPlatformNotSupported
}

func suspend() error {
	return ErrPlatformNotSupported
}
