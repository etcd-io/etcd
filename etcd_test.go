package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"math/rand"
	"os"
	//"strconv"
	"testing"
	"time"
)

// Create a single node and try to set value
func TestSingleNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-h=127.0.0.1", "-f", "-d=/tmp/node1"}

	process, err := os.StartProcess("etcd", args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()

	time.Sleep(time.Second)

	c := etcd.NewClient()

	c.SyncCluster()
	// Test Set
	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

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
	args := []string{"etcd", "-h=127.0.0.1", "-d=/tmp/node1"}

	process, err := os.StartProcess("etcd", append(args, "-f"), procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}

	time.Sleep(time.Second)

	c := etcd.NewClient()

	c.SyncCluster()
	// Test Set
	result, err := c.Set("foo", "bar", 100)

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

	results, err := c.Get("foo")
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

	c := etcd.NewClient()

	c.SyncCluster()

	// Test Set
	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL != 99 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

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

	c := etcd.NewClient()

	c.SyncCluster()

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
