//go:build (darwin || freebsd || netbsd || openbsd || dragonfly) && !appengine
// +build darwin freebsd netbsd openbsd dragonfly
// +build !appengine

package termutil

import "syscall"

const ioctlReadTermios = syscall.TIOCGETA
const ioctlWriteTermios = syscall.TIOCSETA
