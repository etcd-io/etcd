package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
)

var count = 0

func main() {

	good := 0
	bad := 0

	ch := make(chan bool, 10)
	// set up a lock
	c := etcd.NewClient()
	c.Set("lock", "unlock", 0)


	for i := 0; i < 10; i++ {
		go t(i, ch, etcd.NewClient())
	}

	for i := 0; i < 10; i++ {
		if <-ch {
			good++
		} else {
			bad++
		}
	}
	fmt.Println("good: ", good, "bad: ", bad)
}

func t(num int, ch chan bool, c *etcd.Client) {
	for i := 0; i < 100; i++ {
		if lock(c) {
			// a stupid spin lock
			count++
			fmt.Println(num, " got the lock and update count to", count)
			unlock(c)
			fmt.Println(num, " released the lock")
		} else {
			ch <- false
			return
		}
	}
	ch <- true
}

// A stupid spin lock
func lock(c *etcd.Client) bool {
	for {
		_, success, _ := c.TestAndSet("lock", "unlock", "lock", 0)

		if success != true {
			fmt.Println("tried lock failed!")
		} else {
			return true
		}
	}
}

func unlock(c *etcd.Client) {
	for {
		_, err := c.Set("lock", "unlock", 0)
		if err == nil {
			return
		}
		fmt.Println(err)
	}
}
