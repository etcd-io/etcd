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

// TestDiscoveryDownNoBackupPeers ensures that etcd stops if it is started with a
// bad discovery URL and no backups.
func TestDiscoveryDownNoBackupPeers(t *testing.T) {
	g := garbageHandler{t: t}
	ts := httptest.NewServer(&g)
	defer ts.Close()

	discover := ts.URL + "/v2/keys/_etcd/registry/1"
	proc, err := startServer([]string{"-discovery", discover})

	if err != nil {
		t.Fatal(err.Error())
	}
	defer stopServer(proc)

	client := http.Client{}
	err = assertServerNotUp(client, "http")
	if err != nil {
		t.Fatal(err.Error())
	}

	g.Lock()
	defer g.Unlock()
	if !g.success {
		t.Fatal("Discovery server never called")
	}
}

// TestDiscoveryDownWithBackupPeers ensures that etcd runs if it is started with a
// bad discovery URL and a peer list.
func TestDiscoveryDownWithBackupPeers(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		g := garbageHandler{t: t}
		ts := httptest.NewServer(&g)
		defer ts.Close()

		discover := ts.URL + "/v2/keys/_etcd/registry/1"
		u, ok := s.PeerHost("ETCDTEST")
		if !ok {
			t.Fatalf("Couldn't find the URL")
		}
		proc, err := startServer([]string{"-discovery", discover, "-peers", u})

		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		client := http.Client{}
		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}

		g.Lock()
		defer g.Unlock()
		if !g.success {
			t.Fatal("Discovery server never called")
		}
	})
}

// TestDiscoveryNoWithBackupPeers ensures that etcd runs if it is started with
// no discovery URL and a peer list.
func TestDiscoveryNoWithBackupPeers(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		u, ok := s.PeerHost("ETCDTEST")
		if !ok {
			t.Fatalf("Couldn't find the URL")
		}
		proc, err := startServer([]string{"-peers", u})

		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		client := http.Client{}
		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}
	})
}

// TestDiscoveryDownNoBackupPeersWithDataDir ensures that etcd runs if it is
// started with a bad discovery URL, no backups and valid data dir.
func TestDiscoveryDownNoBackupPeersWithDataDir(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		u, ok := s.PeerHost("ETCDTEST")
		if !ok {
			t.Fatalf("Couldn't find the URL")
		}

		// run etcd and connect to ETCDTEST server
		proc, err := startServer([]string{"-peers", u})
		if err != nil {
			t.Fatal(err.Error())
		}

		// check it runs well
		client := http.Client{}
		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}

		// stop etcd, and leave valid data dir for later usage
		stopServer(proc)

		g := garbageHandler{t: t}
		ts := httptest.NewServer(&g)
		defer ts.Close()

		discover := ts.URL + "/v2/keys/_etcd/registry/1"
		// connect to ETCDTEST server again with previous data dir
		proc, err = startServerWithDataDir([]string{"-discovery", discover})
		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		// TODO(yichengq): it needs some time to do leader election
		// improve to get rid of it
		time.Sleep(1 * time.Second)

		client = http.Client{}
		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}

		if !g.success {
			t.Fatal("Discovery server never called")
		}
	})
}

// TestDiscoveryFirstPeer ensures that etcd starts as the leader if it
// registers as the first peer.
func TestDiscoveryFirstPeer(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		proc, err := startServer([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/2"})
		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		client := http.Client{}
		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}
	})
}

// TestDiscoverySecondPeerFirstDown ensures that etcd stops if it is started with a
// correct discovery URL but no active machines are found.
func TestDiscoverySecondPeerFirstDown(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "started")
		resp, err := etcdtest.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/_etcd/registry/2/_state"), v)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)

		proc, err := startServer([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/2"})
		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		client := http.Client{}
		err = assertServerNotUp(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}
	})
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

// TestDiscoverySecondPeerUp ensures that a second peer joining a discovery
// cluster works.
func TestDiscoverySecondPeerUp(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		v := url.Values{}
		v.Set("value", "started")
		resp, err := etcdtest.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/_etcd/registry/3/_state"), v)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)

		u, ok := s.PeerURL("ETCDTEST")
		if !ok {
			t.Fatalf("Couldn't find the URL")
		}

		wc := goetcd.NewClient([]string{s.URL()})
		testResp, err := wc.Set("test", "0", 0)

		if err != nil {
			t.Fatalf("Couldn't set a test key on the leader %v", err)
		}

		v = url.Values{}
		v.Set("value", u)
		resp, err = etcdtest.PutForm(fmt.Sprintf("%s%s", s.URL(), "/v2/keys/_etcd/registry/3/ETCDTEST"), v)
		assert.Equal(t, resp.StatusCode, http.StatusCreated)

		proc, err := startServer([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/3"})
		if err != nil {
			t.Fatal(err.Error())
		}
		defer stopServer(proc)

		watch := fmt.Sprintf("%s%s%d", s.URL(), "/v2/keys/_etcd/registry/3/node1?wait=true&waitIndex=", testResp.EtcdIndex)
		resp, err = http.Get(watch)
		if err != nil {
			t.Fatal(err.Error())
		}

		// TODO(bp): need to have a better way of knowing a machine is up
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)

			etcdc := goetcd.NewClient(nil)
			_, err = etcdc.Set("foobar", "baz", 0)
			if err == nil {
				break
			}
		}

		if err != nil {
			t.Fatal(err.Error())
		}
	})
}

// TestDiscoveryRestart ensures that a discovery cluster could be restarted.
func TestDiscoveryRestart(t *testing.T) {
	etcdtest.RunServer(func(s *server.Server) {
		proc, err := startServer([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/4"})
		if err != nil {
			t.Fatal(err.Error())
		}

		client := http.Client{}
		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}

		proc2, err := startServer2([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/4", "-addr", "127.0.0.1:4002", "-peer-addr", "127.0.0.1:7002"})
		if err != nil {
			t.Fatal(err.Error())
		}

		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}

		stopServer(proc)
		stopServer(proc2)

		proc, err = startServerWithDataDir([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/4"})
		if err != nil {
			t.Fatal(err.Error())
		}
		proc2, err = startServer2WithDataDir([]string{"-discovery", s.URL() + "/v2/keys/_etcd/registry/4", "-addr", "127.0.0.1:4002", "-peer-addr", "127.0.0.1:7002"})
		if err != nil {
			t.Fatal(err.Error())
		}

		err = assertServerFunctional(client, "http")
		if err != nil {
			t.Fatal(err.Error())
		}

		stopServer(proc)
		stopServer(proc2)
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
