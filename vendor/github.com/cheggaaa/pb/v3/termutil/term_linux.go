//go:build linux && !appengine
// +build linux,!appengine

package termutil

const ioctlReadTermios = 0x5401  // syscall.TCGETS
const ioctlWriteTermios = 0x5402 // syscall.TCSETS
