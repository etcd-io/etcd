// +build android darwin dragonfly freebsd linux netbsd openbsd solaris

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func fn2() {
	c := make(chan os.Signal, 1)
	signal.Ignore(syscall.SIGSTOP)    // want `cannot be trapped`
	signal.Notify(c, syscall.SIGSTOP) // want `cannot be trapped`
	signal.Reset(syscall.SIGSTOP)     // want `cannot be trapped`
}
