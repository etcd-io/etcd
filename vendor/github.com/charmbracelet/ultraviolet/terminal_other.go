//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !aix && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!aix,!windows

package uv

func (*Terminal) makeRaw() error {
	return ErrPlatformNotSupported
}

func (*Terminal) getSize() (int, int, error) {
	return 0, 0, ErrPlatformNotSupported
}

func (t *Terminal) optimizeMovements() {}

func (*Terminal) enableWindowsMouse() error  { return ErrPlatformNotSupported }
func (*Terminal) disableWindowsMouse() error { return ErrPlatformNotSupported }
