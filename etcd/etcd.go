/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	goetcd "github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
	golog "github.com/coreos/etcd/third_party/github.com/coreos/go-log/log"
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
	httpclient "github.com/coreos/etcd/third_party/github.com/mreiferson/go-httpclient"

	"github.com/coreos/etcd/config"
	ehttp "github.com/coreos/etcd/http"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
)

// TODO(yichengq): constant extraTimeout is a hack.
// Current problem is that there is big lag between join command
// execution and join success.
// Fix it later. It should be removed when proper method is found and
// enough tests are provided. It is expected to be calculated from
// heartbeatInterval and electionTimeout only.
const extraTimeout = time.Duration(1000) * time.Millisecond

type Etcd struct {
	Config       *config.Config     // etcd config
	Store        store.Store        // data store
	Registry     *server.Registry   // stores URL information for nodes
	Server       *server.Server     // http server, runs on 4001 by default
	PeerServer   *server.PeerServer // peer server, runs on 7001 by default
	listener     net.Listener       // Listener for Server
	peerListener net.Listener       // Listener for PeerServer
	readyC       chan bool          // To signal when server is ready to accept connections
}

// New returns a new Etcd instance.
func New(c *config.Config) *Etcd {
	if c == nil {
		c = config.New()
	}
	return &Etcd{
		Config: c,
		readyC: make(chan bool),
	}
}

