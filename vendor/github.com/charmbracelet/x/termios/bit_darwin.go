//go:build darwin
// +build darwin

package termios

func speed(b uint32) uint64 { return uint64(b) }
func bit(b uint32) uint64   { return uint64(b) }
