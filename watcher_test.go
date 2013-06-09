package main

import (
	"testing"
	"fmt"
)

func TestWatch(t *testing.T) {
	// watcher := createWatcher()
	c := make(chan Notification)
	d := make(chan Notification)
	w.add("/", c, say)
	w.add("/prefix/", d, say)
	s.Set("/prefix/foo", "bar")
}

func say(c chan Notification) {
	result := <-c

	if result.action != -1 {
		fmt.Println("yes")
	} else {
		fmt.Println("no")
	}

}
