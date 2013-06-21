package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"github.com/benbjohnson/go-raft"
	"github.com/xiangli-cmu/raft-etcd/store"
	"github.com/xiangli-cmu/raft-etcd/web"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var verbose bool
var leaderHost string
var address string
var webPort int
var certFile string
var keyFile string
var CAFile string

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.StringVar(&leaderHost, "c", "", "join to a existing cluster")
	flag.StringVar(&address, "a", "", "the address of the local machine")
	flag.IntVar(&webPort, "w", -1, "the port of web interface")
	flag.StringVar(&CAFile, "CAFile", "", "the path of the CAFile")
	flag.StringVar(&certFile, "cert", "", "the cert file of the server")
	flag.StringVar(&keyFile, "key", "", "the key file of the server")
}

// CONSTANTS
const (
	HTTP = iota
	HTTPS
	HTTPSANDVERIFY
)

const (
	ELECTIONTIMTOUT  = 3 * time.Second
	HEARTBEATTIMEOUT = 1 * time.Second
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

type Info struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

//------------------------------------------------------------------------------
//
// Variables
//
//------------------------------------------------------------------------------

var server *raft.Server
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

	// Use the present working directory if a directory was not passed in.
	var path string
	if flag.NArg() == 0 {
		path, _ = os.Getwd()
	} else {
		path = flag.Arg(0)
		if err := os.MkdirAll(path, 0744); err != nil {
			fatal("Unable to create path: %v", err)
		}
	}

	// Read server info from file or grab it from user.
	var info *Info = getInfo(path)

	name := fmt.Sprintf("%s:%d", info.Host, info.Port)

	fmt.Printf("Name: %s\n\n", name)

	// secrity type
	st := securityType()

	if st == -1 {
		panic("ERROR type")
	}

	t := createTranHandler(st)

	// Setup new raft server.
	s := store.GetStore()

	// create raft server
	server, err = raft.NewServer(name, path, t, s, nil)

	if err != nil {
		fatal("%v", err)
	}

	server.LoadSnapshot()
	debug("%s finished load snapshot", server.Name())
	server.Initialize()
	debug("%s finished init", server.Name())
	server.SetElectionTimeout(ELECTIONTIMTOUT)
	server.SetHeartbeatTimeout(HEARTBEATTIMEOUT)
	debug("%s finished set timeout", server.Name())

	if server.IsLogEmpty() {

		// start as a leader in a new cluster
		if leaderHost == "" {
			server.StartHeartbeatTimeout()
			server.StartLeader()

			// join self as a peer
			command := &JoinCommand{}
			command.Name = server.Name()
			server.Do(command)
			debug("%s start as a leader", server.Name())

			// start as a fellower in a existing cluster
		} else {
			server.StartElectionTimeout()
			server.StartFollower()

			err := Join(server, leaderHost)
			if err != nil {
				panic(err)
			}
			fmt.Println("success join")
		}

		// rejoin the previous cluster
	} else {
		server.StartElectionTimeout()
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

	startTransport(info.Port, st)

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
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)

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

func startTransport(port int, st int) {

	// internal commands
	http.HandleFunc("/join", JoinHttpHandler)
	http.HandleFunc("/vote", VoteHttpHandler)
	http.HandleFunc("/log", GetLogHttpHandler)
	http.HandleFunc("/log/append", AppendEntriesHttpHandler)
	http.HandleFunc("/snapshot", SnapshotHttpHandler)

	// external commands
	http.HandleFunc("/set/", SetHttpHandler)
	http.HandleFunc("/get/", GetHttpHandler)
	http.HandleFunc("/delete/", DeleteHttpHandler)
	http.HandleFunc("/watch/", WatchHttpHandler)
	http.HandleFunc("/master", MasterHttpHandler)

	switch st {

	case HTTP:
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))

	case HTTPS:
		http.ListenAndServeTLS(fmt.Sprintf(":%d", port), certFile, keyFile, nil)

	case HTTPSANDVERIFY:
		pemByte, _ := ioutil.ReadFile(CAFile)

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
		err = server.ListenAndServeTLS(certFile, keyFile)

		if err != nil {
			log.Fatal(err)
		}
	}

}

//--------------------------------------
// Config
//--------------------------------------

func securityType() int {
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

		input := strings.Split(address, ":")

		if len(input) != 2 {
			fatal("Wrong address %s", address)
		}

		info.Host = input[0]
		info.Host = strings.TrimSpace(info.Host)

		info.Port, err = strconv.Atoi(input[1])

		if err != nil {
			fatal("Wrong port %s", address)
		}

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

//--------------------------------------
// Web Helper
//--------------------------------------

func webHelper() {
	storeMsg = make(chan string)
	for {
		web.Hub().Send(<-storeMsg)
	}
}

//--------------------------------------
// HTTP Utilities
//--------------------------------------

func decodeJsonRequest(req *http.Request, data interface{}) error {
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&data); err != nil && err != io.EOF {
		logger.Println("Malformed json request: %v", err)
		return fmt.Errorf("Malformed json request: %v", err)
	}
	return nil
}

func encodeJsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		encoder := json.NewEncoder(w)
		encoder.Encode(data)
	}
}

func Post(t *transHandler, path string, body io.Reader) (*http.Response, error) {

	if t.client != nil {
		resp, err := t.client.Post("https://"+path, "application/json", body)
		return resp, err
	} else {
		resp, err := http.Post("http://"+path, "application/json", body)
		return resp, err
	}
}

func Get(t *transHandler, path string) (*http.Response, error) {
	if t.client != nil {
		resp, err := t.client.Get("https://" + path)
		return resp, err
	} else {
		resp, err := http.Get("http://" + path)
		return resp, err
	}
}

//--------------------------------------
// Log
//--------------------------------------

func debug(msg string, v ...interface{}) {
	if verbose {
		logger.Printf("DEBUG "+msg+"\n", v...)
	}
}

func info(msg string, v ...interface{}) {
	logger.Printf("INFO  "+msg+"\n", v...)
}

func warn(msg string, v ...interface{}) {
	logger.Printf("Alpaca Server: WARN  "+msg+"\n", v...)
}

func fatal(msg string, v ...interface{}) {
	logger.Printf("FATAL "+msg+"\n", v...)
	os.Exit(1)
}
