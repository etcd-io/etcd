package test

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// Ensure that a node can reply to a version check appropriately.
func TestVersionCheck(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-name=node1", "-f", "-data-dir=/tmp/version_check"}

	process, err := os.StartProcess(EtcdBinPath, args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()

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
