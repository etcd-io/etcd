package test

import (
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	etcdtest "github.com/coreos/etcd/tests"
)

// This test creates a single node and then set a value to it.
// Then this test kills the node and restart it and tries to get the value again.
func TestSingleNodeRecovery(t *testing.T) {
	i := etcdtest.NewInstance()
	if err := i.Start(); err != nil {
		t.Fatal("cannot start etcd")
	}

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()
	// Test Set
	result, err := c.Set("foo", "bar", 100)
	node := result.Node

	if err != nil || node.Key != "/foo" || node.Value != "bar" || node.TTL < 95 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", node.Key, node.Value, node.TTL)
	}

	time.Sleep(time.Second)

	i.Stop()

	if err := i.Start(); err != nil {
		t.Fatal("cannot start etcd")
	}
	defer i.Stop()

	time.Sleep(time.Second)

	result, err = c.Get("foo", false, false)
	node = result.Node

	if err != nil {
		t.Fatal("get fail: " + err.Error())
		return
	}

	if err != nil || node.Key != "/foo" || node.Value != "bar" || node.TTL > 99 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Recovery Get failed with %s %s %v", node.Key, node.Value, node.TTL)
	}
}
