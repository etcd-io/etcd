//go:build darwin || linux || aix
// +build darwin linux aix

package uv

import "golang.org/x/sys/unix"

func supportsBackspace(lflag uint64) bool {
	return lflag&unix.BSDLY == unix.BS0
}
