package main

import (
	"encoding/json"
	"github.com/coreos/go-raft"
	"io"
	"net/http"
)

//-------------------------------------------------------------
// Handlers to handle raft related request via raft server port
//-------------------------------------------------------------

// Get all the current logs
func GetLogHttpHandler(w ResponseWriter, req *http.Request) {
	debugf("[recv] GET %s/log", r.url)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(r.LogEntries())
}

// Response to vote request
func VoteHttpHandler(w ResponseWriter, req *http.Request) {
	rvreq := &raft.RequestVoteRequest{}
	err := decodeJsonRequest(req, rvreq)
	if err == nil {
		debugf("[recv] POST %s/vote [%s]", r.url, rvreq.CandidateName)
		if resp := r.RequestVote(rvreq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	warnf("[vote] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to append entries request
func AppendEntriesHttpHandler(w ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)

	if err == nil {
		debugf("[recv] POST %s/log/append [%d]", r.url, len(aereq.Entries))
		if resp := r.AppendEntries(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			if !resp.Success {
				debugf("[Append Entry] Step back")
			}
			return
		}
	}
	warnf("[Append Entry] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to recover from snapshot request
func SnapshotHttpHandler(w ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		debugf("[recv] POST %s/snapshot/ ", r.url)
		if resp := r.RequestSnapshot(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	warnf("[Snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Response to recover from snapshot request
func SnapshotRecoveryHttpHandler(w ResponseWriter, req *http.Request) {
	aereq := &raft.SnapshotRecoveryRequest{}
	err := decodeJsonRequest(req, aereq)
	if err == nil {
		debugf("[recv] POST %s/snapshotRecovery/ ", r.url)
		if resp := r.SnapshotRecoveryRequest(aereq); resp != nil {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}
	}
	warnf("[Snapshot] ERROR: %v", err)
	w.WriteHeader(http.StatusInternalServerError)
}

// Get the port that listening for etcd connecting of the server
func EtcdURLHttpHandler(w ResponseWriter, req *http.Request) {
	debugf("[recv] Get %s/etcdURL/ ", r.url)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, argInfo.EtcdURL)
}

// Response to the join request
func JoinHttpHandler(w ResponseWriter, req *http.Request) {

	command := &JoinCommand{}

	if err := decodeJsonRequest(req, command); err == nil {
		debugf("Receive Join Request from %s", command.Name)
		dispatch(command, w, req, false)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Response to the name request
func NameHttpHandler(w ResponseWriter, req *http.Request) {
	debugf("[recv] Get %s/name/ ", r.url)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, r.name)
}
