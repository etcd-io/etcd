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

var cluster string

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

	flag.StringVar(&cluster, "C", "", "the ip address and port of a existing cluster")

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
}

//------------------------------------------------------------------------------
//
// Variables
//
//------------------------------------------------------------------------------

var raftServer *raft.Server
var raftTransporter transporter
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
	var err error
	flag.Parse()

	// Setup commands.
	registerCommands()

	// Read server info from file or grab it from user.
	if err := os.MkdirAll(dirPath, 0744); err != nil {
		fatal("Unable to create path: %v", err)
	}
	var info *Info = getInfo(dirPath)

	name := fmt.Sprintf("%s:%d", info.Address, info.ServerPort)

	// secrity type
	st := securityType(SERVER)

	clientSt := securityType(CLIENT)

	if st == -1 || clientSt == -1 {
		fatal("Please specify cert and key file or cert and key file and CAFile or none of the three")
	}

	// Create transporter for raft
	raftTransporter = createTransporter(st)

	// Create etcd key-value store
	etcdStore = store.CreateStore(maxSize)

	// Create raft server
	raftServer, err = raft.NewServer(name, dirPath, raftTransporter, etcdStore, nil)

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
		if cluster == "" {
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

			err := joinCluster(raftServer, cluster)
			if err != nil {
				fatal(fmt.Sprintln(err))
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

	if webPort != -1 {
		// start web
		etcdStore.SetMessager(&storeMsg)
		go webHelper()
		go web.Start(raftServer, webPort)
	}

	go startRaftTransport(info.ServerPort, st)

	startClientTransport(info.ClientPort, clientSt)

}

// Create transporter using by raft server
// Create http or https transporter based on
// wether the user give the server cert and key
func createTransporter(st int) transporter {
	t := transporter{}

	switch st {
	case HTTP:
		t.client = nil
		return t

	case HTTPS:
		fallthrough
	case HTTPSANDVERIFY:
		tlsCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)

		if err != nil {
			fatal(fmt.Sprintln(err))
		}

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{tlsCert},
				InsecureSkipVerify: true,
			},
			DisableCompression: true,
		}

		t.client = &http.Client{Transport: tr}
		return t
	}

	// for complier
	return transporter{}
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
		fmt.Println("raft server [%s] listen on http port %v", address, port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		fmt.Println("raft server [%s] listen on https port %v", address, port)
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf(":%d", port), serverCertFile, serverKeyFile, nil))

	case HTTPSANDVERIFY:

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  createCertPool(serverCAFile),
			},
			Addr: fmt.Sprintf(":%d", port),
		}
		fmt.Println("raft server [%s] listen on https port %v", address, port)
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
	http.HandleFunc("/"+version+"/list/", ListHttpHandler)
	http.HandleFunc("/"+version+"/testAndSet/", TestAndSetHttpHandler)
	http.HandleFunc("/leader", LeaderHttpHandler)

	switch st {

	case HTTP:
		fmt.Println("etcd [%s] listen on http port %v", address, clientPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		fmt.Println("etcd [%s] listen on https port %v", address, clientPort)
		http.ListenAndServeTLS(fmt.Sprintf(":%d", port), clientCertFile, clientKeyFile, nil)

	case HTTPSANDVERIFY:

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  createCertPool(clientCAFile),
			},
			Addr: fmt.Sprintf(":%d", port),
		}
		fmt.Println("etcd [%s] listen on https port %v", address, clientPort)
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

func securityType(source int) int {

	var keyFile, certFile, CAFile string

	switch source {

	case SERVER:
		keyFile = serverKeyFile
		certFile = serverCertFile
		CAFile = serverCAFile

	case CLIENT:
		keyFile = clientKeyFile
		certFile = clientCertFile
		CAFile = clientCAFile
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

func getInfo(path string) *Info {
	info := &Info{}

	// Read in the server info if available.
	infoPath := fmt.Sprintf("%s/info", path)

	// delete the old configuration if exist
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

		// Otherwise ask user for info and write it to file.
	} else {

		if address == "" {
			fatal("Please give the address of the local machine")
		}

		info.Address = address
		info.Address = strings.TrimSpace(info.Address)
		fmt.Println("address ", info.Address)

		info.ServerPort = serverPort
		info.ClientPort = clientPort
		info.WebPort = webPort

		// Write to file.
		content, _ := json.Marshal(info)
		content = []byte(string(content) + "\n")
		if err := ioutil.WriteFile(infoPath, content, 0644); err != nil {
			fatal("Unable to write info to file: %v", err)
		}
	}

	return info
}

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
	t, _ := raftServer.Transporter().(transporter)
	debug("Send Join Request to %s", serverName)
	resp, err := t.Post(fmt.Sprintf("%s/join", serverName), &b)

	for {
		if resp != nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			if resp.StatusCode == http.StatusServiceUnavailable {
				address, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					warn("Cannot Read Leader info: %v", err)
				}
				debug("Leader is %s", address)
				debug("Send Join Request to %s", address)
				json.NewEncoder(&b).Encode(command)
				resp, err = t.Post(fmt.Sprintf("%s/join", address), &b)
			}
		}
	}
	return fmt.Errorf("Unable to join: %v", err)
}

// register commands to raft server
func registerCommands() {
	raft.RegisterCommand(&JoinCommand{})
	raft.RegisterCommand(&SetCommand{})
	raft.RegisterCommand(&GetCommand{})
	raft.RegisterCommand(&DeleteCommand{})
	raft.RegisterCommand(&WatchCommand{})
	raft.RegisterCommand(&ListCommand{})
	raft.RegisterCommand(&TestAndSetCommand{})
}
