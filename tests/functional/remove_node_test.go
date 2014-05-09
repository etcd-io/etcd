package test

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
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

	clusterSize := 3
	argGroup, etcds, _ := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()

	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"syncClusterInterval":1}`))
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

			if len(resp.Node.Nodes) != 2 {
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

			time.Sleep(time.Second + time.Second)

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
			etcds[2].Kill()
			fmt.Println("kill node3 and wait for its exiting")
			etcds[2].Wait()

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
				etcds[2], err = os.StartProcess(EtcdBinPath, append(argGroup[2]), procAttr)
			} else {
				// rejoin without log
				etcds[2], err = os.StartProcess(EtcdBinPath, append(argGroup[2], "-f"), procAttr)
			}

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
