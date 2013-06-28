package main

import (
	"encoding/json"
	"github.com/xiangli-cmu/go-raft"
	"net/http"
	//"fmt"
	"io/ioutil"
	//"bytes"
	"strconv"
	"strings"
	"time"
)

//--------------------------------------
// Internal HTTP Handlers via server port
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

func clientHttpHandler(w http.ResponseWriter, req *http.Request) {
	debug("[recv] Get http://%v/client/ ", server.Name())
	w.WriteHeader(http.StatusOK)
	client := address + ":" + strconv.Itoa(clientPort)
	w.Write([]byte(client))
}

func JoinHttpHandler(w http.ResponseWriter, req *http.Request) {

	command := &JoinCommand{}

	if err := decodeJsonRequest(req, command); err == nil {
		debug("Receive Join Request from %s", command.Name)
		excute(command, &w, req)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

//--------------------------------------
// external HTTP Handlers via client port
//--------------------------------------
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
	values := strings.Split(string(content), ",")

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
		command.ExpireTime = time.Unix(0, 0)
	}

	excute(command, &w, req)

}

func DeleteHttpHandler(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Path[len("/delete/"):]

	debug("[recv] GET http://%v/delete/%s", server.Name(), key)

	command := &DeleteCommand{}
	command.Key = key

	excute(command, &w, req)
}

func excute(c Command, w *http.ResponseWriter, req *http.Request) {
	if server.State() == "leader" {
		if body, err := server.Do(c); err != nil {
			warn("Commit failed %v", err)
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		} else {
			(*w).WriteHeader(http.StatusOK)

			if body == nil {
				return
			}

			body, ok := body.([]byte)
			if !ok {
				panic("wrong type")
			}

			(*w).Write(body)
			return
		}
	} else {
		// tell the client where is the leader
		debug("Redirect to the leader %s", server.Leader())

		path := req.URL.Path

		var scheme string

		if scheme = req.URL.Scheme; scheme == "" {
			scheme = "http://"
		}

		url := scheme + leaderClient() + path

		debug("redirect to ", url)
		http.Redirect(*w, req, url, http.StatusTemporaryRedirect)
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

		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

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

		body, ok := body.([]byte)
		if !ok {
			panic("wrong type")
		}

		w.Write(body)
		return
	}

}
