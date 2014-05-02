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
	"sync"
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

type Etcd struct {
	Config *config.Config // etcd config

	Store         store.Store        // data store
	Registry      *server.Registry   // stores URL information for nodes
	Server        *server.Server     // http server, runs on 4001 by default
	PeerServer    *server.PeerServer // peer server, runs on 7001 by default
	StandbyServer *server.StandbyServer
	corsInfo      *ehttp.CORSInfo
	client        *server.Client

	server       *http.Server
	peerServer   *http.Server
	listener     net.Listener // Listener for Server
	peerListener net.Listener // Listener for PeerServer

	mode        Mode
	closeChan   chan bool
	readyNotify chan bool // To signal when server is ready to accept connections
	stopNotify  chan bool // To signal when server is stopped totally
}

// New returns a new Etcd instance.
func New(c *config.Config) *Etcd {
	if c == nil {
		c = config.New()
	}
	return &Etcd{
		Config:      c,
		closeChan:   make(chan bool),
		readyNotify: make(chan bool),
		stopNotify:  make(chan bool),
	}
}

// Run the etcd instance.
func (e *Etcd) Run() {
	defer close(e.stopNotify)

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
	var err error
	e.corsInfo, err = ehttp.NewCORSInfo(e.Config.CorsOrigins)
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

	// TODO(yichengq): constant 1000 is a hack here.
	// Current problem is that there is big lag between join command
	// execution and join success.
	// Fix it later. It should be removed when proper method is found and
	// enough tests are provided.
	clientTransporter := &httpclient.Transport{
		ResponseHeaderTimeout: responseHeaderTimeout + 1000,
		// This is a workaround for Transport.CancelRequest doesn't work on
		// HTTPS connections blocked. The patch for it is in progress,
		// and would be available in Go1.3
		// More: https://codereview.appspot.com/69280043/
		ConnectTimeout: dialTimeout + 1000,
		RequestTimeout: responseHeaderTimeout + dialTimeout + 2000,
	}
	if e.Config.PeerTLSInfo().Scheme() == "https" {
		clientTLSConfig, err := e.Config.PeerTLSInfo().ClientConfig()
		if err != nil {
			log.Fatal("client TLS error: ", err)
		}
		clientTransporter.TLSClientConfig = clientTLSConfig
		clientTransporter.DisableCompression = true
	}
	e.client = server.NewClient(clientTransporter)

	// Create peer server
	psConfig := server.PeerServerConfig{
		Name:          e.Config.Name,
		Scheme:        e.Config.PeerTLSInfo().Scheme(),
		URL:           e.Config.Peer.Addr,
		SnapshotCount: e.Config.SnapshotCount,
		RetryTimes:    e.Config.MaxRetryAttempts,
		RetryInterval: e.Config.RetryInterval,
	}
	e.PeerServer = server.NewPeerServer(psConfig, e.client, e.Registry, e.Store, &mb, followersStats, serverStats)

	// Create raft transporter and server
	raftTransporter := server.NewTransporter(followersStats, serverStats, e.Registry, heartbeatInterval, dialTimeout, responseHeaderTimeout)
	if e.Config.PeerTLSInfo().Scheme() == "https" {
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
	e.PeerServer.SetRaftServer(raftServer, e.Config.Snapshot)

	// Create etcd server
	e.Server = server.New(e.Config.Name, e.Config.Addr, e.PeerServer, e.Registry, e.Store, &mb)

	if e.Config.Trace() {
		e.Server.EnableTracing()
	}

	e.PeerServer.SetServer(e.Server)

	// Create standby server
	ssConfig := server.StandbyServerConfig{
		Name:       e.Config.Name,
		PeerScheme: e.Config.PeerTLSInfo().Scheme(),
		PeerURL:    e.Config.Peer.Addr,
		ClientURL:  e.Config.Addr,
	}
	e.StandbyServer = server.NewStandbyServer(ssConfig, e.client)

	// Generating config could be slow.
	// Put it here to make listen happen immediately after peer-server starting.
	etcdTLSConfig := server.TLSServerConfig(e.Config.EtcdTLSInfo())
	peerTLSConfig := server.TLSServerConfig(e.Config.PeerTLSInfo())

	useStandbyMode, possiblePeers, err := e.PeerServer.FindCluster(e.Config.Discovery, e.Config.Peers)
	if err != nil {
		log.Fatal(err)
	}

	if useStandbyMode {
		e.StandbyServer.SyncCluster(possiblePeers)
		e.mode = StandbyMode
	} else {
		e.mode = PeerMode
	}

	log.Infof("etcd server [name %s, listen on %s, advertised url %s]", e.Server.Name, e.Config.BindAddr, e.Server.URL())
	e.listener = server.NewListener(e.Config.EtcdTLSInfo().Scheme(), e.Config.BindAddr, etcdTLSConfig)
	e.server = &http.Server{}
	log.Infof("peer server [name %s, listen on %s, advertised url %s]", e.PeerServer.Config.Name, e.Config.Peer.BindAddr, e.PeerServer.Config.URL)
	e.peerListener = server.NewListener(e.Config.PeerTLSInfo().Scheme(), e.Config.Peer.BindAddr, peerTLSConfig)
	e.peerServer = &http.Server{}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		<-e.readyNotify
		defer wg.Done()
		if err := e.server.Serve(e.listener); err != nil {
			if !isListenerClosing(err) {
				log.Fatal(err)
			}
		}
	}()
	go func() {
		<-e.readyNotify
		defer wg.Done()
		if err := e.peerServer.Serve(e.peerListener); err != nil {
			if !isListenerClosing(err) {
				log.Fatal(err)
			}
		}
	}()

	for {
		if e.mode == PeerMode {
			log.Infof("%v starts to run in peer mode", e.Config.Name)
			e.runPeerMode()
		} else {
			log.Infof("%v starts to run in standby mode", e.Config.Name)
			e.runStandbyMode()
		}

		select {
		case <-e.closeChan:
			e.listener.Close()
			e.peerListener.Close()
			wg.Wait()
			log.Infof("etcd instance is stopped [name %s]", e.Config.Name)
			return
		default:
		}
	}
}

