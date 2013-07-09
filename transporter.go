package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-raft"
	"io"
	"io/ioutil"
	"net/http"
)

// Transporter layer for communication between raft nodes
type transporter struct {
	name string
	// If https is used for server internal communcation,
	// we will have a http client. Or it will be nil.
	client *http.Client
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t transporter) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("Send LogEntries to %s ", peer.Name())

	resp, _ := t.Post(fmt.Sprintf("%s/log/append", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.AppendEntriesResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {
			return aersp
		}

	}
	return aersp
}

// Sends RequestVote RPCs to a peer when the server is the candidate.
func (t transporter) SendVoteRequest(server *raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) *raft.RequestVoteResponse {
	var rvrsp *raft.RequestVoteResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("Send Vote to %s", peer.Name())

	resp, _ := t.Post(fmt.Sprintf("%s/vote", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		rvrsp := &raft.RequestVoteResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&rvrsp); err == nil || err == io.EOF {
			return rvrsp
		}

	}
	return rvrsp
}

// Sends SnapshotRequest RPCs to a peer when the server is the candidate.
func (t transporter) SendSnapshotRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) *raft.SnapshotResponse {
	var aersp *raft.SnapshotResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("Send Snapshot to %s [Last Term: %d, LastIndex %d]", peer.Name(),
		req.LastTerm, req.LastIndex)

	resp, err := t.Post(fmt.Sprintf("%s/snapshot", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.SnapshotResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {

			return aersp
		}
	}
	return aersp
}

// Get the the client address of the leader in the cluster
func (t transporter) GetLeaderClientAddress() string {
	resp, _ := t.Get(raftServer.Leader() + "/client")
	if resp != nil {
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		return string(body)
	}
	return ""
}

// Send server side POST request
func (t transporter) Post(path string, body io.Reader) (*http.Response, error) {

	if t.client != nil {
		resp, err := t.client.Post("https://"+path, "application/json", body)
		return resp, err
	} else {
		resp, err := http.Post("http://"+path, "application/json", body)
		return resp, err
	}
}

// Send server side GET request
func (t transporter) Get(path string) (*http.Response, error) {
	if t.client != nil {
		resp, err := t.client.Get("https://" + path)
		return resp, err
	} else {
		resp, err := http.Get("http://" + path)
		return resp, err
	}
}
