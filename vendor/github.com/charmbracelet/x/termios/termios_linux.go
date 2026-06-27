//go:build linux
// +build linux

package termios

import "golang.org/x/sys/unix"

const (
	ioctlGets       = unix.TCGETS
	ioctlSets       = unix.TCSETS
	ioctlGetWinSize = unix.TIOCGWINSZ
	ioctlSetWinSize = unix.TIOCSWINSZ
)
