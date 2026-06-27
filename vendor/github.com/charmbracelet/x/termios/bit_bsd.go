//go:build netbsd || openbsd
// +build netbsd openbsd

package termios

func speed(b uint32) int32 { return int32(b) }
func bit(b uint32) uint32  { return b }
