package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coreos/go-raft"
)

// Transporter layer for communication between raft nodes
type transporter struct {
	client  *http.Client
	timeout time.Duration
}

// response struct
type transporterResponse struct {
	resp *http.Response
	err  error
}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(scheme string, tlsConf tls.Config, timeout time.Duration) *transporter {
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

	return &t
}

// Dial with timeout
func dialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, HTTPTimeout)
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t *transporter) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer

	json.NewEncoder(&b).Encode(req)

	size := b.Len()

	r.serverStats.SendAppendReq(size)

	u, _ := nameToRaftURL(peer.Name)

	debugf("Send LogEntries to %s ", u)

	thisPeerStats, ok := r.peersStats.Peers[peer.Name]

	start := time.Now()

	resp, err := t.Post(fmt.Sprintf("%s/log/append", u), &b)

	end := time.Now()

	if err != nil {
		debugf("Cannot send AppendEntriesRequest to %s: %s", u, err)
		if ok {
			thisPeerStats.Fail()
		}
	} else {
		if ok {
			thisPeerStats.Succ(end.Sub(start))
		}
	}

	r.peersStats.Peers[peer.Name] = thisPeerStats

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
func (t *transporter) SendVoteRequest(server *raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) *raft.RequestVoteResponse {
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
func (t *transporter) SendSnapshotRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) *raft.SnapshotResponse {
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
func (t *transporter) SendSnapshotRecoveryRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRecoveryRequest) *raft.SnapshotRecoveryResponse {
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
func (t *transporter) Post(path string, body io.Reader) (*http.Response, error) {

	c := make(chan *transporterResponse, 1)

	go func() {
		tr := new(transporterResponse)
		tr.resp, tr.err = t.client.Post(path, "application/json", body)
		c <- tr
	}()

	return t.waitResponse(c)

}

// Send server side GET request
func (t *transporter) Get(path string) (*http.Response, error) {

	c := make(chan *transporterResponse, 1)

	go func() {
		tr := new(transporterResponse)
		tr.resp, tr.err = t.client.Get(path)
		c <- tr
	}()

	return t.waitResponse(c)
}

func (t *transporter) waitResponse(responseChan chan *transporterResponse) (*http.Response, error) {

	timeoutChan := time.After(t.timeout)

	select {
	case <-timeoutChan:
		return nil, fmt.Errorf("Wait Response Timeout: %v", t.timeout)

	case r := <-responseChan:
		return r.resp, r.err
	}

	// for complier
	return nil, nil
}
