// +build ignore

package test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"

	"github.com/coreos/etcd/server"
	etcdtest "github.com/coreos/etcd/tests"
	goetcd "github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

type garbageHandler struct {
	t       *testing.T
	success bool
	sync.Mutex
}

func (g *garbageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, client")
	if r.URL.String() != "/v2/keys/_etcd/registry/1/node1" {
		g.t.Fatalf("Unexpected web request")
	}
	g.Lock()
	defer g.Unlock()

	g.success = true
}

// TestDiscoverySecondPeerFirstNoResponse ensures that if the first etcd
// machine stops after heartbeating that the second machine fails too.
func TestDiscoverySecondPeerFirstNoResponse(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "started")
		resp, err := etcdtest.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/_etcd/registry/2/_state"), v)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)

		v = url.Values{}
		v.Set("value", "http://127.0.0.1:49151")
		resp, err = etcdtest.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/_etcd/registry/2/ETCDTEST"), v)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)

		proc, err := startServer([]string{"-retry-interval", "0.2", "-discovery", s.URL() + "/v2/keys/_etcd/registry/2"})
		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		// TODO(bp): etcd will take 30 seconds to shutdown, figure this
		// out instead
		time.Sleep(1 * time.Second)

		client := http.Client{}
		_, err = client.Get("/")
		if err != nil && strings.Contains(err.Error(), "connection reset by peer") {
			t.Fatal(err.Error())
		}
	})
}

func assertServerNotUp(client http.Client, scheme string) error {
	path := fmt.Sprintf("%s://127.0.0.1:4001/v2/keys/foo", scheme)
	fields := url.Values(map[string][]string{"value": {"bar"}})

	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)

		_, err := client.PostForm(path, fields)
		if err == nil {
			return errors.New("Expected error during POST, got nil")
		} else {
			errString := err.Error()
			if strings.Contains(errString, "connection refused") {
				return nil
			} else {
				return err
			}
		}
	}

	return nil
}
