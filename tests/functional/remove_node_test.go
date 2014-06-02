package test

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// remove the node and node rejoin with previous log
func TestRemoveNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 4
	argGroup, etcds, _ := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()

	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":4, "syncInterval":5}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	rmReq, _ := http.NewRequest("DELETE", "http://127.0.0.1:7001/remove/node3", nil)

	client := &http.Client{}
	for i := 0; i < 2; i++ {
		for i := 0; i < 2; i++ {
			client.Do(rmReq)

			fmt.Println("send remove to node3 and wait for its exiting")
			time.Sleep(100 * time.Millisecond)

			resp, err := c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 3 {
				t.Fatal("cannot remove peer")
			}

			etcds[2].Kill()
			etcds[2].Wait()

			if i == 1 {
				// rejoin with log
				etcds[2], err = os.StartProcess(EtcdBinPath, argGroup[2], procAttr)
			} else {
				// rejoin without log
				etcds[2], err = os.StartProcess(EtcdBinPath, append(argGroup[2], "-f"), procAttr)
			}

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second + 5*time.Second)

			resp, err = c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 4 {
				t.Fatalf("add peer fails #1 (%d != 4)", len(resp.Node.Nodes))
			}
		}

		// first kill the node, then remove it, then add it back
		for i := 0; i < 2; i++ {
			etcds[2].Kill()
			fmt.Println("kill node3 and wait for its exiting")
			etcds[2].Wait()

			client.Do(rmReq)

			time.Sleep(100 * time.Millisecond)

			resp, err := c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 3 {
				t.Fatal("cannot remove peer")
			}

			if i == 1 {
				// rejoin with log
				etcds[2], err = os.StartProcess(EtcdBinPath, append(argGroup[2]), procAttr)
			} else {
				// rejoin without log
				etcds[2], err = os.StartProcess(EtcdBinPath, append(argGroup[2], "-f"), procAttr)
			}

			if err != nil {
				panic(err)
			}

			time.Sleep(time.Second + time.Second)

			resp, err = c.Get("_etcd/machines", false, false)

			if err != nil {
				panic(err)
			}

			if len(resp.Node.Nodes) != 4 {
				t.Fatalf("add peer fails #2 (%d != 4)", len(resp.Node.Nodes))
			}
		}
	}
}

func TestRemovePausedNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 4
	_, etcds, _ := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()

	r, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":3, "removeDelay":1, "syncInterval":1}`))
	if !assert.Equal(t, r.StatusCode, 200) {
		t.FailNow()
	}
	// Wait for standby instances to update its cluster config
	time.Sleep(6 * time.Second)

	resp, err := c.Get("_etcd/machines", false, false)
	if err != nil {
		panic(err)
	}
	if len(resp.Node.Nodes) != 3 {
		t.Fatal("cannot remove peer")
	}

	for i := 0; i < clusterSize; i++ {
		// first pause the node, then remove it, then resume it
		idx := rand.Int() % clusterSize

		etcds[idx].Signal(syscall.SIGSTOP)
		fmt.Printf("pause node%d and let standby node take its place\n", idx+1)

		time.Sleep(4 * time.Second)

		etcds[idx].Signal(syscall.SIGCONT)
		// let it change its state to candidate at least
		time.Sleep(time.Second)

		stop := make(chan bool)
		leaderChan := make(chan string, 1)
		all := make(chan bool, 1)

		go Monitor(clusterSize, clusterSize, leaderChan, all, stop)
		<-all
		<-leaderChan
		stop <- true

		resp, err = c.Get("_etcd/machines", false, false)
		if err != nil {
			panic(err)
		}
		if len(resp.Node.Nodes) != 3 {
			t.Fatalf("add peer fails (%d != 3)", len(resp.Node.Nodes))
		}
		for i := 0; i < 3; i++ {
			if resp.Node.Nodes[i].Key == fmt.Sprintf("node%d", idx+1) {
				t.Fatal("node should be removed")
			}
		}
	}
}
