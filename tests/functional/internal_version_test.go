package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

// Ensure that etcd does not come up if the internal raft versions do not match.
func TestInternalVersion(t *testing.T) {
	var mu sync.Mutex

	checkedVersion := false
	testMux := http.NewServeMux()

	testMux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "This is not a version number")
		mu.Lock()
		defer mu.Unlock()

		checkedVersion = true
	})

	testMux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not attempt to join!")
	})

	ts := httptest.NewServer(testMux)
	defer ts.Close()

	fakeURL, _ := url.Parse(ts.URL)

	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}
	args := []string{"etcd", "-name=node1", "-f", "-data-dir=/tmp/node1", "-peers=" + fakeURL.Host}

	process, err := os.StartProcess(EtcdBinPath, args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}

	time.Sleep(time.Second)
	process.Kill()

	_, err = http.Get("http://127.0.0.1:4001")
	if err == nil {
		t.Fatal("etcd node should not be up")
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if checkedVersion == false {
		t.Fatal("etcd did not check the version")
		return
	}
}
