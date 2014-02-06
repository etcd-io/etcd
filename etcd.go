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

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/coreos/etcd/third_party/github.com/coreos/raft"

	"github.com/coreos/etcd/config"
	ehttp "github.com/coreos/etcd/http"
	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/metrics"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
)

func main() {
	// Load configuration.
	var config = config.New()
	if err := config.Load(os.Args[1:]); err != nil {
		fmt.Println(server.Usage() + "\n")
		fmt.Println(err.Error() + "\n")
		os.Exit(1)
	} else if config.ShowVersion {
		fmt.Println(server.ReleaseVersion)
		os.Exit(0)
	} else if config.ShowHelp {
		fmt.Println(server.Usage() + "\n")
		os.Exit(0)
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
		log.Fatal("The data dir was not set and could not be guessed from machine name")
	}

	// Create data directory if it doesn't already exist.
	if err := os.MkdirAll(config.DataDir, 0744); err != nil {
		log.Fatalf("Unable to create path: %s", err)
	}

	// Warn people if they have an info file
	info := filepath.Join(config.DataDir, "info")
	if _, err := os.Stat(info); err == nil {
		log.Warnf("All cached configuration is now ignored. The file %s can be removed.", info)
	}

	// Retrieve TLS configuration.
	tlsConfig, err := config.TLSInfo().Config()
	if err != nil {
		log.Fatal("Client TLS:", err)
	}
	peerTLSConfig, err := config.PeerTLSInfo().Config()
	if err != nil {
		log.Fatal("Peer TLS:", err)
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
		log.Fatal("CORS:", err)
	}

	// Create etcd key-value store and registry.
	store := store.New()
	registry := server.NewRegistry(store)

	// Create stats objects
	followersStats := server.NewRaftFollowersStats(config.Name)
	serverStats := server.NewRaftServerStats(config.Name)

	// Calculate all of our timeouts
	heartbeatTimeout := time.Duration(config.Peer.HeartbeatTimeout) * time.Millisecond
	electionTimeout := time.Duration(config.Peer.ElectionTimeout) * time.Millisecond
	dialTimeout := (3 * heartbeatTimeout) + electionTimeout
	responseHeaderTimeout := (3 * heartbeatTimeout) + electionTimeout

	// Create peer server.
	psConfig := server.PeerServerConfig{
		Name:           config.Name,
		Scheme:         peerTLSConfig.Scheme,
		URL:            config.Peer.Addr,
		SnapshotCount:  config.SnapshotCount,
		MaxClusterSize: config.MaxClusterSize,
		RetryTimes:     config.MaxRetryAttempts,
	}
	ps := server.NewPeerServer(psConfig, registry, store, &mb, followersStats, serverStats)

	var psListener net.Listener
	if psConfig.Scheme == "https" {
		psListener, err = server.NewTLSListener(&tlsConfig.Server, config.Peer.BindAddr, config.PeerTLSInfo().CertFile, config.PeerTLSInfo().KeyFile)
	} else {
		psListener, err = server.NewListener(config.Peer.BindAddr)
	}
	if err != nil {
		panic(err)
	}

	// Create Raft transporter and server
	raftTransporter := server.NewTransporter(followersStats, serverStats, registry, heartbeatTimeout, dialTimeout, responseHeaderTimeout)
	if psConfig.Scheme == "https" {
		raftTransporter.SetTLSConfig(peerTLSConfig.Client)
	}
	raftServer, err := raft.NewServer(config.Name, config.DataDir, raftTransporter, store, ps, "")
	if err != nil {
		log.Fatal(err)
	}
	raftServer.SetElectionTimeout(electionTimeout)
	raftServer.SetHeartbeatTimeout(heartbeatTimeout)
	ps.SetRaftServer(raftServer)

	// Create client server.
	s := server.New(config.Name, config.Addr, ps, registry, store, &mb)

	if config.Trace() {
		s.EnableTracing()
	}

	var sListener net.Listener
	if tlsConfig.Scheme == "https" {
		sListener, err = server.NewTLSListener(&tlsConfig.Server, config.BindAddr, config.TLSInfo().CertFile, config.TLSInfo().KeyFile)
	} else {
		sListener, err = server.NewListener(config.BindAddr)
	}
	if err != nil {
		panic(err)
	}

	ps.SetServer(s)

	ps.Start(config.Snapshot, config.Peers)

	// Run peer server in separate thread while the client server blocks.
	go func() {
		log.Infof("raft server [name %s, listen on %s, advertised url %s]", ps.Config.Name, psListener.Addr(), ps.Config.URL)
		sHTTP := &ehttp.CORSHandler{ps.HTTPHandler(), corsInfo}
		log.Fatal(http.Serve(psListener, sHTTP))
	}()

	log.Infof("etcd server [name %s, listen on %s, advertised url %s]", s.Name, sListener.Addr(), s.URL())
	sHTTP := &ehttp.CORSHandler{s.HTTPHandler(), corsInfo}
	log.Fatal(http.Serve(sListener, sHTTP))
}
