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
)

//------------------------------------------------------------------------------
//
// Initialization
//
//------------------------------------------------------------------------------

var verbose bool

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose logging")
	flag.BoolVar(&verbose, "verbose", false, "verbose logging")
}

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
	if verbose {
		fmt.Println("Verbose logging enabled.\n")
	}

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
	server, err = raft.NewServer(name, path, t, nil)
	//server.DoHandler = DoHandler;
	if err != nil {
		fatal("%v", err)
	}
	server.Initialize()
	fmt.Println("1 join as ", server.State(), " term ",  server.Term())
	// Join to another server if we don't have a log.
	if server.IsLogEmpty() {
		var leaderHost string
		fmt.Println("2 join as ", server.State(), " term ",  server.Term())
		fmt.Println("This server has no log. Please enter a server in the cluster to join\nto or hit enter to initialize a cluster.");
		fmt.Printf("Join to (host:port)> ");
		fmt.Scanf("%s", &leaderHost)
		fmt.Println("3 join as ", server.State(), " term ",  server.Term())
		if leaderHost == "" {
			fmt.Println("init")
			server.SetElectionTimeout(10 * time.Second)
			server.SetHeartbeatTimeout(1 * time.Second)
			server.StartLeader()

			// join self 
			command := &JoinCommand{}
			command.Name = server.Name()

			server.Do(command)
		} else {
			server.SetElectionTimeout(10 * time.Second)
			server.SetHeartbeatTimeout(1 * time.Second)
			server.StartFollower()
			fmt.Println("4 join as ", server.State(), " term ",  server.Term())
			Join(server, leaderHost)
			fmt.Println("success join")
		}
	} else {
		server.SetElectionTimeout(10 * time.Second)
		server.SetHeartbeatTimeout(1 * time.Second)
		server.StartFollower()
	}

	// open snapshot
	//go server.Snapshot()

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
		fmt.Printf("Enter hostname: [localhost] ");
		fmt.Scanf("%s", &info.Host)
		info.Host = strings.TrimSpace(info.Host)
		if info.Host == "" {
			info.Host = "localhost"
		}

		fmt.Printf("Enter port: [4001] ");
		fmt.Scanf("%d", &info.Port)
		if info.Port == 0 {
			info.Port = 4001
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