// Note: Starting peer server should be followed close by listening on its port
// If not, it may leave many requests unaccepted, or cannot receive heartbeat from the cluster.
// One severe problem caused if failing receiving heartbeats is when the second node joins one-node cluster,
// the cluster could be out of work as long as the two nodes cannot transfer messages.
//
// Note: Peer server should be started quickly after join request success.
func (e *Etcd) runPeerMode() {
	e.PeerServer.Start(e.Config.Snapshot)

	e.server.Handler = &ehttp.CORSHandler{e.Server.HTTPHandler(), e.corsInfo}
	e.peerServer.Handler = &ehttp.CORSHandler{e.PeerServer.HTTPHandler(), e.corsInfo}

	// etcd server is ready to accept connections, notify waiters.
	select {
	case <-e.readyNotify:
	default:
		close(e.readyNotify)
	}

	select {
	case <-e.closeChan:
		e.PeerServer.Stop()
		return
	case <-e.PeerServer.RemoveNotify():
	}

	e.StandbyServer.SyncCluster(e.Registry.PeerURLs(e.PeerServer.RaftServer().Leader(), e.Config.Name))
	e.mode = StandbyMode
}

func (e *Etcd) runStandbyMode() {
	e.StandbyServer.Start()

	e.server.Handler = &ehttp.CORSHandler{e.StandbyServer.ClientHTTPHandler(), e.corsInfo}
	e.peerServer.Handler = http.NotFoundHandler()

	// etcd server is ready to accept connections, notify waiters.
	select {
	case <-e.readyNotify:
	default:
		close(e.readyNotify)
	}

	select {
	case <-e.closeChan:
		e.StandbyServer.Stop()
		return
	case <-e.StandbyServer.RemoveNotify():
	}

	// Generate new peer server here.
	// TODO(yichengq): raft server cannot be started after stopped.
	// It should be removed when raft restart is implemented.
	heartbeatInterval := time.Duration(e.Config.Peer.HeartbeatInterval) * time.Millisecond
	electionTimeout := time.Duration(e.Config.Peer.ElectionTimeout) * time.Millisecond
	raftServer, err := raft.NewServer(e.Config.Name, e.Config.DataDir, e.PeerServer.RaftServer().Transporter(), e.Store, e.PeerServer, "")
	if err != nil {
		log.Fatal(err)
	}
	raftServer.SetElectionTimeout(electionTimeout)
	raftServer.SetHeartbeatInterval(heartbeatInterval)
	e.PeerServer.SetRaftServer(raftServer, e.Config.Snapshot)

	e.PeerServer.SetJoinIndex(e.StandbyServer.JoinIndex())
	e.mode = PeerMode
}

// Stop the etcd instance.
//
// TODO Shutdown gracefully.
func (e *Etcd) Stop() {
	close(e.closeChan)
	<-e.stopNotify
}

// ReadyNotify returns a channel that is going to be closed
// when the etcd instance is ready to accept connections.
func (e *Etcd) ReadyNotify() <-chan bool {
	return e.readyNotify
}

func isListenerClosing(err error) bool {
	// An error string equivalent to net.errClosing for using with
	// http.Serve() during server shutdown. Need to re-declare
	// here because it is not exported by "net" package.
	const errClosing = "use of closed network connection"

	return strings.Contains(err.Error(), errClosing)
}

type Mode int

const (
	PeerMode Mode = iota
	StandbyMode
)
