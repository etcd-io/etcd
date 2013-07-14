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
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var verbose bool

var machines string
var machinesFile string

var cluster []string

var address string
var clientPort int
var serverPort int
var webPort int

var serverCertFile string
var serverKeyFile string
var serverCAFile string

var clientCertFile string
var clientKeyFile string
var clientCAFile string

var dirPath string

var ignore bool

var maxSize int

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")

	flag.StringVar(&machines, "C", "", "the ip address and port of a existing machines in the cluster, sepearate by comma")
	flag.StringVar(&machinesFile, "CF", "", "the file contains a list of existing machines in the cluster, seperate by comma")

	flag.StringVar(&address, "a", "0.0.0.0", "the ip address of the local machine")
	flag.IntVar(&clientPort, "c", 4001, "the port to communicate with clients")
	flag.IntVar(&serverPort, "s", 7001, "the port to communicate with servers")
	flag.IntVar(&webPort, "w", -1, "the port of web interface")

	flag.StringVar(&serverCAFile, "serverCAFile", "", "the path of the CAFile")
	flag.StringVar(&serverCertFile, "serverCert", "", "the cert file of the server")
	flag.StringVar(&serverKeyFile, "serverKey", "", "the key file of the server")

	flag.StringVar(&clientCAFile, "clientCAFile", "", "the path of the client CAFile")
	flag.StringVar(&clientCertFile, "clientCert", "", "the cert file of the client")
	flag.StringVar(&clientKeyFile, "clientKey", "", "the key file of the client")

	flag.StringVar(&dirPath, "d", "/tmp/", "the directory to store log and snapshot")

	flag.BoolVar(&ignore, "i", false, "ignore the old configuration, create a new node")

	flag.IntVar(&maxSize, "m", 1024, "the max size of result buffer")
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
	ELECTIONTIMTOUT  = 200 * time.Millisecond
	HEARTBEATTIMEOUT = 50 * time.Millisecond

	// Timeout for internal raft http connection
	// The original timeout for http is 45 seconds
	// which is too long for our usage.
	HTTPTIMEOUT = 10 * time.Second
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

type Info struct {
	Address    string `json:"address"`
	ServerPort int    `json:"serverPort"`
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

	if machines != "" {
		cluster = strings.Split(machines, ",")
	} else if machinesFile != "" {
		b, err := ioutil.ReadFile(machinesFile)
		if err != nil {
			fatal("Unable to read the given machines file: %s", err)
		}
		cluster = strings.Split(string(b), ",")
	}

	// Setup commands.
	registerCommands()

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(dirPath, 0744); err != nil {
		fatal("Unable to create path: %s", err)
	}

	info = getInfo(dirPath)

	// secrity type
	st := securityType(SERVER)

	clientSt := securityType(CLIENT)

	if st == -1 || clientSt == -1 {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	// Create etcd key-value store
	etcdStore = store.CreateStore(maxSize)

	startRaft(st)

	if webPort != -1 {
		// start web
		etcdStore.SetMessager(&storeMsg)
		go webHelper()
		go web.Start(raftServer, webPort)
	}

	startClientTransport(info.ClientPort, clientSt)

}

// Start the raft server
func startRaft(securityType int) {
	var err error

	raftName := fmt.Sprintf("%s:%d", info.Address, info.ServerPort)

	// Create transporter for raft
	raftTransporter = createTransporter(securityType)

	// Create raft server
	raftServer, err = raft.NewServer(raftName, dirPath, raftTransporter, etcdStore, nil)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// LoadSnapshot
	// err = raftServer.LoadSnapshot()

	// if err == nil {
	// 	debug("%s finished load snapshot", raftServer.Name())
	// } else {
	// 	debug(err)
	// }

	raftServer.Initialize()
	raftServer.SetElectionTimeout(ELECTIONTIMTOUT)
	raftServer.SetHeartbeatTimeout(HEARTBEATTIMEOUT)

	if raftServer.IsLogEmpty() {

		// start as a leader in a new cluster
		if len(cluster) == 1 && cluster[0] == "" {
			raftServer.StartLeader()

			time.Sleep(time.Millisecond * 20)

			// leader need to join self as a peer
			for {
				command := &JoinCommand{}
				command.Name = raftServer.Name()
				_, err := raftServer.Do(command)
				if err == nil {
					break
				}
			}
			debug("%s start as a leader", raftServer.Name())

			// start as a follower in a existing cluster
		} else {
			raftServer.StartFollower()

			for _, machine := range cluster {

				err = joinCluster(raftServer, machine)
				if err != nil {
					debug("cannot join to cluster via machine %s", machine)
				} else {
					break
				}
			}
			if err != nil {
				fatal("cannot join to cluster via all given machines!")
			}
			debug("%s success join to the cluster", raftServer.Name())
		}

	} else {
		// rejoin the previous cluster
		raftServer.StartFollower()
		debug("%s restart as a follower", raftServer.Name())
	}

	// open the snapshot
	// go server.Snapshot()

	// start to response to raft requests
	go startRaftTransport(info.ServerPort, securityType)

}

// Create transporter using by raft server
// Create http or https transporter based on
// wether the user give the server cert and key
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
		return t

	case HTTPS:
		fallthrough
	case HTTPSANDVERIFY:
		t.scheme = "https://"

		tlsCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)

		if err != nil {
			fatal(fmt.Sprintln(err))
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
		return t
	}

	// for complier
	return transporter{}
}

