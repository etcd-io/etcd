package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

// Create a single node and try to set value
func TestSingleNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-i", "-v", "-d=/tmp/node1"}

	process, err := os.StartProcess("etcd", args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()

	time.Sleep(time.Second)

	etcd.SyncCluster()
	// Test Set
	result, err := etcd.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = etcd.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}
}

// Create a three nodes and try to set value
func TestSimpleMultiNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3

	_, etcds, err := createCluster(clusterSize, procAttr)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer destroyCluster(etcds)

	time.Sleep(time.Second)

	etcd.SyncCluster()

	// Test Set
	result, err := etcd.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = etcd.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

}

// Create a five nodes
// Randomly kill one of the node and keep on sending set command to the cluster
func TestMultiNodeRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 5
	argGroup, etcds, err := createCluster(clusterSize, procAttr)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer destroyCluster(etcds)

	time.Sleep(2 * time.Second)

	etcd.SyncCluster()

	stop := make(chan bool)
	// Test Set
	go set(t, stop)

	for i := 0; i < 10; i++ {
		num := rand.Int() % clusterSize
		fmt.Println("kill node", num+1)

		// kill
		etcds[num].Kill()
		etcds[num].Release()
		time.Sleep(time.Second)

		// restart
		etcds[num], err = os.StartProcess("etcd", argGroup[num], procAttr)
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Second)
	}

	stop <- true
	<-stop
}

// Sending set commands
func set(t *testing.T, stop chan bool) {

	stopSet := false
	i := 0

	for {
		key := fmt.Sprintf("%s_%v", "foo", i)

		result, err := etcd.Set(key, "bar", 0)

		if err != nil || result.Key != "/"+key || result.Value != "bar" {
			if err != nil {
				t.Fatal(err)
			}
			t.Fatalf("Set failed with %s %s %v", result.Key, result.Value)
		}

		select {
		case <-stop:
			stopSet = true

		default:
		}

		if stopSet {
			break
		}

		i++
	}

	stop <- true
}

// Create a cluster of etcd nodes
func createCluster(size int, procAttr *os.ProcAttr) ([][]string, []*os.Process, error) {
	argGroup := make([][]string, size)
	for i := 0; i < size; i++ {
		if i == 0 {
			argGroup[i] = []string{"etcd", "-d=/tmp/node1"}
		} else {
			strI := strconv.Itoa(i + 1)
			argGroup[i] = []string{"etcd", "-c=400" + strI, "-s=700" + strI, "-d=/tmp/node" + strI, "-C=127.0.0.1:7001"}
		}
	}

	etcds := make([]*os.Process, size)

	for i, _ := range etcds {
		var err error
		etcds[i], err = os.StartProcess("etcd", append(argGroup[i], "-i"), procAttr)
		if err != nil {
			return nil, nil, err
		}
	}

	return argGroup, etcds, nil
}

// Destroy all the nodes in the cluster
func destroyCluster(etcds []*os.Process) error {
	for _, etcd := range etcds {
		etcd.Kill()
		etcd.Release()
	}
	return nil
}
