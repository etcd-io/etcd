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

// TODO(yichengq): constant extraTimeout is a hack.
// Current problem is that there is big lag between join command
// execution and join success.
// Fix it later. It should be removed when proper method is found and
// enough tests are provided. It is expected to be calculated from
// heartbeatInterval and electionTimeout only.
const extraTimeout = time.Duration(1000) * time.Millisecond

type Etcd struct {
	Config *config.Config // etcd config

	Store         store.Store        // data store
	Registry      *server.Registry   // stores URL information for nodes
	Server        *server.Server     // http server, runs on 4001 by default
	PeerServer    *server.PeerServer // peer server, runs on 7001 by default
	StandbyServer *server.StandbyServer

	server     *http.Server
	peerServer *http.Server

	mode        Mode
	modeMutex   sync.Mutex
	closeChan   chan bool
	readyNotify chan bool // To signal when server is ready to accept connections
	onceReady   sync.Once
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
		DataDir:    e.Config.DataDir,
	}
	e.StandbyServer = server.NewStandbyServer(ssConfig, client)
	e.StandbyServer.SetRaftServer(raftServer)

	// Generating config could be slow.
	// Put it here to make listen happen immediately after peer-server starting.
	peerTLSConfig := server.TLSServerConfig(e.Config.PeerTLSInfo())
	etcdTLSConfig := server.TLSServerConfig(e.Config.EtcdTLSInfo())

	if !e.StandbyServer.IsRunning() {
		startPeerServer, possiblePeers, err := e.PeerServer.FindCluster(e.Config.Discovery, e.Config.Peers)
		if err != nil {
			log.Fatal(err)
		}
		if startPeerServer {
			e.setMode(PeerMode)
		} else {
			e.StandbyServer.SyncCluster(possiblePeers)
			e.setMode(StandbyMode)
		}
	} else {
		e.setMode(StandbyMode)
	}

	serverHTTPHandler := &ehttp.CORSHandler{e.Server.HTTPHandler(), corsInfo}
	peerServerHTTPHandler := &ehttp.CORSHandler{e.PeerServer.HTTPHandler(), corsInfo}
	standbyServerHTTPHandler := &ehttp.CORSHandler{e.StandbyServer.ClientHTTPHandler(), corsInfo}

	log.Infof("etcd server [name %s, listen on %s, advertised url %s]", e.Server.Name, e.Config.BindAddr, e.Server.URL())
	listener := server.NewListener(e.Config.EtcdTLSInfo().Scheme(), e.Config.BindAddr, etcdTLSConfig)

	e.server = &http.Server{Handler: &ModeHandler{e, serverHTTPHandler, standbyServerHTTPHandler},
		ReadTimeout:  time.Duration(e.Config.HTTPReadTimeout) * time.Second,
		WriteTimeout: time.Duration(e.Config.HTTPWriteTimeout) * time.Second,
	}

	log.Infof("peer server [name %s, listen on %s, advertised url %s]", e.PeerServer.Config.Name, e.Config.Peer.BindAddr, e.PeerServer.Config.URL)
	peerListener := server.NewListener(e.Config.PeerTLSInfo().Scheme(), e.Config.Peer.BindAddr, peerTLSConfig)

	e.peerServer = &http.Server{Handler: &ModeHandler{e, peerServerHTTPHandler, http.NotFoundHandler()},
		ReadTimeout:  time.Duration(server.DefaultReadTimeout) * time.Second,
		WriteTimeout: time.Duration(server.DefaultWriteTimeout) * time.Second,
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		<-e.readyNotify
		defer wg.Done()
		if err := e.server.Serve(listener); err != nil {
			if !isListenerClosing(err) {
				log.Fatal(err)
			}
		}
	}()
	go func() {
		<-e.readyNotify
		defer wg.Done()
		if err := e.peerServer.Serve(peerListener); err != nil {
			if !isListenerClosing(err) {
				log.Fatal(err)
			}
		}
	}()

	e.runServer()

	listener.Close()
	peerListener.Close()
	wg.Wait()
	log.Infof("etcd instance is stopped [name %s]", e.Config.Name)
	close(e.stopNotify)
}

func (e *Etcd) runServer() {
	var removeNotify <-chan bool
	for {
		if e.mode == PeerMode {
			log.Infof("%v starting in peer mode", e.Config.Name)
			// Starting peer server should be followed close by listening on its port
			// If not, it may leave many requests unaccepted, or cannot receive heartbeat from the cluster.
			// One severe problem caused if failing receiving heartbeats is when the second node joins one-node cluster,
			// the cluster could be out of work as long as the two nodes cannot transfer messages.
			e.PeerServer.Start(e.Config.Snapshot, e.Config.ClusterConfig())
			removeNotify = e.PeerServer.RemoveNotify()
		} else {
			log.Infof("%v starting in standby mode", e.Config.Name)
			e.StandbyServer.Start()
			removeNotify = e.StandbyServer.RemoveNotify()
		}

		// etcd server is ready to accept connections, notify waiters.
		e.onceReady.Do(func() { close(e.readyNotify) })

		select {
		case <-e.closeChan:
			e.PeerServer.Stop()
			e.StandbyServer.Stop()
			return
		case <-removeNotify:
		}

		if e.mode == PeerMode {
			peerURLs := e.Registry.PeerURLs(e.PeerServer.RaftServer().Leader(), e.Config.Name)
			e.StandbyServer.SyncCluster(peerURLs)
			e.setMode(StandbyMode)
		} else {
			// Create etcd key-value store and registry.
			e.Store = store.New()
			e.Registry = server.NewRegistry(e.Store)
			e.PeerServer.SetStore(e.Store)
			e.PeerServer.SetRegistry(e.Registry)
			e.Server.SetStore(e.Store)
			e.Server.SetRegistry(e.Registry)

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
			e.StandbyServer.SetRaftServer(raftServer)

			e.PeerServer.SetJoinIndex(e.StandbyServer.JoinIndex())
			e.setMode(PeerMode)
		}
	}
}

// Stop the etcd instance.
func (e *Etcd) Stop() {
	close(e.closeChan)
	<-e.stopNotify
}

// ReadyNotify returns a channel that is going to be closed
// when the etcd instance is ready to accept connections.
func (e *Etcd) ReadyNotify() <-chan bool {
	return e.readyNotify
}

func (e *Etcd) Mode() Mode {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()
	return e.mode
}

func (e *Etcd) setMode(m Mode) {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()
	e.mode = m
}

func isListenerClosing(err error) bool {
	// An error string equivalent to net.errClosing for using with
	// http.Serve() during server shutdown. Need to re-declare
	// here because it is not exported by "net" package.
	const errClosing = "use of closed network connection"

	return strings.Contains(err.Error(), errClosing)
}

type ModeGetter interface {
	Mode() Mode
}

type ModeHandler struct {
	ModeGetter
	PeerModeHandler    http.Handler
	StandbyModeHandler http.Handler
}

func (h *ModeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch h.Mode() {
	case PeerMode:
		h.PeerModeHandler.ServeHTTP(w, r)
	case StandbyMode:
		h.StandbyModeHandler.ServeHTTP(w, r)
	}
}

type Mode int

const (
	PeerMode Mode = iota
	StandbyMode
)
