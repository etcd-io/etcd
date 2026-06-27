//go:build !darwin && !linux && !freebsd && !solaris && !aix && !windows
// +build !darwin,!linux,!freebsd,!solaris,!aix,!windows

package uv

func supportsHardTabs(uint64) bool {
	return false
}
