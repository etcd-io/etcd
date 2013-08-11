package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/web"
	"github.com/coreos/go-raft"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
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

	flag.StringVar(&argInfo.Name, "n", "", "the node name (required)")
	flag.StringVar(&argInfo.EtcdURL, "c", "127.0.0.1:4001", "the hostname:port for etcd client communication")
	flag.StringVar(&argInfo.RaftURL, "s", "127.0.0.1:7001", "the hostname:port for raft server communication")
	flag.StringVar(&argInfo.WebURL, "w", "", "the hostname:port of web interface")

	flag.StringVar(&argInfo.ServerCAFile, "serverCAFile", "", "the path of the CAFile")
	flag.StringVar(&argInfo.ServerCertFile, "serverCert", "", "the cert file of the server")
	flag.StringVar(&argInfo.ServerKeyFile, "serverKey", "", "the key file of the server")

	flag.StringVar(&argInfo.ClientCAFile, "clientCAFile", "", "the path of the client CAFile")
	flag.StringVar(&argInfo.ClientCertFile, "clientCert", "", "the cert file of the client")
	flag.StringVar(&argInfo.ClientKeyFile, "clientKey", "", "the key file of the client")

	flag.StringVar(&dirPath, "d", ".", "the directory to store log and snapshot")

	flag.BoolVar(&force, "f", false, "force new node configuration if existing is found (WARNING: data loss!)")

	flag.BoolVar(&snapshot, "snapshot", false, "open or close snapshot")

	flag.IntVar(&maxSize, "m", 1024, "the max size of result buffer")

	flag.IntVar(&retryTimes, "r", 3, "the max retry attempts when trying to join a cluster")

	flag.IntVar(&maxClusterSize, "maxsize", 9, "the max size of the cluster")

	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")
}

// CONSTANTS
const (
	RaftServer = iota
	EtcdServer
)

const (
	ELECTIONTIMEOUT  = 200 * time.Millisecond
	HEARTBEATTIMEOUT = 50 * time.Millisecond

	// Timeout for internal raft http connection
	// The original timeout for http is 45 seconds
	// which is too long for our usage.
	HTTPTIMEOUT   = 10 * time.Second
	RETRYINTERVAL = 10
)

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

	ServerCertFile string `json:"serverCertFile"`
	ServerKeyFile  string `json:"serverKeyFile"`
	ServerCAFile   string `json:"serverCAFile"`

	ClientCertFile string `json:"clientCertFile"`
	ClientKeyFile  string `json:"clientKeyFile"`
	ClientCAFile   string `json:"clientCAFile"`
}

//------------------------------------------------------------------------------
//
// Variables
//
//------------------------------------------------------------------------------

