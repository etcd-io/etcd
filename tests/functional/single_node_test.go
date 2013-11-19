package test

import (
	"os"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
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

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.TTL < 95 {
		if err != nil {
			t.Fatal("Set 1: ", err)
		}

		t.Fatalf("Set 1 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	time.Sleep(time.Second)

	result, err = c.Set("foo", "bar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "bar" || result.PrevValue != "bar" || result.TTL != 100 {
		if err != nil {
			t.Fatal("Set 2: ", err)
		}
		t.Fatalf("Set 2 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	// Add a test-and-set test

	// First, we'll test we can change the value if we get it write
	result, match, err := c.TestAndSet("foo", "bar", "foobar", 100)

	if err != nil || result.Key != "/foo" || result.Value != "foobar" || result.PrevValue != "bar" || result.TTL != 100 || !match {
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Set 3 failed with %s %s %v", result.Key, result.Value, result.TTL)
	}

	// Next, we'll make sure we can't set it without the correct prior value
	_, _, err = c.TestAndSet("foo", "bar", "foofoo", 100)

	if err == nil {
		t.Fatalf("Set 4 expecting error when setting key with incorrect previous value")
	}

	// Finally, we'll make sure a blank previous value still counts as a test-and-set and still has to match
	_, _, err = c.TestAndSet("foo", "", "barbar", 100)

	if err == nil {
		t.Fatalf("Set 5 expecting error when setting key with blank (incorrect) previous value")
	}
}
