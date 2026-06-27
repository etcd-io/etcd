//go:build darwin || linux || freebsd || solaris || aix
// +build darwin linux freebsd solaris aix

package uv

import "golang.org/x/sys/unix"

func supportsHardTabs(oflag uint64) bool {
	return oflag&unix.TABDLY == unix.TAB0
}