var raftServer *raft.Server
var raftTransporter transporter
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

	raftTlsConfs, ok := tlsConf(RaftServer)
	if !ok {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	raftDefaultScheme := "http"
	if raftTlsConfs[0] != nil {
		raftDefaultScheme = "https"
	}

	etcdTlsConfs, ok := tlsConf(EtcdServer)
	if !ok {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	etcdDefaultScheme := "http"
	if etcdTlsConfs[0] != nil {
		raftDefaultScheme = "https"
	}

	// Otherwise ask user for info and write it to file.
	argInfo.Name = strings.TrimSpace(argInfo.Name)

	if argInfo.Name == "" {
		fatal("ERROR: server name required. e.g. '-n=server_name'")
	}

	argInfo.RaftURL = sanitizeURL(argInfo.RaftURL, raftTlsConfig.Scheme)
	argInfo.EtcdURL = sanitizeURL(argInfo.EtcdURL, etcdTlsConfig.Scheme)
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

	startRaft(raftTlsConfs)

	if argInfo.WebURL != "" {
		// start web
		argInfo.WebURL = checkURL(argInfo.WebURL, "http")
		go webHelper()
		go web.Start(raftServer, argInfo.WebURL)
	}

	startEtcdTransport(*info, etcdTlsConfs[0])

}

// Start the raft server
func startRaft(tlsConfs []*tls.Config) {
	var err error

	raftName := info.Name

	// Create transporter for raft
	raftTransporter = newTransporter(tlsConfs[1])

	// Create raft server
	raftServer, err = raft.NewServer(raftName, dirPath, raftTransporter, etcdStore, nil)

	if err != nil {
		fatal(err)
	}

	// LoadSnapshot
	if snapshot {
		err = raftServer.LoadSnapshot()

		if err == nil {
			debugf("%s finished load snapshot", raftServer.Name())
		} else {
			debug(err)
		}
	}

	raftServer.SetElectionTimeout(ELECTIONTIMEOUT)
	raftServer.SetHeartbeatTimeout(HEARTBEATTIMEOUT)

	raftServer.Start()

	if raftServer.IsLogEmpty() {

		// start as a leader in a new cluster
		if len(cluster) == 0 {

			time.Sleep(time.Millisecond * 20)

			// leader need to join self as a peer
			for {
				command := &JoinCommand{
					Name:    raftServer.Name(),
					RaftURL: argInfo.RaftURL,
					EtcdURL: argInfo.EtcdURL,
				}
				_, err := raftServer.Do(command)
				if err == nil {
					break
				}
			}
			debugf("%s start as a leader", raftServer.Name())

			// start as a follower in a existing cluster
		} else {

			time.Sleep(time.Millisecond * 20)

			for i := 0; i < retryTimes; i++ {

				success := false
				for _, machine := range cluster {
					if len(machine) == 0 {
						continue
					}
					err = joinCluster(raftServer, machine)
					if err != nil {
						if err.Error() == errors[103] {
							fmt.Println(err)
							os.Exit(1)
						}
						debugf("cannot join to cluster via machine %s %s", machine, err)
					} else {
						success = true
						break
					}
				}

				if success {
					break
				}

				warnf("cannot join to cluster via given machines, retry in %d seconds", RETRYINTERVAL)
				time.Sleep(time.Second * RETRYINTERVAL)
			}
			if err != nil {
				fatalf("Cannot join the cluster via given machines after %x retries", retryTimes)
			}
			debugf("%s success join to the cluster", raftServer.Name())
		}

	} else {
		// rejoin the previous cluster
		debugf("%s restart as a follower", raftServer.Name())
	}

	// open the snapshot
	if snapshot {
		go raftServer.Snapshot()
	}

	// start to response to raft requests
	go startRaftTransport(*info, tlsConfs[0])

}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(tlsConf *tls.Config) transporter {
	t := transporter{}

	if tlsConf == nil {
		t.scheme = "http://"

		t.client = &http.Client{
			Transport: &http.Transport{
				Dial: dialTimeout,
			},
		}

	} else {
		t.scheme = "https://"

		tr := &http.Transport{
			TLSClientConfig:    tlsConf,
			Dial:               dialTimeout,
			DisableCompression: true,
		}

		t.client = &http.Client{Transport: tr}
	}

	return t
}

// Dial with timeout
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, HTTPTIMEOUT)
}

// Start to listen and response raft command
func startRaftTransport(info Info, tlsConf *tls.Config) {
	u, _ := url.Parse(info.RaftURL)
	fmt.Printf("raft server [%s] listening on %s\n", info.Name, u)

	raftMux := http.NewServeMux()

	server := &http.Server{
		Handler:   raftMux,
		TLSConfig: tlsConf,
		Addr:      u.Host,
	}

	// internal commands
	raftMux.HandleFunc("/name", NameHttpHandler)
	raftMux.HandleFunc("/join", JoinHttpHandler)
	raftMux.HandleFunc("/vote", VoteHttpHandler)
	raftMux.HandleFunc("/log", GetLogHttpHandler)
	raftMux.HandleFunc("/log/append", AppendEntriesHttpHandler)
	raftMux.HandleFunc("/snapshot", SnapshotHttpHandler)
	raftMux.HandleFunc("/snapshotRecovery", SnapshotRecoveryHttpHandler)
	raftMux.HandleFunc("/etcdURL", EtcdURLHttpHandler)

	if tlsConf == nil {
		fatal(server.ListenAndServe())
	} else {
		fatal(server.ListenAndServeTLS(info.ServerCertFile, argInfo.ServerKeyFile))
	}

}

