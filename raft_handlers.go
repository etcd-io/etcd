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

	if _, err := rvreq.Decode(req.Body); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		warnf("[recv] BADREQUEST %s/vote [%v]", r.url, err)
		return
	}

	debugf("[recv] POST %s/vote [%s]", r.url, rvreq.CandidateName)

	resp := r.RequestVote(rvreq)

	if resp == nil {
		warn("[vote] Error: nil response")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if _, err := resp.Encode(w); err != nil {
		warn("[vote] Error: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

// Response to append entries request
func AppendEntriesHttpHandler(w http.ResponseWriter, req *http.Request) {
	aereq := &raft.AppendEntriesRequest{}

	if _, err := aereq.Decode(req.Body); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		warnf("[recv] BADREQUEST %s/log/append [%v]", r.url, err)
		return
	}

	debugf("[recv] POST %s/log/append [%d]", r.url, len(aereq.Entries))

	r.serverStats.RecvAppendReq(aereq.LeaderName, int(req.ContentLength))

	resp := r.AppendEntries(aereq)

	if resp == nil {
		warn("[ae] Error: nil response")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		debugf("[Append Entry] Step back")
	}

	if _, err := resp.Encode(w); err != nil {
		warn("[ae] Error: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

// Response to recover from snapshot request
func SnapshotHttpHandler(w http.ResponseWriter, req *http.Request) {
	ssreq := &raft.SnapshotRequest{}

	if _, err := ssreq.Decode(req.Body); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		warnf("[recv] BADREQUEST %s/snapshot [%v]", r.url, err)
		return
	}

	debugf("[recv] POST %s/snapshot", r.url)

	resp := r.RequestSnapshot(ssreq)

	if resp == nil {
		warn("[ss] Error: nil response")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if _, err := resp.Encode(w); err != nil {
		warn("[ss] Error: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

// Response to recover from snapshot request
func SnapshotRecoveryHttpHandler(w http.ResponseWriter, req *http.Request) {
	ssrreq := &raft.SnapshotRecoveryRequest{}

	if _, err := ssrreq.Decode(req.Body); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		warnf("[recv] BADREQUEST %s/snapshotRecovery [%v]", r.url, err)
		return
	}

	debugf("[recv] POST %s/snapshotRecovery", r.url)

	resp := r.SnapshotRecoveryRequest(ssrreq)

	if resp == nil {
		warn("[ssr] Error: nil response")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if _, err := resp.Encode(w); err != nil {
		warn("[ssr] Error: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
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
