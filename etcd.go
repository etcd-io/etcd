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
	"path"
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

	flag.StringVar(&argInfo.Hostname, "h", "0.0.0.0", "the hostname of the local machine")
	flag.IntVar(&argInfo.ClientPort, "c", 4001, "the port to communicate with clients")
	flag.IntVar(&argInfo.RaftPort, "s", 7001, "the port to communicate with servers")
	flag.IntVar(&argInfo.WebPort, "w", -1, "the port of web interface (-1 means do not start web interface)")

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
	HTTP = iota
	HTTPS
	HTTPSANDVERIFY
)

const (
	SERVER = iota
	CLIENT
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
	Hostname   string `json:"hostname"`
	RaftPort   int    `json:"raftPort"`
	ClientPort int    `json:"clientPort"`
	WebPort    int    `json:"webPort"`

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

	// Setup commands.
	registerCommands()

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(dirPath, 0744); err != nil {
		fatalf("Unable to create path: %s", err)
	}

	info = getInfo(dirPath)

	// security type
	st := securityType(SERVER)

	clientSt := securityType(CLIENT)

	if st == -1 || clientSt == -1 {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	// Create etcd key-value store
	etcdStore = store.CreateStore(maxSize)

	startRaft(st)

	if argInfo.WebPort != -1 {
		// start web
		etcdStore.SetMessager(storeMsg)
		go webHelper()
		go web.Start(raftServer, argInfo.WebPort)
	}

	startClientTransport(*info, clientSt)

}

// Start the raft server
func startRaft(securityType int) {
	var err error

	raftName := fmt.Sprintf("%s:%d", info.Hostname, info.RaftPort)

	// Create transporter for raft
	raftTransporter = createTransporter(securityType)

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
					Name:       raftServer.Name(),
					Hostname:   argInfo.Hostname,
					RaftPort:   argInfo.RaftPort,
					ClientPort: argInfo.ClientPort,
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
	go startRaftTransport(*info, securityType)

}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func createTransporter(st int) transporter {
	t := transporter{}

	switch st {
	case HTTP:
		t.scheme = "http://"

		tr := &http.Transport{
			Dial: dialTimeout,
		}

		t.client = &http.Client{
			Transport: tr,
		}

	case HTTPS:
		fallthrough
	case HTTPSANDVERIFY:
		t.scheme = "https://"

		tlsCert, err := tls.LoadX509KeyPair(argInfo.ServerCertFile, argInfo.ServerKeyFile)

		if err != nil {
			fatal(err)
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{tlsCert},
				InsecureSkipVerify: true,
			},
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
func startRaftTransport(info Info, st int) {

	// internal commands
	http.HandleFunc("/join", JoinHttpHandler)
	http.HandleFunc("/vote", VoteHttpHandler)
	http.HandleFunc("/log", GetLogHttpHandler)
	http.HandleFunc("/log/append", AppendEntriesHttpHandler)
	http.HandleFunc("/snapshot", SnapshotHttpHandler)
	http.HandleFunc("/snapshotRecovery", SnapshotRecoveryHttpHandler)
	http.HandleFunc("/client", ClientHttpHandler)

	switch st {

	case HTTP:
		fmt.Printf("raft server [%s] listen on http port %v\n", info.Hostname, info.RaftPort)
		fatal(http.ListenAndServe(fmt.Sprintf(":%d", info.RaftPort), nil))

	case HTTPS:
		fmt.Printf("raft server [%s] listen on https port %v\n", info.Hostname, info.RaftPort)
		fatal(http.ListenAndServeTLS(fmt.Sprintf(":%d", info.RaftPort), info.ServerCertFile, argInfo.ServerKeyFile, nil))

	case HTTPSANDVERIFY:

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  createCertPool(info.ServerCAFile),
			},
			Addr: fmt.Sprintf(":%d", info.RaftPort),
		}
		fmt.Printf("raft server [%s] listen on https port %v\n", info.Hostname, info.RaftPort)
		fatal(server.ListenAndServeTLS(info.ServerCertFile, argInfo.ServerKeyFile))
	}

}

// Start to listen and response client command
func startClientTransport(info Info, st int) {
	// external commands
	http.HandleFunc("/"+version+"/keys/", Multiplexer)
	http.HandleFunc("/"+version+"/watch/", WatchHttpHandler)
	http.HandleFunc("/leader", LeaderHttpHandler)
	http.HandleFunc("/machines", MachinesHttpHandler)
	http.HandleFunc("/", VersionHttpHandler)
	http.HandleFunc("/stats", StatsHttpHandler)
	http.HandleFunc("/test/", TestHttpHandler)

	switch st {

	case HTTP:
		fmt.Printf("etcd [%s] listen on http port %v\n", info.Hostname, info.ClientPort)
		fatal(http.ListenAndServe(fmt.Sprintf(":%d", info.ClientPort), nil))

	case HTTPS:
		fmt.Printf("etcd [%s] listen on https port %v\n", info.Hostname, info.ClientPort)
		http.ListenAndServeTLS(fmt.Sprintf(":%d", info.ClientPort), info.ClientCertFile, info.ClientKeyFile, nil)

	case HTTPSANDVERIFY:

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  createCertPool(info.ClientCAFile),
			},
			Addr: fmt.Sprintf(":%d", info.ClientPort),
		}
		fmt.Printf("etcd [%s] listen on https port %v\n", info.Hostname, info.ClientPort)
		fatal(server.ListenAndServeTLS(info.ClientCertFile, info.ClientKeyFile))
	}
}

//--------------------------------------
// Config
//--------------------------------------

// Get the security type
func securityType(source int) int {

	var keyFile, certFile, CAFile string

	switch source {

	case SERVER:
		keyFile = info.ServerKeyFile
		certFile = info.ServerCertFile
		CAFile = info.ServerCAFile

	case CLIENT:
		keyFile = info.ClientKeyFile
		certFile = info.ClientCertFile
		CAFile = info.ClientCAFile
	}

	// If the user do not specify key file, cert file and
	// CA file, the type will be HTTP
	if keyFile == "" && certFile == "" && CAFile == "" {

		return HTTP

	}

	if keyFile != "" && certFile != "" {
		if CAFile != "" {
			// If the user specify all the three file, the type
			// will be HTTPS with client cert auth
			return HTTPSANDVERIFY
		}
		// If the user specify key file and cert file but not
		// CA file, the type will be HTTPS without client cert
		// auth
		return HTTPS
	}

	// bad specification
	return -1
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

	// Otherwise ask user for info and write it to file.
	argInfo.Hostname = strings.TrimSpace(argInfo.Hostname)

	if argInfo.Hostname == "" {
		fatal("Please give the address of the local machine")
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
func createCertPool(CAFile string) *x509.CertPool {
	pemByte, _ := ioutil.ReadFile(CAFile)

	block, pemByte := pem.Decode(pemByte)

	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		fatal(err)
	}

	certPool := x509.NewCertPool()

	certPool.AddCert(cert)

	return certPool
}

// Send join requests to the leader.
func joinCluster(s *raft.Server, serverName string) error {
	var b bytes.Buffer

	command := &JoinCommand{
		Name:       s.Name(),
		Hostname:   info.Hostname,
		RaftPort:   info.RaftPort,
		ClientPort: info.ClientPort,
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
				u, err := url.Parse(address)

				if err != nil {
					return fmt.Errorf("Unable to join: %s", err.Error())
				}

				json.NewEncoder(&b).Encode(command)

				resp, err = t.Post(path.Join(u.Host, u.Path), &b)

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
