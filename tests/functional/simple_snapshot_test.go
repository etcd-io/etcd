package test

import (
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"

	etcdtest "github.com/coreos/etcd/tests"
)

// This test creates a single node and then set a value to it to trigger snapshot
func TestSimpleSnapshot(t *testing.T) {
	i := etcdtest.NewInstance()
	i.Conf.Snapshot = true
	i.Conf.SnapshotCount = 500
	if err := i.Start(); err != nil {
		t.Fatal("cannot start etcd")
	}
	defer i.Stop()

	time.Sleep(time.Second)

	c := etcd.NewClient(nil)

	c.SyncCluster()
	// issue first 501 commands
	for i := 0; i < 501; i++ {
		result, err := c.Set("foo", "bar", 100)
		node := result.Node

		if err != nil || node.Key != "/foo" || node.Value != "bar" || node.TTL < 95 {
			if err != nil {
				t.Fatal(err)
			}

			t.Fatalf("Set failed with %s %s %v", node.Key, node.Value, node.TTL)
		}
	}

	// wait for a snapshot interval
	time.Sleep(3 * time.Second)

	snapshots, err := ioutil.ReadDir("/tmp/node/snapshot")

	if err != nil {
		t.Fatal("list snapshot failed:" + err.Error())
	}

	if len(snapshots) != 1 {
		t.Fatal("wrong number of snapshot :[1/", len(snapshots), "]")
	}

	index, _ := strconv.Atoi(snapshots[0].Name()[2:5])

	if index < 507 || index > 510 {
		t.Fatal("wrong name of snapshot :", snapshots[0].Name())
	}

	// issue second 501 commands
	for i := 0; i < 501; i++ {
		result, err := c.Set("foo", "bar", 100)
		node := result.Node

		if err != nil || node.Key != "/foo" || node.Value != "bar" || node.TTL < 95 {
			if err != nil {
				t.Fatal(err)
			}

			t.Fatalf("Set failed with %s %s %v", node.Key, node.Value, node.TTL)
		}
	}

	// wait for a snapshot interval
	time.Sleep(3 * time.Second)

	snapshots, err = ioutil.ReadDir("/tmp/node/snapshot")

	if err != nil {
		t.Fatal("list snapshot failed:" + err.Error())
	}

	if len(snapshots) != 1 {
		t.Fatal("wrong number of snapshot :[1/", len(snapshots), "]")
	}

	index, _ = strconv.Atoi(snapshots[0].Name()[2:6])

	if index < 1014 || index > 1017 {
		t.Fatal("wrong name of snapshot :", snapshots[0].Name())
	}
}
