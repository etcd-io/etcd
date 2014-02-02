package test

import (
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

// Create a single node and try to set value
func TestSingleNode(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-name=node1", "-f", "-data-dir=/tmp/node1"}

	process, err := os.StartProcess(EtcdBinPath, args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()
	// Test Set
	result, err := c.Set("foo", "bar", 100)
	node := result.Node

	if err != nil || node.Key != "/foo" || node.Value != "bar" || node.TTL < 95 {
		if err != nil {
			t.Fatal("Set 1: ", err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", node.Key, node.Value, node.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)
	node = result.Node

	if err != nil || node.Key != "/foo" || node.Value != "bar" || node.TTL != 100 {
		if err != nil {
			t.Fatal("Set 2: ", err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", node.Key, node.Value, node.TTL)
	}

	// Add a test-and-set test

	// First, we'll test we can change the value if we get it write
	result, err = c.CompareAndSwap("foo", "foobar", 100, "bar", 0)
	node = result.Node

	if err != nil || node.Key != "/foo" || node.Value != "foobar" || node.TTL != 100 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 3 failed with %s %s %v", node.Key, node.Value, node.TTL)
	}

	// Next, we'll make sure we can't set it without the correct prior value
	_, err = c.CompareAndSwap("foo", "foofoo", 100, "bar", 0)

	if err == nil {
		t.Fatalf("Set 4 expecting error when setting key with incorrect previous value")
	}
}
