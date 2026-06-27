//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix
// +build darwin dragonfly freebsd linux netbsd openbsd solaris aix

package uv

import (
	"os"
	"os/signal"
	"syscall"
)

func openTTY() (inTty, outTty *os.File, err error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err //nolint:wrapcheck
	}
	return f, f, nil
}

func suspend() (err error) {
	// Send SIGTSTP to the entire process group.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGCONT)
	err = syscall.Kill(0, syscall.SIGTSTP)
	// blocks until a CONT happens...
	<-c
	return
}
