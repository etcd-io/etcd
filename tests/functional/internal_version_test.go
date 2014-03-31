package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"

	etcdtest "github.com/coreos/etcd/tests"
)

// Ensure that etcd does not come up if the internal raft versions do not match.
func TestInternalVersion(t *testing.T) {
	checkedVersion := false
	testMux := http.NewServeMux()

	testMux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "This is not a version number")
		checkedVersion = true
	})

	testMux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not attempt to join!")
	})

	ts := httptest.NewServer(testMux)
	defer ts.Close()

	fakeURL, _ := url.Parse(ts.URL)

	i := etcdtest.NewInstance()
	i.Conf.Peers = []string{fakeURL.Host}
	if !assert.Panics(t, func() { i.Start() }) {
		t.Fatal("Expect start panic")
	}

	time.Sleep(time.Second)

	_, err := http.Get("http://127.0.0.1:4001")
	if err == nil {
		t.Fatal("etcd node should not be up")
		return
	}

	if checkedVersion == false {
		t.Fatal("etcd did not check the version")
		return
	}
}
