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
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

const DefaultConfigFile = "/etc/etcd/etcd.toml"

var (
	config               *Config
	configFile           string
	cors                 string
	etcdAdvertisedUrl    string
	etcdCAFile           string
	etcdCPUProfileFile   string
	etcdCertFile         string
	etcdDataDir          string
	etcdKeyFile          string
	etcdListenHost       string
	etcdMaxClusterSize   int
	etcdMaxResultBuffer  int
	etcdMaxRetryAttempts int
	etcdName             string
	etcdSnapshot         bool
	etcdWebURL           string
	etcdVerbose          bool
	etcdVeryVerbose      bool
	machines             string
	machinesFile         string
	printVersion         bool
	raftAdvertisedUrl    string
	raftCAFile           string
	raftCertFile         string
	raftKeyFile          string
	raftListenHost       string
)

func init() {
	config = NewConfig()
	flag.StringVar(&configFile, "configFile", DefaultConfigFile, "the etcd config file")
	flag.StringVar(&cors, "cors", "", "comma seperated list of origins to whitelist for cross-origin resource sharing ('*' or 'http://localhost:8001',...)")
	flag.StringVar(&machines, "C", "", "comma seperated list of machines in the cluster ('hostname:port',...)")
	flag.StringVar(&machinesFile, "CF", "", "file containing a comma seperated list of machines in the cluster ('hostname:port',...)")
	flag.BoolVar(&printVersion, "version", false, "print the version and exit")
	// Etcd flags
	flag.StringVar(&etcdAdvertisedUrl, "c", "127.0.0.1:4001", "advertised 'hostname:port' for client-server communication")
	flag.StringVar(&etcdCAFile, "clientCAFile", "", "CA cert file for client-server communication")
	flag.StringVar(&etcdCPUProfileFile, "cpuprofile", "", "where to write cpu profile data")
	flag.StringVar(&etcdCertFile, "clientCert", "", "cert file for client-server communication")
	flag.StringVar(&etcdDataDir, "d", ".", "where to store the log and snapshots")
	flag.StringVar(&etcdKeyFile, "clientKey", "", "key file for client-server communication")
	flag.StringVar(&etcdListenHost, "cl", "", "listening 'hostname' for client-server communication")
	flag.IntVar(&etcdMaxClusterSize, "maxsize", 9, "maximum cluster size")
	flag.IntVar(&etcdMaxResultBuffer, "m", 1024, "maximum result buffer size")
	flag.IntVar(&etcdMaxRetryAttempts, "r", 3, "maximum number of attempts to join the cluster")
	flag.StringVar(&etcdName, "n", "", "node name (required)")
	flag.BoolVar(&etcdSnapshot, "snapshot", false, "open or close snapshot")
	flag.StringVar(&etcdWebURL, "w", "", "listening 'hostname:port' for the web interface")
	flag.BoolVar(&etcdVerbose, "v", false, "enable verbose logging")
	flag.BoolVar(&etcdVeryVerbose, "vv", false, "enable very verbose logging")
	// Raft flags
	flag.StringVar(&raftAdvertisedUrl, "s", "127.0.0.1:7001", "advertised 'hostname:port' for server-server communication")
	flag.StringVar(&raftCAFile, "serverCAFile", "", "CA cert file for server-server communication")
	flag.StringVar(&raftCertFile, "serverCert", "", "cert file for server-server communication")
	flag.StringVar(&raftKeyFile, "serverKey", "", "key file for server-server communication")
	flag.StringVar(&raftListenHost, "sl", "", "listening 'hostname' for server-server communication")
}

const (
	ElectionTimeout  = 200 * time.Millisecond
	HeartbeatTimeout = 50 * time.Millisecond
	RetryInterval    = 10
)

//------------------------------------------------------------------------------
//
// Variables
//
//------------------------------------------------------------------------------

var etcdStore *store.Store

//------------------------------------------------------------------------------
//
// Functions
//
//------------------------------------------------------------------------------

//--------------------------------------
// Main
//--------------------------------------

func main() {
	flag.Usage = Usage
	flag.Parse()

	if printVersion {
		fmt.Println(releaseVersion)
		os.Exit(0)
	}
	// Use the default configuration file unless one was defined by the user.
	if configFile == "" {
		configFile = os.Getenv("ETCD_CONFIG_FILE")
	}
	if configFile == "" {
		configFile = DefaultConfigFile
	}
	config.setConfigFile(configFile)
	// Set up the system wide etcd and raft configuration. From this point on
	// configuration settings can be accessed through the global config var.
	config.processConfig()

	if config.Etcd.CPUProfileFile != "" {
		runCPUProfile()
	}

	if config.Etcd.VeryVerbose {
		config.Etcd.Verbose = true
		raft.SetLogLevel(raft.Debug)
	}

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(config.Etcd.DataDir, 0744); err != nil {
		fatalf("Unable to create path: %s", err)
	}

	// Create etcd key-value store
	etcdStore = store.CreateStore(config.Etcd.MaxClusterSize)
	snapConf = newSnapshotConf()

	// Create etcd and raft server
	e = newEtcdServer(config.Etcd.Name, config.Etcd.AdvertisedUrl, config.Etcd.ListenHost, &config.Etcd.TLSConfig, &config.Etcd.TLSInfo)
	r = newRaftServer(config.Etcd.Name, config.Raft.AdvertisedUrl, config.Raft.ListenHost, &config.Raft.TLSConfig, &config.Raft.TLSInfo)

	startWebInterface()
	r.ListenAndServe()
	e.ListenAndServe()
}

// Usage prints the etcd usage to stderr.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage %s [-h] [options...]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.VisitAll(func(f *flag.Flag) {
		format := " -%-14s%s\n"
		fmt.Fprintf(os.Stderr, format, f.Name, f.Usage)
	})
}
