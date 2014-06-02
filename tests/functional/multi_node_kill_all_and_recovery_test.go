package test

import (
	"bytes"
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

// Create a five nodes
// Kill all the nodes and restart
func TestMultiNodeKillAllAndRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	c := etcd.NewClient(nil)

	go Monitor(clusterSize, clusterSize, leaderChan, all, stop)
	<-all
	<-leaderChan
	stop <- true

	c.SyncCluster()

	// send 10 commands
	for i := 0; i < 10; i++ {
		// Test Set
		_, err := c.Set("foo", "bar", 0)
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Second)

	// kill all
	DestroyCluster(etcds)

	time.Sleep(time.Second)

	stop = make(chan bool)
	leaderChan = make(chan string, 1)
	all = make(chan bool, 1)

	time.Sleep(time.Second)

	for i := 0; i < clusterSize; i++ {
		etcds[i], err = os.StartProcess(EtcdBinPath, argGroup[i], procAttr)
	}

	go Monitor(clusterSize, 1, leaderChan, all, stop)

	<-all
	<-leaderChan

	result, err := c.Set("foo", "bar", 0)

	if err != nil {
		t.Fatalf("Recovery error: %s", err)
	}

	if result.Node.ModifiedIndex != 17 {
		t.Fatalf("recovery failed! [%d/17]", result.Node.ModifiedIndex)
	}
}

// TestTLSMultiNodeKillAllAndRecovery create a five nodes
// then kill all the nodes and restart
func TestTLSMultiNodeKillAllAndRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, true)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	go Monitor(clusterSize, clusterSize, leaderChan, all, stop)
	<-all
	<-leaderChan
	stop <- true

	c.SyncCluster()

	// send 10 commands
	for i := 0; i < 10; i++ {
		// Test Set
		_, err := c.Set("foo", "bar", 0)
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Second)

	// kill all
	DestroyCluster(etcds)

	time.Sleep(time.Second)

	stop = make(chan bool)
	leaderChan = make(chan string, 1)
	all = make(chan bool, 1)

	time.Sleep(time.Second)

	for i := 0; i < clusterSize; i++ {
		etcds[i], err = os.StartProcess(EtcdBinPath, argGroup[i], procAttr)
		// See util.go for the reason to wait for server
		client := buildClient()
		err = WaitForServer("127.0.0.1:400"+strconv.Itoa(i+1), client, "http")
		if err != nil {
			t.Fatalf("node start error: %s", err)
		}
	}

	go Monitor(clusterSize, 1, leaderChan, all, stop)

	<-all
	<-leaderChan

	result, err := c.Set("foo", "bar", 0)

	if err != nil {
		t.Fatalf("Recovery error: %s", err)
	}

	if result.Node.ModifiedIndex != 17 {
		t.Fatalf("recovery failed! [%d/17]", result.Node.ModifiedIndex)
	}
}

// Create a five-node cluster
// Kill all the nodes and restart
func TestMultiNodeKillAllAndRecoveryWithStandbys(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	clusterSize := 15
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	c := etcd.NewClient(nil)

	go Monitor(clusterSize, clusterSize, leaderChan, all, stop)
	<-all
	<-leaderChan
	stop <- true

	c.SyncCluster()

	// Reconfigure with smaller active size (7 nodes) and wait for remove.
	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":7}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	time.Sleep(2*server.ActiveMonitorTimeout + (1 * time.Second))

	// Verify that there is three machines in peer mode.
	result, err := c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 7)

	// send set commands
	for i := 0; i < 2*clusterSize; i++ {
		// Test Set
		_, err := c.Set("foo", "bar", 0)
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(time.Second)

	// kill all
	DestroyCluster(etcds)

	time.Sleep(time.Second)

	stop = make(chan bool)
	leaderChan = make(chan string, 1)
	all = make(chan bool, 1)

	time.Sleep(time.Second)

	for i := 0; i < clusterSize; i++ {
		etcds[i], err = os.StartProcess(EtcdBinPath, append(argGroup[i], "-peers="), procAttr)
	}

	time.Sleep(2 * time.Second)

	// send set commands
	for i := 0; i < 2*clusterSize; i++ {
		// Test Set
		_, err := c.Set("foo", "bar", 0)
		if err != nil {
			t.Fatalf("Recovery error: %s", err)
		}
	}

	// Verify that we have seven machines.
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 7)
}

// Create a five nodes
// Kill all the nodes and restart, then remove the leader
func TestMultiNodeKillAllAndRecoveryAndRemoveLeader(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	c := etcd.NewClient(nil)

	go Monitor(clusterSize, clusterSize, leaderChan, all, stop)
	<-all
	<-leaderChan
	stop <- true

	// It needs some time to sync current commits and write it to disk.
	// Or some instance may be restarted as a new peer, and we don't support
	// to connect back the old cluster that doesn't have majority alive
	// without log now.
	time.Sleep(time.Second)

	c.SyncCluster()

	// kill all
	DestroyCluster(etcds)

	time.Sleep(time.Second)

	stop = make(chan bool)
	leaderChan = make(chan string, 1)
	all = make(chan bool, 1)

	time.Sleep(time.Second)

	for i := 0; i < clusterSize; i++ {
		etcds[i], err = os.StartProcess(EtcdBinPath, argGroup[i], procAttr)
	}

	go Monitor(clusterSize, 1, leaderChan, all, stop)

	<-all
	leader := <-leaderChan

	_, err = c.Set("foo", "bar", 0)
	if err != nil {
		t.Fatalf("Recovery error: %s", err)
	}

	port, _ := strconv.Atoi(strings.Split(leader, ":")[2])
	num := port - 7000
	resp, _ := tests.Delete(leader+"/v2/admin/machines/node"+strconv.Itoa(num), "application/json", nil)
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// check the old leader is in standby mode now
	time.Sleep(time.Second)
	resp, _ = tests.Get(leader + "/name")
	assert.Equal(t, resp.StatusCode, 404)
}
