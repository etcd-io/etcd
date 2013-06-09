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
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t transHandler) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)
	debug("[send] POST http://%s/log/append [%d]", peer.Name(), len(req.Entries))
	resp, err := http.Post(fmt.Sprintf("http://%s/log/append", peer.Name()), "application/json", &b)
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
	debug("[send] POST http://%s/vote", peer.Name())
	resp, err := http.Post(fmt.Sprintf("http://%s/vote", peer.Name()), "application/json", &b)
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
	debug("[send] POST http://%s/snapshot [%d %d]", peer.Name(), req.LastTerm, req.LastIndex)
	resp, err := http.Post(fmt.Sprintf("http://%s/snapshot", peer.Name()), "application/json", &b)
	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.SnapshotResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {

			return aersp, nil
		}
	}
	fmt.Println("error send snapshot")
	return aersp, fmt.Errorf("raftd: Unable to send snapshot: %v", err)
}