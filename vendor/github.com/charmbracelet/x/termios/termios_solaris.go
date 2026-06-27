//go:build solaris
// +build solaris

package termios

import "golang.org/x/sys/unix"

// see https://src.illumos.org/source/xref/illumos-gate/usr/src/lib/libc/port/gen/isatty.c
// see https://github.com/omniti-labs/illumos-omnios/blob/master/usr/src/uts/common/sys/termios.h
const (
	ioctlSets       = unix.TCSETA
	ioctlGets       = unix.TCGETA
	ioctlSetWinSize = (int('T') << 8) | 103
	ioctlGetWinSize = (int('T') << 8) | 104
)

func setSpeed(*unix.Termios, uint32, uint32) {
	// TODO: support setting speed on Solaris?
	// see cfgetospeed(3C) and cfsetospeed(3C)
	// see cfgetispeed(3C) and cfsetispeed(3C)
	// https://github.com/omniti-labs/illumos-omnios/blob/master/usr/src/uts/common/sys/termios.h#L103
}

func getSpeed(*unix.Termios) (uint32, uint32) {
	return 0, 0
}