// Dial with timeout
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, HTTPTIMEOUT)
}

// Start to listen and response raft command
func startRaftTransport(port int, st int) {

	// internal commands
	http.HandleFunc("/join", JoinHttpHandler)
	http.HandleFunc("/vote", VoteHttpHandler)
	http.HandleFunc("/log", GetLogHttpHandler)
	http.HandleFunc("/log/append", AppendEntriesHttpHandler)
	http.HandleFunc("/snapshot", SnapshotHttpHandler)
	http.HandleFunc("/client", ClientHttpHandler)

	switch st {

	case HTTP:
		fmt.Printf("raft server [%s] listen on http port %v\n", address, port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		fmt.Printf("raft server [%s] listen on https port %v\n", address, port)
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf(":%d", port), serverCertFile, serverKeyFile, nil))

	case HTTPSANDVERIFY:

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  createCertPool(serverCAFile),
			},
			Addr: fmt.Sprintf(":%d", port),
		}
		fmt.Printf("raft server [%s] listen on https port %v\n", address, port)
		err := server.ListenAndServeTLS(serverCertFile, serverKeyFile)

		if err != nil {
			log.Fatal(err)
		}
	}

}

// Start to listen and response client command
func startClientTransport(port int, st int) {
	// external commands
	http.HandleFunc("/"+version+"/keys/", Multiplexer)
	http.HandleFunc("/"+version+"/watch/", WatchHttpHandler)
	http.HandleFunc("/"+version+"/testAndSet/", TestAndSetHttpHandler)
	http.HandleFunc("/leader", LeaderHttpHandler)
	http.HandleFunc("/machines", MachinesHttpHandler)

	switch st {

	case HTTP:
		fmt.Printf("etcd [%s] listen on http port %v\n", address, clientPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		fmt.Printf("etcd [%s] listen on https port %v\n", address, clientPort)
		http.ListenAndServeTLS(fmt.Sprintf(":%d", port), clientCertFile, clientKeyFile, nil)

	case HTTPSANDVERIFY:

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  createCertPool(clientCAFile),
			},
			Addr: fmt.Sprintf(":%d", port),
		}
		fmt.Printf("etcd [%s] listen on https port %v\n", address, clientPort)
		err := server.ListenAndServeTLS(clientCertFile, clientKeyFile)

		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
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

// Get the server info from previous conf file
// or from the user
func getInfo(path string) *Info {
	info := &Info{}

	// Read in the server info if available.
	infoPath := fmt.Sprintf("%s/info", path)

	// Delete the old configuration if exist
	if ignore {

		logPath := fmt.Sprintf("%s/log", path)
		snapshotPath := fmt.Sprintf("%s/snapshotPath", path)
		os.Remove(infoPath)
		os.Remove(logPath)
		os.RemoveAll(snapshotPath)

	}

	if file, err := os.Open(infoPath); err == nil {
		if content, err := ioutil.ReadAll(file); err != nil {
			fatal("Unable to read info: %v", err)
		} else {
			if err = json.Unmarshal(content, &info); err != nil {
				fatal("Unable to parse info: %v", err)
			}
		}
		file.Close()

	} else {
		// Otherwise ask user for info and write it to file.

		if address == "" {
			fatal("Please give the address of the local machine")
		}

		info.Address = address
		info.Address = strings.TrimSpace(info.Address)
		fmt.Println("address ", info.Address)

		info.ServerPort = serverPort
		info.ClientPort = clientPort
		info.WebPort = webPort

		info.ClientCAFile = clientCAFile
		info.ClientCertFile = clientCertFile
		info.ClientKeyFile = clientKeyFile

		info.ServerCAFile = serverCAFile
		info.ServerKeyFile = serverKeyFile
		info.ServerCertFile = serverCertFile

		// Write to file.
		content, _ := json.Marshal(info)
		content = []byte(string(content) + "\n")
		if err := ioutil.WriteFile(infoPath, content, 0644); err != nil {
			fatal("Unable to write info to file: %v", err)
		}
	}

	return info
}

// Create client auth certpool
func createCertPool(CAFile string) *x509.CertPool {
	pemByte, _ := ioutil.ReadFile(CAFile)

	block, pemByte := pem.Decode(pemByte)

	cert, err := x509.ParseCertificate(block.Bytes)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	certPool := x509.NewCertPool()

	certPool.AddCert(cert)

	return certPool
}

// Send join requests to the leader.
func joinCluster(s *raft.Server, serverName string) error {
	var b bytes.Buffer

	command := &JoinCommand{}
	command.Name = s.Name()

	json.NewEncoder(&b).Encode(command)

	// t must be ok
	t, ok := raftServer.Transporter().(transporter)

	if !ok {
		panic("wrong type")
	}

	debug("Send Join Request to %s", serverName)

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
				address = resp.Header.Get("Location")
				debug("Leader is %s", address)
				debug("Send Join Request to %s", address)
				json.NewEncoder(&b).Encode(command)
				resp, err = t.Post(fmt.Sprintf("%s/join", address), &b)
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
