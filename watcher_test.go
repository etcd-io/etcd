package raftd

import (
	"testing"
)

func TestWatch(t *testing.T) {
	watcher := createWatcher()
	c := make(chan int)
	d := make(chan int)
	watcher.add("/prefix/", c)
	watcher.add("/prefix/", d)
	watcher.notify(0, "/prefix/hihihi", "1", "1")
}