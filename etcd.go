package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime/pprof"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

func main() {
	parseFlags()

	// Load configuration.
	var config = server.NewConfig()
	if err := config.Load(os.Args[1:]); err != nil {
		log.Fatal("Configuration error:", err)
	}

	// Turn on logging.
	if config.VeryVerbose {
		log.Verbose = true
		raft.SetLogLevel(raft.Debug)
	} else if config.Verbose {
		log.Verbose = true
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
	if info.Name == "" {
		host, err := os.Hostname()
		if err != nil || host == "" {
			log.Fatal("Machine name required and hostname not set. e.g. '-n=machine_name'")
		}
		log.Warnf("Using hostname %s as the machine name. You must ensure this name is unique among etcd machines.", host)
		info.Name = host
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
	ps := server.NewPeerServer(info.Name, config.DataDir, info.RaftURL, info.RaftListenHost, &peerTLSConfig, &info.RaftTLS, registry, store, config.SnapCount)
	ps.MaxClusterSize = config.MaxClusterSize
	ps.RetryTimes = config.MaxRetryAttempts

	// Create client server.
	s := server.New(info.Name, info.EtcdURL, info.EtcdListenHost, &tlsConfig, &info.EtcdTLS, ps, registry, store)
	if err := s.AllowOrigins(config.Cors); err != nil {
		panic(err)
	}

	ps.SetServer(s)

	// Run peer server in separate thread while the client server blocks.
	go func() {
		log.Fatal(ps.ListenAndServe(config.Snapshot, config.Machines))
	}()
	log.Fatal(s.ListenAndServe())
}

// Parses non-configuration flags.
func parseFlags() {
	var versionFlag bool
	var cpuprofile string

	f := flag.NewFlagSet(os.Args[0], -1)
	f.SetOutput(ioutil.Discard)
	f.BoolVar(&versionFlag, "version", false, "print the version and exit")
	f.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")
	f.Parse(os.Args[1:])

	// Print version if necessary.
	if versionFlag {
		fmt.Println(server.ReleaseVersion)
		os.Exit(0)
	}

	// Begin CPU profiling if specified.
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			sig := <-c
			log.Infof("captured %v, stopping profiler and exiting..", sig)
			pprof.StopCPUProfile()
			os.Exit(1)
		}()
	}
}
