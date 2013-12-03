package test

import (
	"os"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
)

// Create a five nodes
// Kill all the nodes and restart
func TestMultiNodeKillAllAndRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := CreateCluster(clusterSize, procAttr, false)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	c := etcd.NewClient(nil)

	c.SyncCluster()

	time.Sleep(time.Second)

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

	stop := make(chan bool)
	leaderChan := make(chan string, 1)
	all := make(chan bool, 1)

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

	if result.Node.ModifiedIndex != 16 {
		t.Fatalf("recovery failed! [%d/16]", result.Node.ModifiedIndex)
	}
}
