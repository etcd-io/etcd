package raft

import (
	"sync"
	"time"
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// A peer is a reference to another server involved in the consensus protocol.
type Peer struct {
	server           *Server
	name             string
	prevLogIndex     uint64
	mutex            sync.RWMutex
	stopChan         chan bool
	heartbeatTimeout time.Duration
}

//------------------------------------------------------------------------------
//
// Constructor
//
//------------------------------------------------------------------------------

// Creates a new peer.
func newPeer(server *Server, name string, heartbeatTimeout time.Duration) *Peer {
	return &Peer{
		server:           server,
		name:             name,
		heartbeatTimeout: heartbeatTimeout,
	}
}

//------------------------------------------------------------------------------
//
// Accessors
//
//------------------------------------------------------------------------------

// Retrieves the name of the peer.
func (p *Peer) Name() string {
	return p.name
}

// Sets the heartbeat timeout.
func (p *Peer) setHeartbeatTimeout(duration time.Duration) {
	p.heartbeatTimeout = duration
}

//--------------------------------------
// Prev log index
//--------------------------------------

// Retrieves the previous log index.
func (p *Peer) getPrevLogIndex() uint64 {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.prevLogIndex
}

// Sets the previous log index.
func (p *Peer) setPrevLogIndex(value uint64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.prevLogIndex = value
}

//------------------------------------------------------------------------------
//
// Methods
//
//------------------------------------------------------------------------------

//--------------------------------------
// Heartbeat
//--------------------------------------

// Starts the peer heartbeat.
func (p *Peer) startHeartbeat() {
	p.stopChan = make(chan bool, 1)
	c := make(chan bool)
	go p.heartbeat(c)
	<-c
}

// Stops the peer heartbeat.
func (p *Peer) stopHeartbeat() {
	// here is a problem
	// the previous stop is no buffer leader may get blocked
	// when heartbeat returns at line 132
	// I make the channel with 1 buffer
	// and try to panic here
	select {
	case p.stopChan <- true:

	default:
		panic("[" + p.server.Name() + "] cannot stop [" + p.Name() + "] heartbeat")
	}
}

//--------------------------------------
// Copying
//--------------------------------------

// Clones the state of the peer. The clone is not attached to a server and
// the heartbeat timer will not exist.
func (p *Peer) clone() *Peer {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return &Peer{
		name:         p.name,
		prevLogIndex: p.prevLogIndex,
	}
}

//--------------------------------------
// Heartbeat
//--------------------------------------

// Listens to the heartbeat timeout and flushes an AppendEntries RPC.
func (p *Peer) heartbeat(c chan bool) {
	stopChan := p.stopChan

	c <- true

	debugln("peer.heartbeat: ", p.Name(), p.heartbeatTimeout)

	for {
		select {
		case <-stopChan:
			debugln("peer.heartbeat.stop: ", p.Name())
			return

		case <-time.After(p.heartbeatTimeout):
			debugln("peer.heartbeat.run: ", p.Name())
			prevLogIndex := p.getPrevLogIndex()
			entries, prevLogTerm := p.server.log.getEntriesAfter(prevLogIndex, p.server.maxLogEntriesPerRequest)

			if p.server.State() != Leader {
				return
			}

			if entries != nil {
				p.sendAppendEntriesRequest(newAppendEntriesRequest(p.server.currentTerm, prevLogIndex, prevLogTerm, p.server.log.CommitIndex(), p.server.name, entries))
			} else {
				p.sendSnapshotRequest(newSnapshotRequest(p.server.name, p.server.lastSnapshot))
			}
		}
	}
}

//--------------------------------------
// Append Entries
//--------------------------------------

// Sends an AppendEntries request to the peer through the transport.
func (p *Peer) sendAppendEntriesRequest(req *AppendEntriesRequest) {
	traceln("peer.flush.send: ", p.server.Name(), "->", p.Name(), " ", len(req.Entries))

	resp := p.server.Transporter().SendAppendEntriesRequest(p.server, p, req)
	if resp == nil {
		debugln("peer.flush.timeout: ", p.server.Name(), "->", p.Name())
		return
	}
	traceln("peer.flush.recv: ", p.Name())

	// If successful then update the previous log index.
	p.mutex.Lock()
	if resp.Success {
		if len(req.Entries) > 0 {
			p.prevLogIndex = req.Entries[len(req.Entries)-1].Index

			// if peer append a log entry from the current term
			// we set append to true
			if req.Entries[len(req.Entries)-1].Term == p.server.currentTerm {
				resp.append = true
			}
		}
		traceln("peer.flush.success: ", p.server.Name(), "->", p.Name(), "; idx =", p.prevLogIndex)

		// If it was unsuccessful then decrement the previous log index and
		// we'll try again next time.
	} else {
		if resp.CommitIndex >= p.prevLogIndex {

			// we may miss a response from peer
			// so maybe the peer has commited the logs we sent
			// but we did not receive the success reply and did not increase
			// the prevLogIndex

			p.prevLogIndex = resp.CommitIndex

			debugln("peer.flush.commitIndex: ", p.server.Name(), "->", p.Name(), " idx =", p.prevLogIndex)
		} else if p.prevLogIndex > 0 {
			// Decrement the previous log index down until we find a match. Don't
			// let it go below where the peer's commit index is though. That's a
			// problem.
			p.prevLogIndex--
			// if it not enough, we directly decrease to the index of the
			if p.prevLogIndex > resp.Index {
				p.prevLogIndex = resp.Index
			}

			debugln("peer.flush.decrement: ", p.server.Name(), "->", p.Name(), " idx =", p.prevLogIndex)
		}
	}
	p.mutex.Unlock()

	// Attach the peer to resp, thus server can know where it comes from
	resp.peer = p.Name()
	// Send response to server for processing.
	p.server.send(resp)
}

// Sends an Snapshot request to the peer through the transport.
func (p *Peer) sendSnapshotRequest(req *SnapshotRequest) {
	debugln("peer.snap.send: ", p.name)

	resp := p.server.Transporter().SendSnapshotRequest(p.server, p, req)
	if resp == nil {
		debugln("peer.snap.timeout: ", p.name)
		return
	}

	debugln("peer.snap.recv: ", p.name)

	// If successful, the peer should have been to snapshot state
	// Send it the snapshot!
	if resp.Success {
		p.sendSnapshotRecoveryRequest()
	} else {
		debugln("peer.snap.failed: ", p.name)
		return
	}

}

// Sends an Snapshot Recovery request to the peer through the transport.
func (p *Peer) sendSnapshotRecoveryRequest() {
	req := newSnapshotRecoveryRequest(p.server.name, p.server.lastSnapshot)
	debugln("peer.snap.recovery.send: ", p.name)
	resp := p.server.Transporter().SendSnapshotRecoveryRequest(p.server, p, req)
	if resp.Success {
		p.prevLogIndex = req.LastIndex
	} else {
		debugln("peer.snap.recovery.failed: ", p.name)
		return
	}
	// Send response to server for processing.
	p.server.send(&AppendEntriesResponse{Term: resp.Term, Success: resp.Success, append: (resp.Term == p.server.currentTerm)})
}

//--------------------------------------
// Vote Requests
//--------------------------------------

// send VoteRequest Request
func (p *Peer) sendVoteRequest(req *RequestVoteRequest, c chan *RequestVoteResponse) {
	debugln("peer.vote: ", p.server.Name(), "->", p.Name())
	req.peer = p
	if resp := p.server.Transporter().SendVoteRequest(p.server, p, req); resp != nil {
		debugln("peer.vote: recv", p.server.Name(), "<-", p.Name())
		resp.peer = p
		c <- resp
	}
}
