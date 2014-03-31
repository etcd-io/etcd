package test

import (
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	etcdtest "github.com/coreos/etcd/tests"
)

// Create a five nodes
// Kill all the nodes and restart
func TestMultiNodeKillAllAndRecovery(t *testing.T) {
	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	clusterSize := 5
	cluster := etcdtest.NewCluster(clusterSize, false)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

	c := etcd.NewClient(nil)

	go cluster.Monitor(clusterSize, leaderChan, all, stop)
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
	cluster.Stop()

	time.Sleep(time.Second)

	stop = make(chan bool)
	leaderChan = make(chan string, 1)
	all = make(chan bool, 1)

	time.Sleep(time.Second)

	cluster.Start()

	go cluster.Monitor(1, leaderChan, all, stop)

	<-all
	<-leaderChan

	result, err := c.Set("foo", "bar", 0)

	if err != nil {
		t.Fatalf("Recovery error: %s", err)
	}

	if result.Node.ModifiedIndex != 16 {
		t.Fatalf("recovery failed! [%d/16]", result.Node.ModifiedIndex)
	}
}

// TestTLSMultiNodeKillAllAndRecovery create a five nodes
// then kill all the nodes and restart
func TestTLSMultiNodeKillAllAndRecovery(t *testing.T) {
	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

	clusterSize := 5
	cluster := etcdtest.NewCluster(clusterSize, false)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

	c := etcd.NewClient(nil)

	go cluster.Monitor(clusterSize, leaderChan, all, stop)
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
	cluster.Stop()

	time.Sleep(time.Second)

	stop = make(chan bool)
	leaderChan = make(chan string, 1)
	all = make(chan bool, 1)

	time.Sleep(time.Second)

	cluster.Start()

	go cluster.Monitor(1, leaderChan, all, stop)

	<-all
	<-leaderChan

	result, err := c.Set("foo", "bar", 0)

	if err != nil {
		t.Fatalf("Recovery error: %s", err)
	}

	if result.Node.ModifiedIndex != 16 {
		t.Fatalf("recovery failed! [%d/16]", result.Node.ModifiedIndex)
	}
}
