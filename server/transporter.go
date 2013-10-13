package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/go-raft"
)

// Timeout for setup internal raft http connection
// This should not exceed 3 * RTT
var dailTimeout = 3 * HeartbeatTimeout

// Timeout for setup internal raft http connection + receive response header
// This should not exceed 3 * RTT + RTT
var responseHeaderTimeout = 4 * HeartbeatTimeout

// Timeout for receiving the response body from the server
// This should not exceed election timeout
var tranTimeout = ElectionTimeout

// Transporter layer for communication between raft nodes
type transporter struct {
	client     *http.Client
	transport  *http.Transport
	peerServer *PeerServer
}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(scheme string, tlsConf tls.Config, peerServer *PeerServer) *transporter {
	t := transporter{}

	tr := &http.Transport{
		Dial: dialWithTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}

	if scheme == "https" {
		tr.TLSClientConfig = &tlsConf
		tr.DisableCompression = true
	}

	t.client = &http.Client{Transport: tr}
	t.transport = tr
	t.peerServer = peerServer

	return &t
}

// Dial with timeout
func dialWithTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, dailTimeout)
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t *transporter) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var aersp *raft.AppendEntriesResponse
	var b bytes.Buffer

	json.NewEncoder(&b).Encode(req)

	size := b.Len()

	t.peerServer.serverStats.SendAppendReq(size)

	u, _ := t.peerServer.registry.PeerURL(peer.Name)

	log.Debugf("Send LogEntries to %s ", u)

	thisFollowerStats, ok := t.peerServer.followersStats.Followers[peer.Name]

	if !ok { //this is the first time this follower has been seen
		thisFollowerStats = &raftFollowerStats{}
		thisFollowerStats.Latency.Minimum = 1 << 63
		t.peerServer.followersStats.Followers[peer.Name] = thisFollowerStats
	}

	start := time.Now()

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/log/append", u), &b)

	end := time.Now()

	if err != nil {
		log.Debugf("Cannot send AppendEntriesRequest to %s: %s", u, err)
		if ok {
			thisFollowerStats.Fail()
		}
	} else {
		if ok {
			thisFollowerStats.Succ(end.Sub(start))
		}
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

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

	u, _ := t.peerServer.registry.PeerURL(peer.Name)
	log.Debugf("Send Vote to %s", u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/vote", u), &b)

	if err != nil {
		log.Debugf("Cannot send VoteRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

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

	u, _ := t.peerServer.registry.PeerURL(peer.Name)
	log.Debugf("Send Snapshot to %s [Last Term: %d, LastIndex %d]", u,
		req.LastTerm, req.LastIndex)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/snapshot", u), &b)

	if err != nil {
		log.Debugf("Cannot send SendSnapshotRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

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

	u, _ := t.peerServer.registry.PeerURL(peer.Name)
	log.Debugf("Send SnapshotRecovery to %s [Last Term: %d, LastIndex %d]", u,
		req.LastTerm, req.LastIndex)

	resp, _, err := t.Post(fmt.Sprintf("%s/snapshotRecovery", u), &b)

	if err != nil {
		log.Debugf("Cannot send SendSnapshotRecoveryRequest to %s : %s", u, err)
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
func (t *transporter) Post(urlStr string, body io.Reader) (*http.Response, *http.Request, error) {
	req, _ := http.NewRequest("POST", urlStr, body)
	resp, err := t.client.Do(req)
	return resp, req, err
}

// Send server side GET request
func (t *transporter) Get(urlStr string) (*http.Response, *http.Request, error) {
	req, _ := http.NewRequest("GET", urlStr, nil)
	resp, err := t.client.Do(req)
	return resp, req, err
}

// Cancel the on fly HTTP transaction when timeout happens.
func (t *transporter) CancelWhenTimeout(req *http.Request) {
	go func() {
		time.Sleep(ElectionTimeout)
		t.transport.CancelRequest(req)
	}()
}
