package tests

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
)

const (
	testName          = "ETCDTEST"
	testClientURL     = "localhost:4401"
	testRaftURL       = "localhost:7701"
	testSnapshotCount = 10000
)

// Starts a server in a temporary directory.
func RunServer(f func(*server.Server)) {
	path, _ := ioutil.TempDir("", "etcd-")
	defer os.RemoveAll(path)

	store := store.New()
	registry := server.NewRegistry(store)
	ps := server.NewPeerServer(testName, path, "http://" + testRaftURL, testRaftURL, &server.TLSConfig{Scheme: "http"}, &server.TLSInfo{}, registry, store, testSnapshotCount)
	ps.MaxClusterSize = 9
	s := server.New(testName, "http://" + testClientURL, testClientURL, &server.TLSConfig{Scheme: "http"}, &server.TLSInfo{}, ps, registry, store)
	ps.SetServer(s)

	// Start up peer server.
	c := make(chan bool)
	go func() {
		c <- true
		ps.ListenAndServe(false, []string{})
	}()
	<-c

	// Start up etcd server.
	go func() {
		c <- true
		s.ListenAndServe()
	}()
	<-c

	// Wait to make sure servers have started.
	time.Sleep(50 * time.Millisecond)

	// Execute the function passed in.
	f(s)

	// Clean up servers.
	ps.Close()
	s.Close()
}
