package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/benbjohnson/go-raft"
	"log"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"os"
	"time"
	"strconv"
	"github.com/xiangli-cmu/raft-etcd/web"
	"github.com/xiangli-cmu/raft-etcd/store"
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
var cert string
var key string
var CAFile string

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.StringVar(&leaderHost, "c", "", "join to a existing cluster")
	flag.StringVar(&address, "a", "", "the address of the local machine")
	flag.IntVar(&webPort, "w", -1, "the port of web interface")
}

const (
	ELECTIONTIMTOUT = 3 * time.Second
	HEARTBEATTIMEOUT = 1 * time.Second
)



//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

type Info struct {
	Host string `json:"host"`
	Port int `json:"port"`
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
	
	t := transHandler{}

	// Setup new raft server.
	s := store.GetStore()

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

			Join(server, leaderHost)
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


    if webPort != -1 {
    	// start web
    	s.SetMessager(&storeMsg)
    	go webHelper()
    	go web.Start(server, webPort)
    } 

    // listen on http port
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", info.Port), nil))
}

func usage() {
	fatal("usage: raftd [PATH]")
}

//--------------------------------------
// Config
//--------------------------------------

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
	debug("[send] POST http://%v/join", "localhost:4001")
	resp, err := http.Post(fmt.Sprintf("http://%s/join", serverName), "application/json", &b)
	if resp != nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}
	return fmt.Errorf("raftd: Unable to join: %v", err)
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

//--------------------------------------
// Log
//--------------------------------------

func debug(msg string, v ...interface{}) {
	if verbose {
		logger.Printf("DEBUG " + msg + "\n", v...)
	}
}

func info(msg string, v ...interface{}) {
	logger.Printf("INFO  " + msg + "\n", v...)
}

func warn(msg string, v ...interface{}) {
	logger.Printf("WARN  " + msg + "\n", v...)
}

func fatal(msg string, v ...interface{}) {
	logger.Printf("FATAL " + msg + "\n", v...)
	os.Exit(1)
}



