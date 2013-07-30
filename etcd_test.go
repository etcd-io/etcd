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
	args := []string{"etcd", "-i", "-d=/tmp/node1"}

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

// This test creates a single node and then set a value to it.
// Then this test kills the node and restart it and tries to get the value again.
func TestSingleNodeRecovery(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-d=/tmp/node1"}

	process, err := os.StartProcess("etcd", append(args, "-i"), procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}

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

	process.Kill()

	process, err = os.StartProcess("etcd", args, procAttr)
	defer process.Kill()
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}

	time.Sleep(time.Second)

	results, err := etcd.Get("foo")
	if err != nil {
		t.Fatal("get fail: " + err.Error())
		return
	}

	result = results[0]

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL > 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Recovery Get failed with %s %s %v", result.Key, result.Value, result.TTL)
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
	go set(stop)

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
	fmt.Println("stop")
	stop <- true
	<-stop
}

// Sending set commands
func set(stop chan bool) {

	stopSet := false
	i := 0

	for {
		key := fmt.Sprintf("%s_%v", "foo", i)

		result, err := etcd.Set(key, "bar", 0)

		if err != nil || result.Key != "/"+key || result.Value != "bar" {
			select {
			case <-stop:
				stopSet = true

			default:
				fmt.Println("Set failed!")
				return
			}
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
	fmt.Println("set stop")
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
	for i, etcd := range etcds {
		err := etcd.Kill()
		fmt.Println("kill ", i)
		if err != nil {
			panic(err.Error())
		}
		etcd.Release()
	}
	return nil
}
