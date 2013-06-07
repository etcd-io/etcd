package raftd

import (
	"testing"
	"fmt"
)

func TestWatch(t *testing.T) {
	watcher := createWatcher()
	c := make(chan Notification)
	d := make(chan Notification)
	watcher.add("/", c, say)
	watcher.add("/prefix/", d, say)
	watcher.notify(0, "/prefix/hihihi", "1", "1")
}

func say(c chan Notification) {
	result := <-c

	if result.action != -1 {
		fmt.Println("yes")
	} else {
		fmt.Println("no")
	}

}
