package main

import (
	"crypto/tls"
	"flag"
	"github.com/coreos/etcd/store"
	"github.com/coreos/go-raft"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var (
	verbose     bool
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
)

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.BoolVar(&veryVerbose, "vv", false, "very verbose logging")

	flag.StringVar(&machines, "C", "", "the ip address and port of a existing machines in the cluster, sepearate by comma")
	flag.StringVar(&machinesFile, "CF", "", "the file contains a list of existing machines in the cluster, seperate by comma")

	flag.StringVar(&argInfo.Name, "n", "default-name", "the node name (required)")
	flag.StringVar(&argInfo.EtcdURL, "c", "127.0.0.1:4001", "the hostname:port for etcd client communication")
	flag.StringVar(&argInfo.RaftURL, "s", "127.0.0.1:7001", "the hostname:port for raft server communication")
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
}

const (
	ElectionTimeout  = 200 * time.Millisecond
	HeartbeatTimeout = 50 * time.Millisecond

	// Timeout for internal raft http connection
	// The original timeout for http is 45 seconds
	// which is too long for our usage.
	HTTPTimeout   = 10 * time.Second
	RetryInterval = 10
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

type TLSInfo struct {
	CertFile string `json:"CertFile"`
	KeyFile  string `json:"KeyFile"`
	CAFile   string `json:"CAFile"`
}

type Info struct {
	Name string `json:"name"`

	RaftURL string `json:"raftURL"`
	EtcdURL string `json:"etcdURL"`
	WebURL  string `json:"webURL"`

	RaftTLS TLSInfo `json:"raftTLS"`
	EtcdTLS TLSInfo `json:"etcdTLS"`
}

type TLSConfig struct {
	Scheme string
	Server tls.Config
	Client tls.Config
}

type EtcdServer struct {
}

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
	flag.Parse()

	if cpuprofile != "" {
		runCPUProfile()
	}

	if veryVerbose {
		verbose = true
		raft.SetLogLevel(raft.Debug)
	}

	if machines != "" {
		cluster = strings.Split(machines, ",")
	} else if machinesFile != "" {
		b, err := ioutil.ReadFile(machinesFile)
		if err != nil {
			fatalf("Unable to read the given machines file: %s", err)
		}
		cluster = strings.Split(string(b), ",")
	}

	// Check TLS arguments
	raftTLSConfig, ok := tlsConfigFromInfo(argInfo.RaftTLS)
	if !ok {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	etcdTLSConfig, ok := tlsConfigFromInfo(argInfo.EtcdTLS)
	if !ok {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	argInfo.Name = strings.TrimSpace(argInfo.Name)
	if argInfo.Name == "" {
		fatal("ERROR: server name required. e.g. '-n=server_name'")
	}

	// Check host name arguments
	argInfo.RaftURL = sanitizeURL(argInfo.RaftURL, raftTLSConfig.Scheme)
	argInfo.EtcdURL = sanitizeURL(argInfo.EtcdURL, etcdTLSConfig.Scheme)
	argInfo.WebURL = sanitizeURL(argInfo.WebURL, "http")

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(dirPath, 0744); err != nil {
		fatalf("Unable to create path: %s", err)
	}

	info := getInfo(dirPath)

	// Create etcd key-value store
	etcdStore = store.CreateStore(maxSize)
	snapConf = newSnapshotConf()

	// Create etcd and raft server
	e = newEtcdServer(info.Name, info.EtcdURL, &etcdTLSConfig, &info.EtcdTLS)
	r = newRaftServer(info.Name, info.RaftURL, &raftTLSConfig, &info.RaftTLS)

	startWebInterface()
	r.ListenAndServe()
	e.ListenAndServe()

}
