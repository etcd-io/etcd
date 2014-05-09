package test

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// This test will kill the current leader and wait for the etcd cluster to elect a new leader for 200 times.
// It will print out the election time and the average election time.
func TestKillLeader(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	if err != nil {
		t.Fatal("cannot create cluster")
	}
	defer DestroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	go Monitor(clusterSize, 1, leaderChan, all, stop)

	var totalTime time.Duration

	leader := "http://127.0.0.1:7001"

	for i := 0; i < clusterSize; i++ {
		fmt.Println("leader is ", leader)
		port, _ := strconv.Atoi(strings.Split(leader, ":")[2])
		num := port - 7001
		fmt.Println("kill server ", num)
		etcds[num].Kill()
		etcds[num].Release()

		start := time.Now()
		for {
			newLeader := <-leaderChan
			if newLeader != leader {
				leader = newLeader
				break
			}
		}
		take := time.Now().Sub(start)

		totalTime += take
		avgTime := totalTime / (time.Duration)(i+1)
		fmt.Println("Total time:", totalTime, "; Avg time:", avgTime)

		etcds[num], err = os.StartProcess(EtcdBinPath, argGroup[num], procAttr)
	}
	stop <- true
}

// This test will kill the current leader and wait for the etcd cluster to elect a new leader for 200 times.
// It will print out the election time and the average election time.
// It runs in a cluster with standby nodes.
func TestKillLeaderWithStandbys(t *testing.T) {
	// https://github.com/goraft/raft/issues/222
	t.Skip("stuck on raft issue")

	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	if err != nil {
		t.Fatal("cannot create cluster")
	}
	defer DestroyCluster(etcds)

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	time.Sleep(time.Second)

	go Monitor(clusterSize, 1, leaderChan, all, stop)

	c := etcd.NewClient(nil)
	c.SyncCluster()

	// Reconfigure with a small active size.
	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":3, "removeDelay":2, "syncInterval":1}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// Wait for two monitor cycles before checking for demotion.
	time.Sleep((2 * server.ActiveMonitorTimeout) + (2 * time.Second))

	// Verify that we have 3 peers.
	result, err := c.Get("_etcd/machines", true, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 3)

	var totalTime time.Duration

	leader := "http://127.0.0.1:7001"

	for i := 0; i < clusterSize; i++ {
		t.Log("leader is ", leader)
		port, _ := strconv.Atoi(strings.Split(leader, ":")[2])
		num := port - 7001
		t.Log("kill server ", num)
		etcds[num].Kill()
		etcds[num].Release()

		start := time.Now()
		for {
			newLeader := <-leaderChan
			if newLeader != leader {
				leader = newLeader
				break
			}
		}
		take := time.Now().Sub(start)

		totalTime += take
		avgTime := totalTime / (time.Duration)(i+1)
		fmt.Println("Total time:", totalTime, "; Avg time:", avgTime)

		time.Sleep(server.ActiveMonitorTimeout + (1 * time.Second))
		time.Sleep(2 * time.Second)

		// Verify that we have 3 peers.
		result, err = c.Get("_etcd/machines", true, true)
		assert.NoError(t, err)
		assert.Equal(t, len(result.Node.Nodes), 3)

		// Verify that killed node is not one of those peers.
		_, err = c.Get(fmt.Sprintf("_etcd/machines/node%d", num+1), false, false)
		assert.Error(t, err)

		etcds[num], err = os.StartProcess(EtcdBinPath, argGroup[num], procAttr)
	}
	stop <- true
}
