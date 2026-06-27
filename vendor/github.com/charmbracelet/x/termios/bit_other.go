//go:build !darwin && !netbsd && !openbsd
// +build !darwin,!netbsd,!openbsd

package termios

func speed(b uint32) uint32 { return b }
func bit(b uint32) uint32   { return b }
