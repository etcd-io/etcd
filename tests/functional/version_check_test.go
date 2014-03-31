package test

import (
	"net/http"
	"testing"
	"time"

	etcdtest "github.com/coreos/etcd/tests"
)

// Ensure that a node can reply to a version check appropriately.
func TestVersionCheck(t *testing.T) {
	i := etcdtest.NewInstance()
	if err := i.Start(); err != nil {
		t.Fatal("cannot start etcd")
	}
	defer i.Stop()

	time.Sleep(time.Second)

	// Check a version too small.
	resp, _ := http.Get("http://localhost:7001/version/1/check")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatal("Invalid version check: ", resp.StatusCode)
	}

	// Check a version too large.
	resp, _ = http.Get("http://localhost:7001/version/3/check")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatal("Invalid version check: ", resp.StatusCode)
	}

	// Check a version that's just right.
	resp, _ = http.Get("http://localhost:7001/version/2/check")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal("Invalid version check: ", resp.StatusCode)
	}
}
