//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package uv

func (n *SizeNotifier) start() error {
	return ErrPlatformNotSupported
}

func (n *SizeNotifier) stop() error {
	return ErrPlatformNotSupported
}

func (n *SizeNotifier) getWindowSize() (cells Size, pixels Size, err error) {
	cells.Width, cells.Height, err = n.GetSize()
	return cells, pixels, err
}
