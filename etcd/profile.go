package etcd

import (
	"os"
	"os/signal"
	"runtime/pprof"

	"github.com/coreos/etcd/log"
)

// profile starts CPU profiling.
func profile(path string) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		sig := <-c
		log.Infof("captured %v, stopping profiler and exiting..", sig)
		pprof.StopCPUProfile()
		os.Exit(1)
	}()
}
