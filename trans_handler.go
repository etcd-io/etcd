package main

import(
	"encoding/json"
	"github.com/benbjohnson/go-raft"
	"bytes"
	"net/http"
	"fmt"
	"io"
)

type transHandler struct {
	name string
	client *http.Client
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t transHandler) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)
	
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

	resp, err := Post(&t, fmt.Sprintf("%s/vote", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		rvrsp := &raft.RequestVoteResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&rvrsp); err == nil || err == io.EOF {
			return rvrsp, nil
		}
		
	}
	return rvrsp, fmt.Errorf("raftd: Unable to request vote: %v", err)
}

// Sends SnapshotRequest RPCs to a peer when the server is the candidate.
func (t transHandler) SendSnapshotRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) (*raft.SnapshotResponse, error) {
	var aersp *raft.SnapshotResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	debug("[send] POST %s/snapshot [%d %d]", peer.Name(), req.LastTerm, req.LastIndex)

	resp, err := Post(&t, fmt.Sprintf("%s/snapshot", peer.Name()), &b)

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.SnapshotResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {

			return aersp, nil
		}
	}
	return aersp, fmt.Errorf("raftd: Unable to send snapshot: %v", err)
}