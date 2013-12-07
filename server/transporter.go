package server

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coreos/etcd/log"
	"github.com/coreos/raft"
)

// Transporter layer for communication between raft nodes
type transporter struct {
	client     *http.Client
	transport  *http.Transport
	peerServer *PeerServer
}

type dialer func(network, addr string) (net.Conn, error)

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func newTransporter(scheme string, tlsConf tls.Config, peerServer *PeerServer) *transporter {
	// names for each type of timeout, for the sake of clarity
	dialTimeout := (3 * peerServer.HeartbeatTimeout) + peerServer.ElectionTimeout
	responseHeaderTimeout := (3 * peerServer.HeartbeatTimeout) + peerServer.ElectionTimeout

	t := transporter{}

	tr := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, dialTimeout)
		},
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

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t *transporter) SendAppendEntriesRequest(server raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		log.Warn("transporter.ae.encoding.error:", err)
		return nil
	}

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
			log.Warn("transporter.ae.decoding.error:", err)
			return nil
		}
		return aeresp
	}

	return nil
}

// Sends RequestVote RPCs to a peer when the server is the candidate.
func (t *transporter) SendVoteRequest(server raft.Server, peer *raft.Peer, req *raft.RequestVoteRequest) *raft.RequestVoteResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		log.Warn("transporter.vr.encoding.error:", err)
		return nil
	}

	u, _ := t.peerServer.registry.PeerURL(peer.Name)
	log.Debugf("Send Vote from %s to %s", server.Name(), u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/vote", u), &b)

	if err != nil {
		log.Debugf("Cannot send VoteRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		rvrsp := &raft.RequestVoteResponse{}
		if _, err = rvrsp.Decode(resp.Body); err != nil && err != io.EOF {
			log.Warn("transporter.vr.decoding.error:", err)
			return nil
		}
		return rvrsp
	}
	return nil
}

// Sends SnapshotRequest RPCs to a peer when the server is the candidate.
func (t *transporter) SendSnapshotRequest(server raft.Server, peer *raft.Peer, req *raft.SnapshotRequest) *raft.SnapshotResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		log.Warn("transporter.ss.encoding.error:", err)
		return nil
	}

	u, _ := t.peerServer.registry.PeerURL(peer.Name)
	log.Debugf("Send Snapshot Request from %s to %s", server.Name(), u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/snapshot", u), &b)

	if err != nil {
		log.Debugf("Cannot send Snapshot Request to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		ssrsp := &raft.SnapshotResponse{}
		if _, err = ssrsp.Decode(resp.Body); err != nil && err != io.EOF {
			log.Warn("transporter.ss.decoding.error:", err)
			return nil
		}
		return ssrsp
	}
	return nil
}

// Sends SnapshotRecoveryRequest RPCs to a peer when the server is the candidate.
func (t *transporter) SendSnapshotRecoveryRequest(server raft.Server, peer *raft.Peer, req *raft.SnapshotRecoveryRequest) *raft.SnapshotRecoveryResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		log.Warn("transporter.ss.encoding.error:", err)
		return nil
	}

	u, _ := t.peerServer.registry.PeerURL(peer.Name)
	log.Debugf("Send Snapshot Recovery from %s to %s", server.Name(), u)

	resp, httpRequest, err := t.Post(fmt.Sprintf("%s/snapshotRecovery", u), &b)

	if err != nil {
		log.Debugf("Cannot send Snapshot Recovery to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

		t.CancelWhenTimeout(httpRequest)

		ssrrsp := &raft.SnapshotRecoveryResponse{}
		if _, err = ssrrsp.Decode(resp.Body); err != nil && err != io.EOF {
			log.Warn("transporter.ssr.decoding.error:", err)
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
		time.Sleep(t.peerServer.HeartbeatTimeout)
		t.transport.CancelRequest(req)
	}()
}
