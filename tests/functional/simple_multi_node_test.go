package test

import (
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	etcdtest "github.com/coreos/etcd/tests"
)

func TestSimpleMultiNode(t *testing.T) {
	templateTestSimpleMultiNode(t, false)
}

func TestSimpleMultiNodeTls(t *testing.T) {
	templateTestSimpleMultiNode(t, true)
}

// Create a three nodes and try to set value
func templateTestSimpleMultiNode(t *testing.T, tls bool) {
	clusterSize := 3
	cluster := etcdtest.NewCluster(clusterSize, tls)
	if !cluster.Start() {
		t.Fatal("cannot start cluster")
	}
	defer cluster.Stop()

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	if c.SyncCluster() == false {
		t.Fatal("Cannot sync cluster!")
	}

	// Test Set
	result, err := c.Set("foo", "bar", 100)
	if err != nil {
		t.Fatal(err)
	}

	node := result.Node
	if node.Key != "/foo" || node.Value != "bar" || node.TTL < 95 {
		t.Fatalf("Set 1 failed with %s %s %v", node.Key, node.Value, node.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)
	if err != nil {
		t.Fatal(err)
	}

	node = result.Node
	if node.Key != "/foo" || node.Value != "bar" || node.TTL < 95 {
		t.Fatalf("Set 2 failed with %s %s %v", node.Key, node.Value, node.TTL)
	}

}
