package main

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"

	"github.com/coreos/etcd/log"
	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var (
	veryVerbose bool

	machines     string
	machinesFile string

	cluster []string

	argInfo Info
	dirPath string

	force bool

	maxSize int

	snapshot bool

	retryTimes int

	maxClusterSize int

	cpuprofile string

	cors string
)

func init() {
	flag.BoolVar(&log.Verbose, "v", false, "verbose logging")
	flag.BoolVar(&veryVerbose, "vv", false, "very verbose logging")

	flag.StringVar(&machines, "C", "", "the ip address and port of a existing machines in the cluster, sepearate by comma")
	flag.StringVar(&machinesFile, "CF", "", "the file contains a list of existing machines in the cluster, seperate by comma")

	flag.StringVar(&argInfo.Name, "n", "default-name", "the node name (required)")
	flag.StringVar(&argInfo.EtcdURL, "c", "127.0.0.1:4001", "the advertised public hostname:port for etcd client communication")
	flag.StringVar(&argInfo.RaftURL, "s", "127.0.0.1:7001", "the advertised public hostname:port for raft server communication")
	flag.StringVar(&argInfo.EtcdListenHost, "cl", "", "the listening hostname for etcd client communication (defaults to advertised ip)")
	flag.StringVar(&argInfo.RaftListenHost, "sl", "", "the listening hostname for raft server communication (defaults to advertised ip)")
	flag.StringVar(&argInfo.WebURL, "w", "", "the hostname:port of web interface")

	flag.StringVar(&argInfo.RaftTLS.CAFile, "serverCAFile", "", "the path of the CAFile")
	flag.StringVar(&argInfo.RaftTLS.CertFile, "serverCert", "", "the cert file of the server")
	flag.StringVar(&argInfo.RaftTLS.KeyFile, "serverKey", "", "the key file of the server")

	flag.StringVar(&argInfo.EtcdTLS.CAFile, "clientCAFile", "", "the path of the client CAFile")
	flag.StringVar(&argInfo.EtcdTLS.CertFile, "clientCert", "", "the cert file of the client")
	flag.StringVar(&argInfo.EtcdTLS.KeyFile, "clientKey", "", "the key file of the client")

	flag.StringVar(&dirPath, "d", ".", "the directory to store log and snapshot")

	flag.BoolVar(&force, "f", false, "force new node configuration if existing is found (WARNING: data loss!)")

	flag.BoolVar(&snapshot, "snapshot", false, "open or close snapshot")

	flag.IntVar(&maxSize, "m", 1024, "the max size of result buffer")

	flag.IntVar(&retryTimes, "r", 3, "the max retry attempts when trying to join a cluster")

	flag.IntVar(&maxClusterSize, "maxsize", 9, "the max size of the cluster")

	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")

	flag.StringVar(&cors, "cors", "", "whitelist origins for cross-origin resource sharing (e.g. '*' or 'http://localhost:8001,etc')")
}

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

type Info struct {
	Name string `json:"name"`

	RaftURL string `json:"raftURL"`
	EtcdURL string `json:"etcdURL"`
	WebURL  string `json:"webURL"`

	RaftListenHost string `json:"raftListenHost"`
	EtcdListenHost string `json:"etcdListenHost"`

	RaftTLS server.TLSInfo `json:"raftTLS"`
	EtcdTLS server.TLSInfo `json:"etcdTLS"`
}

//------------------------------------------------------------------------------
//
// Functions
//
//------------------------------------------------------------------------------

//--------------------------------------
// Main
//--------------------------------------

func main() {
	flag.Parse()

	if cpuprofile != "" {
		runCPUProfile()
	}

	if veryVerbose {
		log.Verbose = true
		raft.SetLogLevel(raft.Debug)
	}

	if machines != "" {
		cluster = strings.Split(machines, ",")
	} else if machinesFile != "" {
		b, err := ioutil.ReadFile(machinesFile)
		if err != nil {
			log.Fatalf("Unable to read the given machines file: %s", err)
		}
		cluster = strings.Split(string(b), ",")
	}

	// Check TLS arguments
	raftTLSConfig, ok := tlsConfigFromInfo(argInfo.RaftTLS)
	if !ok {
		log.Fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	etcdTLSConfig, ok := tlsConfigFromInfo(argInfo.EtcdTLS)
	if !ok {
		log.Fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	argInfo.Name = strings.TrimSpace(argInfo.Name)
	if argInfo.Name == "" {
		log.Fatal("ERROR: server name required. e.g. '-n=server_name'")
	}

	// Check host name arguments
	argInfo.RaftURL = sanitizeURL(argInfo.RaftURL, raftTLSConfig.Scheme)
	argInfo.EtcdURL = sanitizeURL(argInfo.EtcdURL, etcdTLSConfig.Scheme)
	argInfo.WebURL = sanitizeURL(argInfo.WebURL, "http")

	argInfo.RaftListenHost = sanitizeListenHost(argInfo.RaftListenHost, argInfo.RaftURL)
	argInfo.EtcdListenHost = sanitizeListenHost(argInfo.EtcdListenHost, argInfo.EtcdURL)

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(dirPath, 0744); err != nil {
		log.Fatalf("Unable to create path: %s", err)
	}

	info := getInfo(dirPath)

	// Create etcd key-value store
	store := store.New()

	// Create a shared node registry.
	registry := server.NewRegistry(store)

	// Create peer server.
	ps := server.NewPeerServer(info.Name, dirPath, info.RaftURL, info.RaftListenHost, &raftTLSConfig, &info.RaftTLS, registry, store)
	ps.MaxClusterSize = maxClusterSize
	ps.RetryTimes = retryTimes

	s := server.New(info.Name, info.EtcdURL, info.EtcdListenHost, &etcdTLSConfig, &info.EtcdTLS, ps.Server, registry, store)
	if err := s.AllowOrigins(cors); err != nil {
		panic(err)
	}

	ps.SetServer(s)

	ps.ListenAndServe(snapshot, cluster)
	s.ListenAndServe()
}
