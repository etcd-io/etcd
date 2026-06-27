//go:build darwin
// +build darwin

package termios

import "syscall"

func init() {
	allLineOpts[IUTF8] = syscall.IUTF8
}
