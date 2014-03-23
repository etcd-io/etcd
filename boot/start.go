package boot

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/raft"

	"github.com/coreos/etcd/config"
	ehttp "github.com/coreos/etcd/http"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
)

// Run runs etcd with given config
func Run(config *config.Config) error {
	_, _, wait, running, err := Start(config)
	if !running {
		return err
	}
	wait()
	return nil
}

// Start starts etcd based on given config
// When it ends, etcd has listened on client port and peer port, and starts to serve requests.
// Return value stop indicates the function to stop the etcd.
// Return value wait indicates the function to wait until the etcd ends.
// Return value running implies whether etcd is running now. It is true iff there's no error.
func Start(config *config.Config) (s *server.Server, stop func(), wait func(), running bool, e error) {
	// Sanitize all the input fields.
	if err := config.Sanitize(); err != nil {
		e = fmt.Errorf("Failed to sanitize configuration: %v", err)
		return
	}
	// Force remove server configuration if specified.
	if config.Force {
		config.Reset()
	}

	if config.ShowVersion {
		fmt.Println("etcd version", server.ReleaseVersion)
		return
	}
	if config.ShowHelp {
		fmt.Println(server.Usage() + "\n")
		return
	}

	// Enable options.
	if config.VeryVeryVerbose {
		log.Verbose = true
		raft.SetLogLevel(raft.Trace)
	} else if config.VeryVerbose {
		log.Verbose = true
		raft.SetLogLevel(raft.Debug)
	} else if config.Verbose {
		log.Verbose = true
	}
	if config.CPUProfileFile != "" {
		profile(config.CPUProfileFile)
	}

	if config.DataDir == "" {
		e = fmt.Errorf("The data dir was not set and could not be guessed from machine name")
		return
	}

	// Create data directory if it doesn't already exist.
	if err := os.MkdirAll(config.DataDir, 0744); err != nil {
		e = fmt.Errorf("Unable to create path: %s", err)
		return
	}

	// Warn people if they have an info file
	info := filepath.Join(config.DataDir, "info")
	if _, err := os.Stat(info); err == nil {
		log.Warnf("All cached configuration is now ignored. The file %s can be removed.", info)
	}

	var mbName string
	if config.Trace() {
		mbName = config.MetricsBucketName()
		runtime.SetBlockProfileRate(1)
	}

	mb := metrics.NewBucket(mbName)

	if config.GraphiteHost != "" {
		err := mb.Publish(config.GraphiteHost)
		if err != nil {
			panic(err)
		}
	}

	// Retrieve CORS configuration
	corsInfo, err := ehttp.NewCORSInfo(config.CorsOrigins)
	if err != nil {
		e = fmt.Errorf("CORS:", err)
		return
	}

	// Create etcd key-value store and registry.
	store := store.New()
	registry := server.NewRegistry(store)

	// Create stats objects
	followersStats := server.NewRaftFollowersStats(config.Name)
	serverStats := server.NewRaftServerStats(config.Name)

	// Calculate all of our timeouts
	heartbeatInterval := time.Duration(config.Peer.HeartbeatInterval) * time.Millisecond
	electionTimeout := time.Duration(config.Peer.ElectionTimeout) * time.Millisecond
	dialTimeout := (3 * heartbeatInterval) + electionTimeout
	responseHeaderTimeout := (3 * heartbeatInterval) + electionTimeout

	// Create peer server
	psConfig := server.PeerServerConfig{
		Name:           config.Name,
		Scheme:         config.PeerTLSInfo().Scheme(),
		URL:            config.Peer.Addr,
		SnapshotCount:  config.SnapshotCount,
		MaxClusterSize: config.MaxClusterSize,
		RetryTimes:     config.MaxRetryAttempts,
		RetryInterval:  config.RetryInterval,
	}
	ps := server.NewPeerServer(psConfig, registry, store, &mb, followersStats, serverStats)

	// Create raft transporter and server
	raftTransporter := server.NewTransporter(followersStats, serverStats, registry, heartbeatInterval, dialTimeout, responseHeaderTimeout)
	if psConfig.Scheme == "https" {
		raftClientTLSConfig, err := config.PeerTLSInfo().ClientConfig()
		if err != nil {
			e = fmt.Errorf("raft client TLS error: ", err)
			return
		}
		raftTransporter.SetTLSConfig(*raftClientTLSConfig)
	}
	raftServer, err := raft.NewServer(config.Name, config.DataDir, raftTransporter, store, ps, "")
	if err != nil {
		e = err
		return
	}
	raftServer.SetElectionTimeout(electionTimeout)
	raftServer.SetHeartbeatInterval(heartbeatInterval)
	ps.SetRaftServer(raftServer)

	// Create etcd server
	s = server.New(config.Name, config.Addr, ps, registry, store, &mb)

	if config.Trace() {
		s.EnableTracing()
	}

	ps.SetServer(s)
	ps.Start(config.Snapshot, config.Discovery, config.Peers)

	wg := &sync.WaitGroup{}
	// One for sListener serving goroutine, one for psListener serving goroutine
	wg.Add(2)

	// Listen on server and peer server port
	// This ensures that server could receive requests now.
	log.Infof("peer server [name %s, listen on %s, advertised url %s]", ps.Config.Name, config.Peer.BindAddr, ps.Config.URL)
	psListener := server.NewListener(psConfig.Scheme, config.Peer.BindAddr, config.PeerTLSInfo())
	log.Infof("etcd server [name %s, listen on %s, advertised url %s]", s.Name, config.BindAddr, s.URL())
	sListener := server.NewListener(config.EtcdTLSInfo().Scheme(), config.BindAddr, config.EtcdTLSInfo())

	go func() {
		defer wg.Done()

		sHTTP := &ehttp.CORSHandler{ps.HTTPHandler(), corsInfo}
		if err := http.Serve(psListener, sHTTP); err != nil {
			// Could be stopped in testing
			log.Debugf("Stop server service: %v", err)
		}
	}()

	go func() {
		defer wg.Done()

		sHTTP := &ehttp.CORSHandler{s.HTTPHandler(), corsInfo}
		if err := http.Serve(sListener, sHTTP); err != nil {
			log.Debugf("Stop peer server service: %v", err)
		}
	}()

	stop = func() {
		ps.Stop()
		psListener.Close()
		sListener.Close()
		wg.Wait()
	}
	wait = func() {
		wg.Wait()
	}
	running = true
	return
}