// Start to listen and response client command
func startEtcdTransport(info Info, tlsConf *tls.Config) {
	u, _ := url.Parse(info.EtcdURL)
	fmt.Printf("etcd server [%s] listening on %s\n", info.Name, u)

	etcdMux := http.NewServeMux()

	server := &http.Server{
		Handler:   etcdMux,
		TLSConfig: tlsConf,
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

	if tlsConf == nil {
		fatal(server.ListenAndServe())
	} else {
		fatal(server.ListenAndServeTLS(info.ClientCertFile, info.ClientKeyFile))
	}
}

//--------------------------------------
// Config
//--------------------------------------
func tlsConf(source int) ([]*tls.Config, bool) {
	var keyFile, certFile, CAFile string
	var tlsCert tls.Certificate
	var isAuth bool
	var err error

	switch source {

	case RaftServer:
		keyFile = info.ServerKeyFile
		certFile = info.ServerCertFile
		CAFile = info.ServerCAFile

		if keyFile != "" && certFile != "" {
			tlsCert, err = tls.LoadX509KeyPair(certFile, keyFile)
			if err == nil {
				fatal(err)
			}
			isAuth = true
		}

	case EtcdServer:
		keyFile = info.ClientKeyFile
		certFile = info.ClientCertFile
		CAFile = info.ClientCAFile
	}

	// If the user do not specify key file, cert file and
	// CA file, the type will be HTTP
	if keyFile == "" && certFile == "" && CAFile == "" {
		return []*tls.Config{nil, nil}, true
	}

	if keyFile != "" && certFile != "" {
		serverConf := &tls.Config{}
		serverConf.ClientAuth, serverConf.ClientCAs = newCertPool(CAFile)

		if isAuth {
			raftTransConf := &tls.Config{
				Certificates:       []tls.Certificate{tlsCert},
				InsecureSkipVerify: true,
			}
			return []*tls.Config{serverConf, raftTransConf}, true
		}

		return []*tls.Config{serverConf, nil}, true

	}

	// bad specification
	return nil, false

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
	infoPath := fmt.Sprintf("%s/info", path)

	// Delete the old configuration if exist
	if force {
		logPath := fmt.Sprintf("%s/log", path)
		confPath := fmt.Sprintf("%s/conf", path)
		snapshotPath := fmt.Sprintf("%s/snapshot", path)
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

// Create client auth certpool
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

// Send join requests to the leader.
func joinCluster(s *raft.Server, serverName string) error {
	var b bytes.Buffer

	command := &JoinCommand{
		Name:    s.Name(),
		RaftURL: info.RaftURL,
		EtcdURL: info.EtcdURL,
	}

	json.NewEncoder(&b).Encode(command)

	// t must be ok
	t, ok := raftServer.Transporter().(transporter)

	if !ok {
		panic("wrong type")
	}

	debugf("Send Join Request to %s", serverName)

	resp, err := t.Post(fmt.Sprintf("%s/join", serverName), &b)

	for {
		if err != nil {
			return fmt.Errorf("Unable to join: %v", err)
		}
		if resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			if resp.StatusCode == http.StatusTemporaryRedirect {

				address := resp.Header.Get("Location")
				debugf("Send Join Request to %s", address)

				json.NewEncoder(&b).Encode(command)

				resp, err = t.Post(address, &b)

			} else if resp.StatusCode == http.StatusBadRequest {
				debug("Reach max number machines in the cluster")
				return fmt.Errorf(errors[103])
			} else {
				return fmt.Errorf("Unable to join")
			}
		}

	}
	return fmt.Errorf("Unable to join: %v", err)
}

// Register commands to raft server
func registerCommands() {
	raft.RegisterCommand(&JoinCommand{})
	raft.RegisterCommand(&SetCommand{})
	raft.RegisterCommand(&GetCommand{})
	raft.RegisterCommand(&DeleteCommand{})
	raft.RegisterCommand(&WatchCommand{})
	raft.RegisterCommand(&TestAndSetCommand{})
}
