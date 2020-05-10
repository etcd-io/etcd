package main

import (
	"os"
	"os/signal"
	"syscall"
)

func fn() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Ignore(os.Signal(syscall.SIGKILL)) // want `cannot be trapped`
	signal.Ignore(os.Kill)                    // want `cannot be trapped`
	signal.Notify(c, os.Kill)                 // want `cannot be trapped`
	signal.Reset(os.Kill)                     // want `cannot be trapped`
	signal.Ignore(syscall.SIGKILL)            // want `cannot be trapped`
	signal.Notify(c, syscall.SIGKILL)         // want `cannot be trapped`
	signal.Reset(syscall.SIGKILL)             // want `cannot be trapped`
}
