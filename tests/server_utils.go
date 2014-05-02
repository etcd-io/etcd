package tests

import (
	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/etcd"
	"github.com/coreos/etcd/server"
)

const (
	testName              = "ETCDTEST"
	testClientURL         = "localhost:4401"
	testRaftURL           = "localhost:7701"
	testSnapshotCount     = 10000
	testHeartbeatInterval = 50
	testElectionTimeout   = 200
	testDataDir           = "/tmp/ETCDTEST"
)

// Starts a new server.
func RunServer(f func(*server.Server)) {
	c := config.New()

	c.Name = testName
	c.Addr = testClientURL
	c.Peer.Addr = testRaftURL

	c.DataDir = testDataDir
	c.Force = true

	c.Peer.HeartbeatInterval = testHeartbeatInterval
	c.Peer.ElectionTimeout = testElectionTimeout
	c.SnapshotCount = testSnapshotCount

	i := etcd.New(c)
	go i.Run()
	<-i.ReadyNotify()
	// Execute the function passed in.
	f(i.Server)
	i.Stop()
}