// Run the etcd instance.
func (e *Etcd) Run() {
	// Sanitize all the input fields.
	if err := e.Config.Sanitize(); err != nil {
		log.Fatalf("failed sanitizing configuration: %v", err)
	}

	// Force remove server configuration if specified.
	if e.Config.Force {
		e.Config.Reset()
	}

	// Enable options.
	if e.Config.VeryVeryVerbose {
		log.Verbose = true
		raft.SetLogLevel(raft.Trace)
		goetcd.SetLogger(
			golog.New(
				"go-etcd",
				false,
				golog.CombinedSink(
					os.Stdout,
					"[%s] %s %-9s | %s\n",
					[]string{"prefix", "time", "priority", "message"},
				),
			),
		)
	} else if e.Config.VeryVerbose {
		log.Verbose = true
		raft.SetLogLevel(raft.Debug)
	} else if e.Config.Verbose {
		log.Verbose = true
	}

	if e.Config.CPUProfileFile != "" {
		profile(e.Config.CPUProfileFile)
	}

	if e.Config.DataDir == "" {
		log.Fatal("The data dir was not set and could not be guessed from machine name")
	}

	// Create data directory if it doesn't already exist.
	if err := os.MkdirAll(e.Config.DataDir, 0744); err != nil {
		log.Fatalf("Unable to create path: %s", err)
	}

	// Warn people if they have an info file
	info := filepath.Join(e.Config.DataDir, "info")
	if _, err := os.Stat(info); err == nil {
		log.Warnf("All cached configuration is now ignored. The file %s can be removed.", info)
	}

	var mbName string
	if e.Config.Trace() {
		mbName = e.Config.MetricsBucketName()
		runtime.SetBlockProfileRate(1)
	}

	mb := metrics.NewBucket(mbName)

	if e.Config.GraphiteHost != "" {
		err := mb.Publish(e.Config.GraphiteHost)
		if err != nil {
			panic(err)
		}
	}

	// Retrieve CORS configuration
	corsInfo, err := ehttp.NewCORSInfo(e.Config.CorsOrigins)
	if err != nil {
		log.Fatal("CORS:", err)
	}

	// Create etcd key-value store and registry.
	e.Store = store.New()
	e.Registry = server.NewRegistry(e.Store)

	// Create stats objects
	followersStats := server.NewRaftFollowersStats(e.Config.Name)
	serverStats := server.NewRaftServerStats(e.Config.Name)

	// Calculate all of our timeouts
	heartbeatInterval := time.Duration(e.Config.Peer.HeartbeatInterval) * time.Millisecond
	electionTimeout := time.Duration(e.Config.Peer.ElectionTimeout) * time.Millisecond
	dialTimeout := (3 * heartbeatInterval) + electionTimeout
	responseHeaderTimeout := (3 * heartbeatInterval) + electionTimeout

	clientTransporter := &httpclient.Transport{
		ResponseHeaderTimeout: responseHeaderTimeout + extraTimeout,
		// This is a workaround for Transport.CancelRequest doesn't work on
		// HTTPS connections blocked. The patch for it is in progress,
		// and would be available in Go1.3
		// More: https://codereview.appspot.com/69280043/
		ConnectTimeout: dialTimeout + extraTimeout,
		RequestTimeout: responseHeaderTimeout + dialTimeout + 2*extraTimeout,
	}
	if e.Config.PeerTLSInfo().Scheme() == "https" {
		clientTLSConfig, err := e.Config.PeerTLSInfo().ClientConfig()
		if err != nil {
			log.Fatal("client TLS error: ", err)
		}
		clientTransporter.TLSClientConfig = clientTLSConfig
		clientTransporter.DisableCompression = true
	}
	client := server.NewClient(clientTransporter)

	// Create peer server
	psConfig := server.PeerServerConfig{
		Name:          e.Config.Name,
		Scheme:        e.Config.PeerTLSInfo().Scheme(),
		URL:           e.Config.Peer.Addr,
		SnapshotCount: e.Config.SnapshotCount,
		RetryTimes:    e.Config.MaxRetryAttempts,
		RetryInterval: e.Config.RetryInterval,
	}
	e.PeerServer = server.NewPeerServer(psConfig, client, e.Registry, e.Store, &mb, followersStats, serverStats)

	// Create raft transporter and server
	raftTransporter := server.NewTransporter(followersStats, serverStats, e.Registry, heartbeatInterval, dialTimeout, responseHeaderTimeout)
	if psConfig.Scheme == "https" {
		raftClientTLSConfig, err := e.Config.PeerTLSInfo().ClientConfig()
		if err != nil {
			log.Fatal("raft client TLS error: ", err)
		}
		raftTransporter.SetTLSConfig(*raftClientTLSConfig)
	}
	raftServer, err := raft.NewServer(e.Config.Name, e.Config.DataDir, raftTransporter, e.Store, e.PeerServer, "")
	if err != nil {
		log.Fatal(err)
	}
	raftServer.SetElectionTimeout(electionTimeout)
	raftServer.SetHeartbeatInterval(heartbeatInterval)
	e.PeerServer.SetRaftServer(raftServer)

	// Create etcd server
	e.Server = server.New(e.Config.Name, e.Config.Addr, e.PeerServer, e.Registry, e.Store, &mb)

	if e.Config.Trace() {
		e.Server.EnableTracing()
	}

	e.PeerServer.SetServer(e.Server)

	// Generating config could be slow.
	// Put it here to make listen happen immediately after peer-server starting.
	peerTLSConfig := server.TLSServerConfig(e.Config.PeerTLSInfo())
	etcdTLSConfig := server.TLSServerConfig(e.Config.EtcdTLSInfo())

	log.Infof("etcd server [name %s, listen on %s, advertised url %s]", e.Server.Name, e.Config.BindAddr, e.Server.URL())
	e.listener = server.NewListener(e.Config.EtcdTLSInfo().Scheme(), e.Config.BindAddr, etcdTLSConfig)

	// An error string equivalent to net.errClosing for using with
	// http.Serve() during server shutdown. Need to re-declare
	// here because it is not exported by "net" package.
	const errClosing = "use of closed network connection"

	peerServerClosed := make(chan bool)
	go func() {
		// Starting peer server should be followed close by listening on its port
		// If not, it may leave many requests unaccepted, or cannot receive heartbeat from the cluster.
		// One severe problem caused if failing receiving heartbeats is when the second node joins one-node cluster,
		// the cluster could be out of work as long as the two nodes cannot transfer messages.
		e.PeerServer.Start(e.Config.Snapshot, e.Config.Discovery, e.Config.Peers)

		go func() {
			select {
			case <-e.PeerServer.StopNotify():
			case <-e.PeerServer.RemoveNotify():
				log.Infof("peer server is removed")
				os.Exit(0)
			}
		}()

		log.Infof("peer server [name %s, listen on %s, advertised url %s]", e.PeerServer.Config.Name, e.Config.Peer.BindAddr, e.PeerServer.Config.URL)
		e.peerListener = server.NewListener(psConfig.Scheme, e.Config.Peer.BindAddr, peerTLSConfig)

		close(e.readyC) // etcd server is ready to accept connections, notify waiters.

		sHTTP := &ehttp.CORSHandler{e.PeerServer.HTTPHandler(), corsInfo}
		if err := http.Serve(e.peerListener, sHTTP); err != nil {
			if !strings.Contains(err.Error(), errClosing) {
				log.Fatal(err)
			}
		}
		close(peerServerClosed)
	}()

	sHTTP := &ehttp.CORSHandler{e.Server.HTTPHandler(), corsInfo}
	if err := http.Serve(e.listener, sHTTP); err != nil {
		if !strings.Contains(err.Error(), errClosing) {
			log.Fatal(err)
		}
	}

	<-peerServerClosed
	log.Infof("etcd instance is stopped [name %s]", e.Config.Name)
}

// Stop the etcd instance.
//
// TODO Shutdown gracefully.
func (e *Etcd) Stop() {
	e.PeerServer.Stop()
	e.peerListener.Close()
	e.listener.Close()
}

// ReadyNotify returns a channel that is going to be closed
// when the etcd instance is ready to accept connections.
func (e *Etcd) ReadyNotify() <-chan bool {
	return e.readyC
}
