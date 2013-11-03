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
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

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
	client    *http.Client
	transport *http.Transport
}

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(scheme string, tlsConf tls.Config) *transporter {
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

	return &t
}

// Dial with timeout
func dialWithTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, dailTimeout)
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t *transporter) SendAppendEntriesRequest(server *raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		warn("transporter.ae.encoding.error:", err)
		return nil
	}

	size := b.Len()

	r.serverStats.SendAppendReq(size)

	u, _ := nameToRaftURL(peer.Name)

	debugf("Send LogEntries to %s ", u)

	thisFollowerStats, ok := r.followersStats.Followers[peer.Name]

	if !ok { //this is the first time this follower has been seen
		thisFollowerStats = &raftFollowerStats{}
		thisFollowerStats.Latency.Minimum = 1 << 63
		r.followersStats.Followers[peer.Name] = thisFollowerStats
	}

	start := time.Now()

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/log/append", u), &b)

	end := time.Now()

	if err != nil {
		debugf("Cannot send AppendEntriesRequest to %s: %s", u, err)
		if ok {
			thisFollowerStats.Fail()
		}
		return nil
	} else {
		if ok {
			thisFollowerStats.Succ(end.Sub(start))
		}
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		aeresp := &raft.AppendEntriesResponse{}
		if _, err = aeresp.Decode(resp.Body); err != nil && err != io.EOF {
			warn("transporter.ae.decoding.error:", err)
			return nil
		}
		return aeresp
	}

	return nil
}

// Sends RequestVote RPCs to a peer when the server is the candidate.
func (t *transporter) SendVoteRequest(server *raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) *raft.RequestVoteResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		warn("transporter.vr.encoding.error:", err)
		return nil
	}

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send Vote from %s to %s", server.Name(), u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/vote", u), &b)

	if err != nil {
		debugf("Cannot send VoteRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		rvrsp := &raft.RequestVoteResponse{}
		if _, err = rvrsp.Decode(resp.Body); err != nil && err != io.EOF {
			warn("transporter.vr.decoding.error:", err)
			return nil
		}
		return rvrsp
	}
	return nil
}

// Sends SnapshotRequest RPCs to a peer when the server is the candidate.
func (t *transporter) SendSnapshotRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) *raft.SnapshotResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		warn("transporter.ss.encoding.error:", err)
		return nil
	}

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send Snapshot Request from %s to %s", server.Name(), u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/snapshot", u), &b)

	if err != nil {
		debugf("Cannot send Snapshot Request to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		ssrsp := &raft.SnapshotResponse{}
		if _, err = ssrsp.Decode(resp.Body); err != nil && err != io.EOF {
			warn("transporter.ss.decoding.error:", err)
			return nil
		}
		return ssrsp
	}
	return nil
}

// Sends SnapshotRecoveryRequest RPCs to a peer when the server is the candidate.
func (t *transporter) SendSnapshotRecoveryRequest(server *raft.Server, peer *raft.Peer, req *raft.SnapshotRecoveryRequest) *raft.SnapshotRecoveryResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		warn("transporter.ss.encoding.error:", err)
		return nil
	}

	u, _ := nameToRaftURL(peer.Name)
	debugf("Send Snapshot Recovery from %s to %s", server.Name(), u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/snapshotRecovery", u), &b)

	if err != nil {
		debugf("Cannot send Snapshot Recovery to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		ssrrsp := &raft.SnapshotRecoveryResponse{}
		if _, err = ssrrsp.Decode(resp.Body); err != nil && err != io.EOF {
			warn("transporter.ssr.decoding.error:", err)
			return nil
		}
		return ssrrsp
	}
	return nil

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
		time.Sleep(tranTimeout)
		t.transport.CancelRequest(req)
	}()
}
