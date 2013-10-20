
package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"strconv"
	"time"
)

func main() {
	fmt.Println("etcd-client started")
	c := etcd.NewClient(nil)
	c.SetCluster([]string{
		"http://127.0.0.1:4001",
		"http://127.0.0.1:4002",
		"http://127.0.0.1:4003",
	})

	ticker := time.NewTicker(time.Second * 3)

	for {
		select {
		case d := <-ticker.C:
			n := d.Second()
			if n <= 0 {
				n = 60
			}

			for ok := c.SyncCluster(); ok == false; {
				fmt.Println("SyncCluster failed, trying again")
				time.Sleep(100 * time.Millisecond)
			}

			result, err := c.Set("foo", "exp_"+strconv.Itoa(n), 0)
			if err != nil {
				fmt.Println("set error", err)
			} else {
				fmt.Printf("set %+v\n", result)
			}

			ss, err := c.Get("foo")
			if err != nil {
				fmt.Println("get error", err)
			} else {
				fmt.Println(len(ss))
			}

		}
	}
}
