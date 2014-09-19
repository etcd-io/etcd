package testutil

import (
	"runtime"
)

// WARNING: This is a hack.
// Remove this when we are able to block/check the status of the go-routines.
func ForceGosched() {
	// possibility enough to sched upto 10 go routines.
	for i := 0; i < 10000; i++ {
		runtime.Gosched()
	}
}
