package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/coreos/go-raft"
	"io"
	"net"
	"net/http"
	"time"
)

// Transporter layer for communication between raft nodes
type transporter struct {
	client  *http.Client
	timeout time.Duration
}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(scheme string, tlsConf tls.Config, timeout time.Duration) transporter {
	t := transporter{}

	tr := &http.Transport{
		Dial: dialTimeout,
	}

	if scheme == "https" {
		tr.TLSClientConfig = &tlsConf
		tr.DisableCompression = true
	}

	t.client = &http.Client{Transport: tr}
	t.timeout = timeout

	return t
}

// Dial with timeout
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, HTTPTimeout)
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t transporter) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send LogEntries to %s ", u)

	resp, err := t.Post(fmt.Sprintf("%s/log/append", u), &b)

	if err != nil {
		debugf("Cannot send AppendEntriesRequest to %s: %s", u, err)
	}

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

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send Vote to %s", u)

	resp, err := t.Post(fmt.Sprintf("%s/vote", u), &b)

	if err != nil {
		debugf("Cannot send VoteRequest to %s : %s", u, err)
	}

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

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send Snapshot to %s [Last Term: %d, LastIndex %d]", u,
		req.LastTerm, req.LastIndex)

	resp, err := t.Post(fmt.Sprintf("%s/snapshot", u), &b)

	if err != nil {
		debugf("Cannot send SendSnapshotRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.SnapshotResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {

			return aersp
		}
	}

	return aersp
}

// Sends SnapshotRecoveryRequest RPCs to a peer when the server is the candidate.
func (t transporter) SendSnapshotRecoveryRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRecoveryRequest) *raft.SnapshotRecoveryResponse {
	var aersp *raft.SnapshotRecoveryResponse
	var b bytes.Buffer
	json.NewEncoder(&b).Encode(req)

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send SnapshotRecovery to %s [Last Term: %d, LastIndex %d]", u,
		req.LastTerm, req.LastIndex)

	resp, err := t.Post(fmt.Sprintf("%s/snapshotRecovery", u), &b)

	if err != nil {
		debugf("Cannot send SendSnapshotRecoveryRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()
		aersp = &raft.SnapshotRecoveryResponse{}
		if err = json.NewDecoder(resp.Body).Decode(&aersp); err == nil || err == io.EOF {
			return aersp
		}
	}

	return aersp
}

// Send server side POST request
func (t transporter) Post(path string, body io.Reader) (*http.Response, error) {

	postChan := make(chan interface{}, 1)

	go func() {
		resp, err := t.client.Post(path, "application/json", body)
		if err == nil {
			postChan <- resp
		} else {
			postChan <- err
		}
	}()

	return t.waitResponse(postChan)

}

// Send server side GET request
func (t transporter) Get(path string) (*http.Response, error) {

	getChan := make(chan interface{}, 1)

	go func() {
		resp, err := t.client.Get(path)
		if err == nil {
			getChan <- resp
		} else {
			getChan <- err
		}
	}()

	return t.waitResponse(getChan)
}

func (t transporter) waitResponse(responseChan chan interface{}) (*http.Response, error) {

	timeoutChan := time.After(t.timeout)

	select {
	case <-timeoutChan:
		return nil, fmt.Errorf("Wait Response Timeout: %v", t.timeout)

	case r := <-responseChan:
		switch r := r.(type) {
		case error:
			return nil, r

		case *http.Response:
			return r, nil

		}
	}

	// for complier
	return nil, nil
}
