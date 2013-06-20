package main

import (
	"github.com/benbjohnson/go-raft"
	"net/http"
	"encoding/json"
	//"fmt"
	"io/ioutil"
	//"bytes"
	"time"
	"strings"
	"strconv"
	)


//--------------------------------------
// HTTP Handlers
//--------------------------------------

// Get all the current logs
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
	warn("[vote] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

func AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		debug("[recv] POST http://%s/log/append [%d]", server.Name(), len(aereq.Entries))
		if resp, _ := server.AppendEntries(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			if !resp.Success {
				debug("[Append Entry] Step back")
			}
			return
		}
	}
	warn("[append] ERROR: %v", err)
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
	
	command := &JoinCommand{}

	if err := decodeJsonRequest(req, command); err == nil {
		debug("Receive Join Request from %s", command.Name)
		excute(command, &w)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}


func SetHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/set/"):]

	content, err := ioutil.ReadAll(req.Body)

	if err != nil {
		warn("raftd: Unable to read: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return 
	}

	debug("[recv] POST http://%v/set/%s [%s]", server.Name(), key, content)

	command := &SetCommand{}
	command.Key = key
	values := strings.Split(string(content), " ")

	command.Value = values[0]

	if len(values) == 2 {
		duration, err := strconv.Atoi(values[1])
		if err != nil {
			warn("raftd: Bad duration: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return 
		}
		command.ExpireTime = time.Now().Add(time.Second * (time.Duration)(duration))
	} else {
		command.ExpireTime = time.Unix(0,0)
	}

	excute(command, &w)

}

func DeleteHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/delete/"):]

	debug("[recv] GET http://%v/delete/%s", server.Name(), key)

	command := &DeleteCommand{}
	command.Key = key

	excute(command, &w)
}


func excute(c Command, w *http.ResponseWriter) {
	if server.State() == "leader" {
		if body, err := server.Do(c); err != nil {
			warn("raftd: Unable to write file: %v", err)
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		} else {
			(*w).WriteHeader(http.StatusOK)
			(*w).Write(body)
			return
		}
	} else {
		// tell the client where is the leader
		(*w).WriteHeader(http.StatusTemporaryRedirect)
		(*w).Write([]byte(server.Leader()))
		return
	}
	
	(*w).WriteHeader(http.StatusInternalServerError)

	return
} 

func MasterHttpHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(server.Leader()))
}

func GetHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/get/"):]

	debug("[recv] GET http://%v/get/%s", server.Name(), key)

	command := &GetCommand{}
	command.Key = key

	if body, err := command.Apply(server); err != nil {
		warn("raftd: Unable to write file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
		return
	}

}

func WatchHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/watch/"):]

	debug("[recv] GET http://%v/watch/%s", server.Name(), key)

	command := &WatchCommand{}
	command.Key = key

	if body, err := command.Apply(server); err != nil {
		warn("raftd: Unable to write file: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
		return
	}

}





