/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"net/http"

	"github.com/coreos/go-raft"
)

//-------------------------------------------------------------
// Handlers to handle raft related request via raft server port
//-------------------------------------------------------------

// Get all the current logs
func GetLogHttpHandler(w http.ResponseWriter, req *http.Request) {
	debugf("[recv] GET %s/log", r.url)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(r.LogEntries())
}

// Response to vote request
func VoteHttpHandler(w http.ResponseWriter, req *http.Request) {
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
func AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}
	err := decodeJsonRequest(req, aereq)

	if err == nil {
		debugf("[recv] POST %s/log/append [%d]", r.url, len(aereq.Entries))

		r.serverStats.RecvAppendReq(aereq.LeaderName, int(req.ContentLength))

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
func SnapshotHttpHandler(w http.ResponseWriter, req *http.Request) {
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
func SnapshotRecoveryHttpHandler(w http.ResponseWriter, req *http.Request) {
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
func EtcdURLHttpHandler(w http.ResponseWriter, req *http.Request) {
	debugf("[recv] Get %s/etcdURL/ ", r.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(argInfo.EtcdURL))
}

// Response to the join request
func JoinHttpHandler(w http.ResponseWriter, req *http.Request) error {

	command := &JoinCommand{}

	if err := decodeJsonRequest(req, command); err == nil {
		debugf("Receive Join Request from %s", command.Name)
		return dispatch(command, w, req, false)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return nil
	}
}

// Response to remove request
func RemoveHttpHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "DELETE" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	nodeName := req.URL.Path[len("/remove/"):]
	command := &RemoveCommand{
		Name: nodeName,
	}

	debugf("[recv] Remove Request [%s]", command.Name)

	dispatch(command, w, req, false)

}

// Response to the name request
func NameHttpHandler(w http.ResponseWriter, req *http.Request) {
	debugf("[recv] Get %s/name/ ", r.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(r.name))
}

// Response to the name request
func RaftVersionHttpHandler(w http.ResponseWriter, req *http.Request) {
	debugf("[recv] Get %s/version/ ", r.url)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(r.version))
}
