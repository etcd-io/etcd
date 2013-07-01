package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/xiangli-cmu/go-raft"
	"github.com/xiangli-cmu/raft-etcd/store"
	"github.com/xiangli-cmu/raft-etcd/web"
	//"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	//"strconv"
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

	flag.StringVar(&address, "a", "", "the ip address of the local machine")
	flag.IntVar(&clientPort, "c", 4001, "the port to communicate with clients")
	flag.IntVar(&serverPort, "s", 7001, "the port to communicate with servers")
	flag.IntVar(&webPort, "w", -1, "the port of web interface")

	flag.StringVar(&serverCAFile, "serverCAFile", "", "the path of the CAFile")
	flag.StringVar(&serverCertFile, "serverCert", "", "the cert file of the server")
	flag.StringVar(&serverKeyFile, "serverKey", "", "the key file of the server")

	flag.StringVar(&clientCAFile, "clientCAFile", "", "the path of the client CAFile")
	flag.StringVar(&clientCertFile, "clientCert", "", "the cert file of the client")
	flag.StringVar(&clientKeyFile, "clientKey", "", "the key file of the client")

	flag.StringVar(&dirPath, "d", "./", "the directory to store log and snapshot")

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

var server *raft.Server
var serverTransHandler transHandler
var logger *log.Logger

var storeMsg chan string

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
	logger = log.New(os.Stdout, "", log.LstdFlags)
	flag.Parse()

	// Setup commands.
	raft.RegisterCommand(&JoinCommand{})
	raft.RegisterCommand(&SetCommand{})
	raft.RegisterCommand(&GetCommand{})
	raft.RegisterCommand(&DeleteCommand{})

	if err := os.MkdirAll(dirPath, 0744); err != nil {
		fatal("Unable to create path: %v", err)
	}

	// Read server info from file or grab it from user.
	var info *Info = getInfo(dirPath)

	name := fmt.Sprintf("%s:%d", info.Address, info.ServerPort)

	fmt.Printf("ServerName: %s\n\n", name)

	// secrity type
	st := securityType(SERVER)

	if st == -1 {
		panic("ERROR type")
	}

	serverTransHandler = createTranHandler(st)

	// Setup new raft server.
	s := store.CreateStore(maxSize)

	// create raft server
	server, err = raft.NewServer(name, dirPath, serverTransHandler, s, nil)

	if err != nil {
		fatal("%v", err)
	}

	err = server.LoadSnapshot()

	if err == nil {
		debug("%s finished load snapshot", server.Name())
	} else {
		fmt.Println(err)
		debug("%s bad snapshot", server.Name())
	}
	server.Initialize()
	debug("%s finished init", server.Name())
	server.SetElectionTimeout(ELECTIONTIMTOUT)
	server.SetHeartbeatTimeout(HEARTBEATTIMEOUT)
	debug("%s finished set timeout", server.Name())

	if server.IsLogEmpty() {

		// start as a leader in a new cluster
		if cluster == "" {
			server.StartLeader()

			// join self as a peer
			command := &JoinCommand{}
			command.Name = server.Name()
			server.Do(command)
			debug("%s start as a leader", server.Name())

			// start as a fellower in a existing cluster
		} else {
			server.StartFollower()

			err := Join(server, cluster)
			if err != nil {
				panic(err)
			}
			fmt.Println("success join")
		}

		// rejoin the previous cluster
	} else {
		server.StartFollower()
		debug("%s start as a follower", server.Name())
	}

	// open the snapshot
	go server.Snapshot()

	if webPort != -1 {
		// start web
		s.SetMessager(&storeMsg)
		go webHelper()
		go web.Start(server, webPort)
	}

	go startServTransport(info.ServerPort, st)
	startClientTransport(info.ClientPort, securityType(CLIENT))

}

func usage() {
	fatal("usage: raftd [PATH]")
}

func createTranHandler(st int) transHandler {
	t := transHandler{}

	switch st {
	case HTTP:
		t := transHandler{}
		t.client = nil
		return t

	case HTTPS:
		fallthrough
	case HTTPSANDVERIFY:
		tlsCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)

		if err != nil {
			panic(err)
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
	return transHandler{}
}

func startServTransport(port int, st int) {

	// internal commands
	http.HandleFunc("/join", JoinHttpHandler)
	http.HandleFunc("/vote", VoteHttpHandler)
	http.HandleFunc("/log", GetLogHttpHandler)
	http.HandleFunc("/log/append", AppendEntriesHttpHandler)
	http.HandleFunc("/snapshot", SnapshotHttpHandler)
	http.HandleFunc("/client", clientHttpHandler)

	switch st {

	case HTTP:
		debug("raft server [%s] listen on http", server.Name())
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		http.ListenAndServeTLS(fmt.Sprintf(":%d", port), serverCertFile, serverKeyFile, nil)

	case HTTPSANDVERIFY:
		pemByte, _ := ioutil.ReadFile(serverCAFile)

		block, pemByte := pem.Decode(pemByte)

		cert, err := x509.ParseCertificate(block.Bytes)

		if err != nil {
			fmt.Println(err)
		}

		certPool := x509.NewCertPool()

		certPool.AddCert(cert)

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  certPool,
			},
			Addr: fmt.Sprintf(":%d", port),
		}
		err = server.ListenAndServeTLS(serverCertFile, serverKeyFile)

		if err != nil {
			log.Fatal(err)
		}
	}

}

func startClientTransport(port int, st int) {
	// external commands
	http.HandleFunc("/v1/keys/", Multiplexer)
	http.HandleFunc("/v1/watch/", WatchHttpHandler)
	http.HandleFunc("/master", MasterHttpHandler)

	switch st {

	case HTTP:
		debug("etcd [%s] listen on http", server.Name())
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		http.ListenAndServeTLS(fmt.Sprintf(":%d", port), clientCertFile, clientKeyFile, nil)

	case HTTPSANDVERIFY:
		pemByte, _ := ioutil.ReadFile(clientCAFile)

		block, pemByte := pem.Decode(pemByte)

		cert, err := x509.ParseCertificate(block.Bytes)

		if err != nil {
			fmt.Println(err)
		}

		certPool := x509.NewCertPool()

		certPool.AddCert(cert)

		server := &http.Server{
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  certPool,
			},
			Addr: fmt.Sprintf(":%d", port),
		}
		err = server.ListenAndServeTLS(clientCertFile, clientKeyFile)

		if err != nil {
			log.Fatal(err)
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

	if keyFile == "" && certFile == "" && CAFile == "" {

		return HTTP

	}

	if keyFile != "" && certFile != "" {

		if CAFile != "" {
			return HTTPSANDVERIFY
		}

		return HTTPS
	}

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

//--------------------------------------
// Handlers
//--------------------------------------

// Send join requests to the leader.
func Join(s *raft.Server, serverName string) error {
	var b bytes.Buffer

	command := &JoinCommand{}
	command.Name = s.Name()

	json.NewEncoder(&b).Encode(command)

	// t must be ok
	t, _ := server.Transporter().(transHandler)
	debug("Send Join Request to %s", serverName)
	resp, err := Post(&t, fmt.Sprintf("%s/join", serverName), &b)

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
				resp, err = Post(&t, fmt.Sprintf("%s/join", address), &b)
			}
		}
	}
	return fmt.Errorf("Unable to join: %v", err)
}
