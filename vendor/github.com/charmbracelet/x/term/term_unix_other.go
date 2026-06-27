//go:build aix || linux || solaris || zos
// +build aix linux solaris zos

package term

import "golang.org/x/sys/unix"

const (
	ioctlReadTermios  = unix.TCGETS
	ioctlWriteTermios = unix.TCSETS
)
