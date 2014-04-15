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
	"github.com/coreos/etcd/third_party/github.com/goraft/raft"
	httpclient "github.com/coreos/etcd/third_party/github.com/mreiferson/go-httpclient"
)

const (
	snapshotTimeout = time.Second * 120
)

// Transporter layer for communication between raft nodes
type transporter struct {
	followersStats *raftFollowersStats
	serverStats    *raftServerStats
	registry       *Registry

	client            *http.Client
	transport         *httpclient.Transport
	snapshotClient    *http.Client
	snapshotTransport *httpclient.Transport
}

type dialer func(network, addr string) (net.Conn, error)

// Create transporter using by raft server
// Create http or https transporter based on
// whether the user give the server cert and key
func NewTransporter(followersStats *raftFollowersStats, serverStats *raftServerStats, registry *Registry, dialTimeout, requestTimeout, responseHeaderTimeout time.Duration) *transporter {
	tr := &httpclient.Transport{
		ResponseHeaderTimeout: responseHeaderTimeout,
		// This is a workaround for Transport.CancelRequest doesn't work on
		// HTTPS connections blocked. The patch for it is in progress,
		// and would be available in Go1.3
		// More: https://codereview.appspot.com/69280043/
		ConnectTimeout: dialTimeout,
		RequestTimeout: requestTimeout,
	}

	// Sending snapshot might take a long time so we use a different HTTP transporter
	// Timeout is set to 120s (Around 100MB if the bandwidth is 10Mbits/s)
	// This timeout is not calculated by heartbeat time.
	// TODO(xiangl1) we can actually calculate the max bandwidth if we know
	// average RTT.
	// It should be equal to (TCP max window size/RTT).
	sTr := &httpclient.Transport{
		ConnectTimeout: dialTimeout,
		RequestTimeout: snapshotTimeout,
	}

	t := transporter{
		client:            &http.Client{Transport: tr},
		transport:         tr,
		snapshotClient:    &http.Client{Transport: sTr},
		snapshotTransport: sTr,
		followersStats:    followersStats,
		serverStats:       serverStats,
		registry:          registry,
	}

	return &t
}

func (t *transporter) SetTLSConfig(tlsConf tls.Config) {
	t.transport.TLSClientConfig = &tlsConf
	t.transport.DisableCompression = true

	t.snapshotTransport.TLSClientConfig = &tlsConf
	t.snapshotTransport.DisableCompression = true
}

// Sends AppendEntries RPCs to a peer when the server is the leader.
func (t *transporter) SendAppendEntriesRequest(server raft.Server, peer *raft.Peer, req *raft.AppendEntriesRequest) *raft.AppendEntriesResponse {
	var b bytes.Buffer

	if _, err := req.Encode(&b); err != nil {
		log.Warn("transporter.ae.encoding.error:", err)
		return nil
	}

	size := b.Len()

	t.serverStats.SendAppendReq(size)

	u, _ := t.registry.PeerURL(peer.Name)

	log.Debugf("Send LogEntries to %s ", u)

	thisFollowerStats, ok := t.followersStats.Followers[peer.Name]

	if !ok { //this is the first time this follower has been seen
		thisFollowerStats = &raftFollowerStats{}
		thisFollowerStats.Latency.Minimum = 1 << 63
		t.followersStats.Followers[peer.Name] = thisFollowerStats
	}

	start := time.Now()

	resp, _, err := t.Post(fmt.Sprintf("%s/log/append", u), &b)

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

	u, _ := t.registry.PeerURL(peer.Name)
	log.Debugf("Send Vote from %s to %s", server.Name(), u)

	resp, _, err := t.Post(fmt.Sprintf("%s/vote", u), &b)

	if err != nil {
		log.Debugf("Cannot send VoteRequest to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

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

	u, _ := t.registry.PeerURL(peer.Name)
	log.Debugf("Send Snapshot Request from %s to %s", server.Name(), u)

	resp, _, err := t.Post(fmt.Sprintf("%s/snapshot", u), &b)

	if err != nil {
		log.Debugf("Cannot send Snapshot Request to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

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

	u, _ := t.registry.PeerURL(peer.Name)
	log.Debugf("Send Snapshot Recovery from %s to %s", server.Name(), u)

	resp, err := t.PostSnapshot(fmt.Sprintf("%s/snapshotRecovery", u), &b)

	if err != nil {
		log.Debugf("Cannot send Snapshot Recovery to %s : %s", u, err)
	}

	if resp != nil {
		defer resp.Body.Close()

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

// Send server side PUT request
func (t *transporter) Put(urlStr string, body io.Reader) (*http.Response, *http.Request, error) {
	req, _ := http.NewRequest("PUT", urlStr, body)
	resp, err := t.client.Do(req)
	return resp, req, err
}

// PostSnapshot posts a json format snapshot to the given url
// The underlying HTTP transport has a minute level timeout
func (t *transporter) PostSnapshot(url string, body io.Reader) (*http.Response, error) {
	return t.snapshotClient.Post(url, "application/json", body)
}
