package main 

import (
	"github.com/coreos/go-etcd/etcd"
	"fmt"
	"time"
)

var count = 0

func main() {
	ch := make(chan bool, 10)
	// set up a lock
	for i:=0; i < 100; i++ {
		go t(i, ch, etcd.NewClient())
	}
	start := time.Now()
	for i:=0; i< 100; i++ {
		<-ch
	}
	fmt.Println(time.Now().Sub(start), ": ", 100 * 50, "commands")
}

func t(num int, ch chan bool, c *etcd.Client) {
	c.SyncCluster()
	for i := 0; i < 50; i++ {
		str := fmt.Sprintf("foo_%d",num * i)
		c.Set(str, "10", 0)
	}
	ch<-true
}
