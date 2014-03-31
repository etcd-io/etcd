package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	etcdtest "github.com/coreos/etcd/tests"
)

// remove the node and node rejoin with previous log
func TestRemoveNode(t *testing.T) {
	clusterSize := 3
	cluster := etcdtest.NewCluster(clusterSize, false)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()

	rmReq, _ := http.NewRequest("DELETE", "http://127.0.0.1:7001/v2/admin/machines/node3", nil)

	client := &http.Client{}
	for i := 0; i < 2; i++ {
		for i := 0; i < 2; i++ {
			client.Do(rmReq)

			fmt.Println("send remove to node3 and wait for its exiting")
			cluster.WaitOne(2)

			resp, err := c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 2 {
				t.Fatal("cannot remove peer")
			}

			if i == 1 {
				// rejoin with log
				cluster.Instances[2].Conf.Force = false
			} else {
				// rejoin without log
				cluster.Instances[2].Conf.Force = true
			}
			cluster.StartOne(2)

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second)

			resp, err = c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 3 {
				t.Fatalf("add peer fails #1 (%d != 3)", len(resp.Node.Nodes))
			}
		}

		// first kill the node, then remove it, then add it back
		for i := 0; i < 2; i++ {
			fmt.Println("kill node3 and wait for its exiting")
			cluster.StopOne(2)

			client.Do(rmReq)

			resp, err := c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 2 {
				t.Fatal("cannot remove peer")
			}

			if i == 1 {
				// rejoin with log
				cluster.Instances[2].Conf.Force = false
			} else {
				// rejoin without log
				cluster.Instances[2].Conf.Force = true
			}
			cluster.StartOne(2)

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second)

			resp, err = c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 3 {
				t.Fatalf("add peer fails #2 (%d != 3)", len(resp.Node.Nodes))
			}
		}
	}
}
