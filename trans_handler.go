package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/benbjohnson/go-raft"
	"io"
	"net/http"
)

type transHandler struct {
	name   string
	client *http.Client
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t transHandler) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("Send LogEntries to %s ", peer.Name())

	resp, err := Post(&t, fmt.Sprintf("%s/log/append", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.AppendEntriesResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {
			return aersp, nil
		}

	}
	return aersp, fmt.Errorf("raftd: Unable to append entries: %v", err)
}

// Sends RequestVote RPCs to a peer when the server is the candidate.
func (t transHandler) SendVoteRequest(server *raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	var rvrsp *raft.RequestVoteResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("Send Vote to %s", peer.Name())

	resp, err := Post(&t, fmt.Sprintf("%s/vote", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		rvrsp := &raft.RequestVoteResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&rvrsp); err == nil || err == io.EOF {
			return rvrsp, nil
		}

	}
	return rvrsp, fmt.Errorf("Unable to request vote: %v", err)
}

// Sends SnapshotRequest RPCs to a peer when the server is the candidate.
func (t transHandler) SendSnapshotRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) (*raft.SnapshotResponse, error) {
	var aersp *raft.SnapshotResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("Send Snapshot to %s [Last Term: %d, LastIndex %d]", peer.Name(),
		req.LastTerm, req.LastIndex)

	resp, err := Post(&t, fmt.Sprintf("%s/snapshot", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.SnapshotResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {

			return aersp, nil
		}
	}
	return aersp, fmt.Errorf("Unable to send snapshot: %v", err)
}
