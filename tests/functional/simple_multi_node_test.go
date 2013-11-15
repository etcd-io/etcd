package test

import (
	"os"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
)

func TestSimpleMultiNode(t *testing.T) {
	templateTestSimpleMultiNode(t, false)
}

func TestSimpleMultiNodeTls(t *testing.T) {
	templateTestSimpleMultiNode(t, true)
}

// Create a three nodes and try to set value
func templateTestSimpleMultiNode(t *testing.T, tls bool) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	clusterSize := 3

	_, etcds, err := CreateCluster(clusterSize, procAttr, tls)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	defer DestroyCluster(etcds)

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()

	// Test Set
	result, err := c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL < 95 {
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL < 95 {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

}
