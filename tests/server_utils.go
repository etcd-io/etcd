package tests

import (
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coreos/etcd/third_party/github.com/goraft/raft"

	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
)

const (
	testName              = "ETCDTEST"
	testClientURL         = "localhost:4401"
	testRaftURL           = "localhost:7701"
	testSnapshotCount     = 10000
	testHeartbeatInterval = time.Duration(50) * time.Millisecond
	testElectionTimeout   = time.Duration(200) * time.Millisecond
)

// Starts a server in a temporary directory.
func RunServer(f func(*server.Server)) {
	path, _ := ioutil.TempDir("", "etcd-")
	defer os.RemoveAll(path)

	store := store.New()
	registry := server.NewRegistry(store)

	serverStats := server.NewRaftServerStats(testName)
	followersStats := server.NewRaftFollowersStats(testName)

	psConfig := server.PeerServerConfig{
		Name:          testName,
		URL:           "http://" + testRaftURL,
		Scheme:        "http",
		SnapshotCount: testSnapshotCount,
	}

	mb := metrics.NewBucket("")

	ps := server.NewPeerServer(psConfig, registry, store, &mb, followersStats, serverStats)
	psListener := server.NewListener("http", testRaftURL, nil)

	// Create Raft transporter and server
	dialTimeout := (3 * testHeartbeatInterval) + testElectionTimeout
	responseHeaderTimeout := (3 * testHeartbeatInterval) + testElectionTimeout
	raftTransporter := server.NewTransporter(followersStats, serverStats, registry, testHeartbeatInterval, dialTimeout, responseHeaderTimeout)
	raftServer, err := raft.NewServer(testName, path, raftTransporter, store, ps, "")
	if err != nil {
		panic(err)
	}
	raftServer.SetElectionTimeout(testElectionTimeout)
	raftServer.SetHeartbeatInterval(testHeartbeatInterval)
	ps.SetRaftServer(raftServer)

	s := server.New(testName, "http://"+testClientURL, ps, registry, store, nil)
	sListener := server.NewListener("http", testClientURL, nil)

	ps.SetServer(s)

	w := &sync.WaitGroup{}

	// Start up peer server.
	c := make(chan bool)
	go func() {
		c <- true
		ps.Start(false, "", []string{})
		h := waitHandler{w, ps.HTTPHandler()}
		http.Serve(psListener, &h)
	}()
	<-c

	// Start up etcd server.
	go func() {
		c <- true
		h := waitHandler{w, s.HTTPHandler()}
		http.Serve(sListener, &h)
	}()
	<-c

	// Wait to make sure servers have started.
	time.Sleep(50 * time.Millisecond)

	// Execute the function passed in.
	f(s)

	// Clean up servers.
	ps.Stop()
	psListener.Close()
	sListener.Close()
	w.Wait()
}

type waitHandler struct {
	wg      *sync.WaitGroup
	handler http.Handler
}

func (h *waitHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.wg.Add(1)
	defer h.wg.Done()
	h.handler.ServeHTTP(w, r)

	//important to flush before decrementing the wait group.
	//we won't get a chance to once main() ends.
	w.(http.Flusher).Flush()
}
