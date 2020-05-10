package pkg

import (
	"os"
	"os/signal"
	"syscall"
)

func fn(b bool) {
	c0 := make(chan os.Signal)
	signal.Notify(c0, os.Interrupt) // want `the channel used with signal\.Notify should be buffered`

	c1 := make(chan os.Signal, 1)
	signal.Notify(c1, os.Interrupt, syscall.SIGHUP)

	c2 := c0
	if b {
		c2 = c1
	}
	signal.Notify(c2, os.Interrupt, syscall.SIGHUP)
}
