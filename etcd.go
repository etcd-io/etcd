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
	"os"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
	"github.com/coreos/raft"
)

func main() {
	// Load configuration.
	var config = server.NewConfig()
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
	if config.VeryVerbose {
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

	// Load info object.
	info, err := config.Info()
	if err != nil {
		log.Fatal("info:", err)
	}

	// Retrieve TLS configuration.
	tlsConfig, err := info.EtcdTLS.Config()
	if err != nil {
		log.Fatal("Client TLS:", err)
	}
	peerTLSConfig, err := info.RaftTLS.Config()
	if err != nil {
		log.Fatal("Peer TLS:", err)
	}

	// Create etcd key-value store and registry.
	store := store.New()
	registry := server.NewRegistry(store)

	// Create peer server.
	ps := server.NewPeerServer(info.Name, config.DataDir, info.RaftURL, info.RaftListenHost, &peerTLSConfig, &info.RaftTLS, registry, store, config.SnapshotCount)
	ps.MaxClusterSize = config.MaxClusterSize
	ps.RetryTimes = config.MaxRetryAttempts

	// Create client server.
	s := server.New(info.Name, info.EtcdURL, info.EtcdListenHost, &tlsConfig, &info.EtcdTLS, ps, registry, store)
	if err := s.AllowOrigins(config.CorsOrigins); err != nil {
		panic(err)
	}

	ps.SetServer(s)

	// Run peer server in separate thread while the client server blocks.
	go func() {
		log.Fatal(ps.ListenAndServe(config.Snapshot, config.Peers))
	}()
	log.Fatal(s.ListenAndServe())
}
