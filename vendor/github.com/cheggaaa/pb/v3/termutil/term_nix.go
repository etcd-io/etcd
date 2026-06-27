//go:build (linux || darwin || freebsd || netbsd || openbsd || dragonfly) && !appengine
// +build linux darwin freebsd netbsd openbsd dragonfly
// +build !appengine

package termutil

import "syscall"

const sysIoctl = syscall.SYS_IOCTL
