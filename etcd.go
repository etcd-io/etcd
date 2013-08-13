package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/web"

	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var verbose bool
var veryVerbose bool

var machines string
var machinesFile string

var cluster []string

var argInfo Info
var dirPath string

var force bool

var maxSize int

var snapshot bool

var retryTimes int

var maxClusterSize int

var cpuprofile string

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

//------------------------------------------------------------------------------
//
// Variables
//
//------------------------------------------------------------------------------

var etcdStore *store.Store
var info *Info

//------------------------------------------------------------------------------
//
// Functions
//
//------------------------------------------------------------------------------

// sanitizeURL will cleanup a host string in the format hostname:port and
// attach a schema.
func sanitizeURL(host string, defaultScheme string) string {
	// Blank URLs are fine input, just return it
	if len(host) == 0 {
		return host
	}

	p, err := url.Parse(host)
	if err != nil {
		fatal(err)
	}

	// Make sure the host is in Host:Port format
	_, _, err = net.SplitHostPort(host)
	if err != nil {
		fatal(err)
	}

	p = &url.URL{Host: host, Scheme: defaultScheme}

	return p.String()
}

//--------------------------------------
// Main
//--------------------------------------

func main() {
	flag.Parse()

	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func() {
			for sig := range c {
				fmt.Printf("captured %v, stopping profiler and exiting..", sig)
				pprof.StopCPUProfile()
				os.Exit(1)
			}
		}()

	}

	if veryVerbose {
		verbose = true
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

	argInfo.RaftURL = sanitizeURL(argInfo.RaftURL, raftTLSConfig.Scheme)
	argInfo.EtcdURL = sanitizeURL(argInfo.EtcdURL, etcdTLSConfig.Scheme)
	argInfo.WebURL = sanitizeURL(argInfo.WebURL, "http")

	// Setup commands.
	registerCommands()

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(dirPath, 0744); err != nil {
		fatalf("Unable to create path: %s", err)
	}

	info = getInfo(dirPath)

	// Create etcd key-value store
	etcdStore = store.CreateStore(maxSize)
	snapConf = newSnapshotConf()

	startRaft(raftTLSConfig)

	if argInfo.WebURL != "" {
		// start web
		argInfo.WebURL = sanitizeURL(argInfo.WebURL, "http")
		go webHelper()
		go web.Start(raftServer, argInfo.WebURL)
	}

	startEtcdTransport(*info, etcdTLSConfig.Scheme, etcdTLSConfig.Server)

}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(scheme string, tlsConf tls.Config) transporter {
	t := transporter{}

	tr := &http.Transport{
		Dial: dialTimeout,
	}

	if scheme == "https" {
		tr.TLSClientConfig = &tlsConf
		tr.DisableCompression = true
	}

	t.client = &http.Client{Transport: tr}

	return t
}

// Dial with timeout
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, HTTPTimeout)
}

// Start to listen and response client command
func startEtcdTransport(info Info, scheme string, tlsConf tls.Config) {
	u, _ := url.Parse(info.EtcdURL)
	fmt.Printf("etcd server [%s] listening on %s\n", info.Name, u)

	etcdMux := http.NewServeMux()

	server := &http.Server{
		Handler:   etcdMux,
		TLSConfig: &tlsConf,
		Addr:      u.Host,
	}

	// external commands
	etcdMux.HandleFunc("/"+version+"/keys/", Multiplexer)
	etcdMux.HandleFunc("/"+version+"/watch/", WatchHttpHandler)
	etcdMux.HandleFunc("/leader", LeaderHttpHandler)
	etcdMux.HandleFunc("/machines", MachinesHttpHandler)
	etcdMux.HandleFunc("/", VersionHttpHandler)
	etcdMux.HandleFunc("/stats", StatsHttpHandler)
	etcdMux.HandleFunc("/test/", TestHttpHandler)

	if scheme == "http" {
		fatal(server.ListenAndServe())
	} else {
		fatal(server.ListenAndServeTLS(info.EtcdTLS.CertFile, info.EtcdTLS.KeyFile))
	}
}

//--------------------------------------
// Config
//--------------------------------------

func tlsConfigFromInfo(info TLSInfo) (t TLSConfig, ok bool) {
	var keyFile, certFile, CAFile string
	var tlsCert tls.Certificate
	var err error

	t.Scheme = "http"

	keyFile = info.KeyFile
	certFile = info.CertFile
	CAFile = info.CAFile

	// If the user do not specify key file, cert file and
	// CA file, the type will be HTTP
	if keyFile == "" && certFile == "" && CAFile == "" {
		return t, true
	}

	// both the key and cert must be present
	if keyFile == "" || certFile == "" {
		return t, false
	}

	tlsCert, err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fatal(err)
	}

	t.Scheme = "https"
	t.Server.ClientAuth, t.Server.ClientCAs = newCertPool(CAFile)

	// The client should trust the RootCA that the Server uses since
	// everyone is a peer in the network.
	t.Client.Certificates = []tls.Certificate{tlsCert}
	t.Client.RootCAs = t.Server.ClientCAs

	return t, true
}

func parseInfo(path string) *Info {
	file, err := os.Open(path)

	if err != nil {
		return nil
	}

	info := &Info{}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		fatalf("Unable to read info: %v", err)
		return nil
	}

	if err = json.Unmarshal(content, &info); err != nil {
		fatalf("Unable to parse info: %v", err)
		return nil
	}

	return info
}

// Get the server info from previous conf file
// or from the user
func getInfo(path string) *Info {

	// Read in the server info if available.
	infoPath := filepath.Join(path, "info")

	// Delete the old configuration if exist
	if force {
		logPath := filepath.Join(path, "log")
		confPath := filepath.Join(path, "conf")
		snapshotPath := filepath.Join(path, "snapshot")
		os.Remove(infoPath)
		os.Remove(logPath)
		os.Remove(confPath)
		os.RemoveAll(snapshotPath)
	}

	info := parseInfo(infoPath)
	if info != nil {
		fmt.Printf("Found node configuration in '%s'. Ignoring flags.\n", infoPath)
		return info
	}

	info = &argInfo

	// Write to file.
	content, _ := json.MarshalIndent(info, "", " ")
	content = []byte(string(content) + "\n")
	if err := ioutil.WriteFile(infoPath, content, 0644); err != nil {
		fatalf("Unable to write info to file: %v", err)
	}

	fmt.Printf("Wrote node configuration to '%s'.\n", infoPath)

	return info
}

// newCertPool creates x509 certPool and corresponding Auth Type.
// If the given CAfile is valid, add the cert into the pool and verify the clients'
// certs against the cert in the pool.
// If the given CAfile is empty, do not verify the clients' cert.
// If the given CAfile is not valid, fatal.
func newCertPool(CAFile string) (tls.ClientAuthType, *x509.CertPool) {
	if CAFile == "" {
		return tls.NoClientCert, nil
	}
	pemByte, _ := ioutil.ReadFile(CAFile)

	block, pemByte := pem.Decode(pemByte)

	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		fatal(err)
	}

	certPool := x509.NewCertPool()

	certPool.AddCert(cert)

	return tls.RequireAndVerifyClientCert, certPool
}
