package main

import (
	"github.com/benbjohnson/go-raft"
	"net/http"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"bytes"
	)

//--------------------------------------
// HTTP Handlers
//--------------------------------------



func GetLogHttpHandler(w http.ResponseWriter, req *http.Request) {
	debug("[recv] GET http://%v/log", server.Name())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(server.LogEntries())
}

func VoteHttpHandler(w http.ResponseWriter, req *http.Request) {
	rvreq := &raft.RequestVoteRequest{}
	err := decodeJsonRequest(req, rvreq)
	if err == nil {
		debug("[recv] POST http://%v/vote [%s]", server.Name(), rvreq.CandidateName)
		if resp, _ := server.RequestVote(rvreq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	w.WriteHeader(http.StatusInternalServerError)
}

func AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		debug("[recv] POST http://%s/log/append [%d]", server.Name(), len(aereq.Entries))
		debug("My role is %s", server.State())
		if resp, _ := server.AppendEntries(aereq); resp != nil {
			debug("write back success")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			if !resp.Success {
				fmt.Println("append error")
			}
			return
		}
	}
	warn("[append] ERROR: %v", err)
	debug("write back")
	w.WriteHeader(http.StatusInternalServerError)
}

func SnapshotHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		debug("[recv] POST http://%s/snapshot/ ", server.Name())
		if resp, _ := server.SnapshotRecovery(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	warn("[snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}


func JoinHttpHandler(w http.ResponseWriter, req *http.Request) {
	debug("[recv] POST http://%v/join", server.Name())
	command := &JoinCommand{}
	if err := decodeJsonRequest(req, command); err == nil {
		if _, err= server.Do(command); err != nil {
			warn("raftd: Unable to join: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	} else {
		warn("[join] ERROR: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}


func SetHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/set/"):]

	debug("[recv] POST http://%v/set/%s", server.Name(), key)

	content, err := ioutil.ReadAll(req.Body)
	if err != nil {
		warn("raftd: Unable to read: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return 
	}

	command := &SetCommand{}
	command.Key = key
	command.Value = string(content)

	Dispatch(server, command, w)

}

func GetHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/get/"):]

	debug("[recv] GET http://%v/get/%s", server.Name(), key)

	command := &GetCommand{}
	command.Key = key

	Dispatch(server, command, w)

}

func DeleteHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/delete/"):]

	debug("[recv] GET http://%v/delete/%s", server.Name(), key)

	command := &DeleteCommand{}
	command.Key = key

	Dispatch(server, command, w)

}


func WatchHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/watch/"):]

	debug("[recv] GET http://%v/watch/%s", server.Name(), key)

	command := &WatchCommand{}
	command.Key = key

	Dispatch(server, command, w)

}

func Dispatch(server *raft.Server, command Command, w http.ResponseWriter) {
	var body []byte
	var err error


	fmt.Println("dispatch")
	// unlikely to fail twice
	for {
		// i am the leader, i will take care of the command
		if server.State() == "leader" {
			if command.Sensitive() {
				if body, err = server.Do(command); err != nil {
					warn("raftd: Unable to write file: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
				// good to go
					w.WriteHeader(http.StatusOK)
					w.Write(body)
					return
				}
			} else {
				fmt.Println("non-sensitive")
				if body, err = command.Apply(server); err != nil {
					warn("raftd: Unable to write file: %v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				} else {
				// good to go
					w.WriteHeader(http.StatusOK)
					w.Write(body)
					return
				}
			}

		// redirect the command to the current leader
		} else {
			leaderName := server.Leader()

			if leaderName =="" {
				// no luckey, during the voting process
				w.WriteHeader(http.StatusInternalServerError)
				return
			} 

			fmt.Println("forward to ", leaderName)

			path := command.GeneratePath()

			if command.Type() == "POST" {
				debug("[send] POST http://%v/%s", leaderName, path)

				reader := bytes.NewReader([]byte(command.GetValue()))

				reps, _ := http.Post(fmt.Sprintf("http://%v/%s", 
					leaderName, command.GeneratePath()), "application/json", reader)

				body, _ := ioutil.ReadAll(reps.Body)
				fmt.Println(body)
				// good to go
				w.WriteHeader(http.StatusOK)

				w.Write(body)

			} else if command.Type() == "GET" {
				debug("[send] GET http://%v/%s", leaderName, path)

				reps, _ := http.Get(fmt.Sprintf("http://%v/%s", 
					leaderName, command.GeneratePath()))
				// good to go
				body, _ := ioutil.ReadAll(reps.Body)
				fmt.Println(body)

				w.WriteHeader(http.StatusOK)
				
				w.Write(body)

			} else {
				//unsupported type
			}

			if err != nil {
				// should check other errors
				continue
			} else {
				//good to go
				return
			}

		}
	}
}
