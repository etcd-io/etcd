package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"sync"
	"time"
)

//------------------------------------------------------------------------------
//
// Constants
//
//------------------------------------------------------------------------------

const (
	Stopped      = "stopped"
	Follower     = "follower"
	Candidate    = "candidate"
	Leader       = "leader"
	Snapshotting = "snapshotting"
)

const (
	MaxLogEntriesPerRequest         = 2000
	NumberOfLogEntriesAfterSnapshot = 200
)

const (
	DefaultHeartbeatTimeout = 50 * time.Millisecond
	DefaultElectionTimeout  = 150 * time.Millisecond
)

var stopValue interface{}

//------------------------------------------------------------------------------
//
// Errors
//
//------------------------------------------------------------------------------

var NotLeaderError = errors.New("raft.Server: Not current leader")
var DuplicatePeerError = errors.New("raft.Server: Duplicate peer")
var CommandTimeoutError = errors.New("raft: Command timeout")

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// A server is involved in the consensus protocol and can act as a follower,
// candidate or a leader.
type Server struct {
	name        string
	path        string
	state       string
	transporter Transporter
	context     interface{}
	currentTerm uint64

	votedFor   string
	log        *Log
	leader     string
	peers      map[string]*Peer
	mutex      sync.RWMutex
	syncedPeer map[string]bool

	c                chan *event
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration

	currentSnapshot         *Snapshot
	lastSnapshot            *Snapshot
	stateMachine            StateMachine
	maxLogEntriesPerRequest uint64
}

// An event to be processed by the server's event loop.
type event struct {
	target      interface{}
	returnValue interface{}
	c           chan error
}

//------------------------------------------------------------------------------
//
// Constructor
//
//------------------------------------------------------------------------------

// Creates a new server with a log at the given path.
func NewServer(name string, path string, transporter Transporter, stateMachine StateMachine, context interface{}) (*Server, error) {
	if name == "" {
		return nil, errors.New("raft.Server: Name cannot be blank")
	}
	if transporter == nil {
		panic("raft: Transporter required")
	}

	s := &Server{
		name:                    name,
		path:                    path,
		transporter:             transporter,
		stateMachine:            stateMachine,
		context:                 context,
		state:                   Stopped,
		peers:                   make(map[string]*Peer),
		log:                     newLog(),
		c:                       make(chan *event, 256),
		electionTimeout:         DefaultElectionTimeout,
		heartbeatTimeout:        DefaultHeartbeatTimeout,
		maxLogEntriesPerRequest: MaxLogEntriesPerRequest,
	}

	// Setup apply function.
	s.log.ApplyFunc = func(c Command) (interface{}, error) {
		result, err := c.Apply(s)
		return result, err
	}

	return s, nil
}

//------------------------------------------------------------------------------
//
// Accessors
//
//------------------------------------------------------------------------------

//--------------------------------------
// General
//--------------------------------------

// Retrieves the name of the server.
func (s *Server) Name() string {
	return s.name
}

// Retrieves the storage path for the server.
func (s *Server) Path() string {
	return s.path
}

// The name of the current leader.
func (s *Server) Leader() string {
	return s.leader
}

// Retrieves a copy of the peer data.
func (s *Server) Peers() map[string]*Peer {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	peers := make(map[string]*Peer)
	for name, peer := range s.peers {
		peers[name] = peer.clone()
	}
	return peers
}

// Retrieves the object that transports requests.
func (s *Server) Transporter() Transporter {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.transporter
}

func (s *Server) SetTransporter(t Transporter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.transporter = t
}

// Retrieves the context passed into the constructor.
func (s *Server) Context() interface{} {
	return s.context
}

// Retrieves the log path for the server.
func (s *Server) LogPath() string {
	return path.Join(s.path, "log")
}

// Retrieves the current state of the server.
func (s *Server) State() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state
}

// Sets the state of the server.
func (s *Server) setState(state string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.state = state
	if state == Leader {
		s.leader = s.Name()
	}
}

// Retrieves the current term of the server.
func (s *Server) Term() uint64 {
	return s.currentTerm
}

// Retrieves the current commit index of the server.
func (s *Server) CommitIndex() uint64 {
	return s.log.commitIndex
}

// Retrieves the name of the candidate this server voted for in this term.
func (s *Server) VotedFor() string {
	return s.votedFor
}

// Retrieves whether the server's log has no entries.
func (s *Server) IsLogEmpty() bool {
	return s.log.isEmpty()
}

// A list of all the log entries. This should only be used for debugging purposes.
func (s *Server) LogEntries() []*LogEntry {
	return s.log.entries
}

// A reference to the command name of the last entry.
func (s *Server) LastCommandName() string {
	return s.log.lastCommandName()
}

// Get the state of the server for debugging
func (s *Server) GetState() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return fmt.Sprintf("Name: %s, State: %s, Term: %v, CommitedIndex: %v ", s.name, s.state, s.currentTerm, s.log.commitIndex)
}

// Check if the server is promotable
func (s *Server) promotable() bool {
	return s.log.currentIndex() > 0
}

//--------------------------------------
// Membership
//--------------------------------------

// Retrieves the number of member servers in the consensus.
func (s *Server) MemberCount() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.peers) + 1
}

// Retrieves the number of servers required to make a quorum.
func (s *Server) QuorumSize() int {
	return (s.MemberCount() / 2) + 1
}

//--------------------------------------
// Election timeout
//--------------------------------------

// Retrieves the election timeout.
func (s *Server) ElectionTimeout() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.electionTimeout
}

// Sets the election timeout.
func (s *Server) SetElectionTimeout(duration time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.electionTimeout = duration
}

//--------------------------------------
// Heartbeat timeout
//--------------------------------------

// Retrieves the heartbeat timeout.
func (s *Server) HeartbeatTimeout() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.heartbeatTimeout
}

// Sets the heartbeat timeout.
func (s *Server) SetHeartbeatTimeout(duration time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.heartbeatTimeout = duration
	for _, peer := range s.peers {
		peer.setHeartbeatTimeout(duration)
	}
}

//------------------------------------------------------------------------------
//
// Methods
//
//------------------------------------------------------------------------------

//--------------------------------------
// Initialization
//--------------------------------------

// Reg the NOPCommand
func init() {
	RegisterCommand(&NOPCommand{})
	RegisterCommand(&DefaultJoinCommand{})
	RegisterCommand(&DefaultLeaveCommand{})
}

// Start as follow
// If log entries exist then allow promotion to candidate if no AEs received.
// If no log entries exist then wait for AEs from another node.
// If no log entries exist and a self-join command is issued then
// immediately become leader and commit entry.

func (s *Server) Start() error {
	// Exit if the server is already running.
	if s.state != Stopped {
		return errors.New("raft.Server: Server already running")
	}

	// Create snapshot directory if not exist
	os.Mkdir(path.Join(s.path, "snapshot"), 0700)

	if err := s.readConf(); err != nil {
		s.debugln("raft: Conf file error: ", err)
		return fmt.Errorf("raft: Initialization error: %s", err)
	}

	// Initialize the log and load it up.
	if err := s.log.open(s.LogPath()); err != nil {
		s.debugln("raft: Log error: ", err)
		return fmt.Errorf("raft: Initialization error: %s", err)
	}

	// Update the term to the last term in the log.
	_, s.currentTerm = s.log.lastInfo()

	s.setState(Follower)

	// If no log entries exist then
	// 1. wait for AEs from another node
	// 2. wait for self-join command
	// to set itself promotable
	if !s.promotable() {
		s.debugln("start as a new raft server")

		// If log entries exist then allow promotion to candidate
		// if no AEs received.
	} else {
		s.debugln("start from previous saved state")
	}

	debugln(s.GetState())

	go s.loop()

	return nil
}

// Shuts down the server.
func (s *Server) Stop() {
	s.send(&stopValue)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.log.close()
}

// Checks if the server is currently running.
func (s *Server) Running() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state != Stopped
}

//--------------------------------------
// Term
//--------------------------------------

// Sets the current term for the server. This is only used when an external
// current term is found.
func (s *Server) setCurrentTerm(term uint64, leaderName string, append bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// update the term and clear vote for
	if term > s.currentTerm {
		s.state = Follower
		s.currentTerm = term
		s.leader = leaderName
		s.votedFor = ""
		return
	}

	// discover new leader when candidate
	// save leader name when follower
	if term == s.currentTerm && s.state != Leader && append {
		s.state = Follower
		s.leader = leaderName
	}

}

//--------------------------------------
// Event Loop
//--------------------------------------

//               ________
//            --|Snapshot|                 timeout
//            |  --------                  ______
// recover    |       ^                   |      |
// snapshot / |       |snapshot           |      |
// higher     |       |                   v      |     recv majority votes
// term       |    --------    timeout    -----------                        -----------
//            |-> |Follower| ----------> | Candidate |--------------------> |  Leader   |
//                 --------               -----------                        -----------
//                    ^          higher term/ |                         higher term |
//                    |            new leader |                                     |
//                    |_______________________|____________________________________ |
// The main event loop for the server
func (s *Server) loop() {
	defer s.debugln("server.loop.end")

	for {
		state := s.State()

		s.debugln("server.loop.run ", state)
		switch state {
		case Follower:
			s.followerLoop()

		case Candidate:
			s.candidateLoop()

		case Leader:
			s.leaderLoop()

		case Snapshotting:
			s.snapshotLoop()

		case Stopped:
			return
		}
	}
}

// Sends an event to the event loop to be processed. The function will wait
// until the event is actually processed before returning.
func (s *Server) send(value interface{}) (interface{}, error) {
	event := s.sendAsync(value)
	err := <-event.c
	return event.returnValue, err
}

func (s *Server) sendAsync(value interface{}) *event {
	event := &event{target: value, c: make(chan error, 1)}
	s.c <- event
	return event
}

// The event loop that is run when the server is in a Follower state.
// Responds to RPCs from candidates and leaders.
// Converts to candidate if election timeout elapses without either:
//   1.Receiving valid AppendEntries RPC, or
//   2.Granting vote to candidate
func (s *Server) followerLoop() {

	s.setState(Follower)
	timeoutChan := afterBetween(s.ElectionTimeout(), s.ElectionTimeout()*2)

	for {
		var err error
		update := false
		select {
		case e := <-s.c:
			if e.target == &stopValue {
				s.setState(Stopped)
			} else {
				switch req := e.target.(type) {
				case JoinCommand:
					//If no log entries exist and a self-join command is issued
					//then immediately become leader and commit entry.
					if s.log.currentIndex() == 0 && req.NodeName() == s.Name() {
						s.debugln("selfjoin and promote to leader")
						s.setState(Leader)
						s.processCommand(req, e)
					} else {
						err = NotLeaderError
					}
				case *AppendEntriesRequest:
					e.returnValue, update = s.processAppendEntriesRequest(req)
				case *RequestVoteRequest:
					e.returnValue, update = s.processRequestVoteRequest(req)
				case *SnapshotRequest:
					e.returnValue = s.processSnapshotRequest(req)
				default:
					err = NotLeaderError
				}
			}

			// Callback to event.
			e.c <- err

		case <-timeoutChan:

			// only allow synced follower to promote to candidate
			if s.promotable() {
				s.setState(Candidate)
			} else {
				update = true
			}
		}

		// Converts to candidate if election timeout elapses without either:
		//   1.Receiving valid AppendEntries RPC, or
		//   2.Granting vote to candidate
		if update {
			timeoutChan = afterBetween(s.ElectionTimeout(), s.ElectionTimeout()*2)
		}

		// Exit loop on state change.
		if s.State() != Follower {
			break
		}
	}
}

// The event loop that is run when the server is in a Candidate state.
func (s *Server) candidateLoop() {
	lastLogIndex, lastLogTerm := s.log.lastInfo()
	s.leader = ""

	for {
		// Increment current term, vote for self.
		s.currentTerm++
		s.votedFor = s.name

		// Send RequestVote RPCs to all other servers.
		respChan := make(chan *RequestVoteResponse, len(s.peers))
		for _, peer := range s.peers {
			go peer.sendVoteRequest(newRequestVoteRequest(s.currentTerm, s.name, lastLogIndex, lastLogTerm), respChan)
		}

		// Wait for either:
		//   * Votes received from majority of servers: become leader
		//   * AppendEntries RPC received from new leader: step down.
		//   * Election timeout elapses without election resolution: increment term, start new election
		//   * Discover higher term: step down (§5.1)
		votesGranted := 1
		timeoutChan := afterBetween(s.ElectionTimeout(), s.ElectionTimeout()*2)
		timeout := false

		for {
			// If we received enough votes then stop waiting for more votes.
			s.debugln("server.candidate.votes: ", votesGranted, " quorum:", s.QuorumSize())
			if votesGranted >= s.QuorumSize() {
				s.setState(Leader)
				break
			}

			// Collect votes from peers.
			select {
			case resp := <-respChan:
				if resp.VoteGranted {
					s.debugln("server.candidate.vote.granted: ", votesGranted)
					votesGranted++
				} else if resp.Term > s.currentTerm {
					s.debugln("server.candidate.vote.failed")
					s.setCurrentTerm(resp.Term, "", false)
				} else {
					s.debugln("server.candidate.vote: denied")
				}

			case e := <-s.c:
				var err error
				if e.target == &stopValue {
					s.setState(Stopped)
				} else {
					switch req := e.target.(type) {
					case Command:
						err = NotLeaderError
					case *AppendEntriesRequest:
						e.returnValue, _ = s.processAppendEntriesRequest(req)
					case *RequestVoteRequest:
						e.returnValue, _ = s.processRequestVoteRequest(req)
					}
				}
				// Callback to event.
				e.c <- err

			case <-timeoutChan:
				timeout = true
			}

			// both process AER and RVR can make the server to follower
			// also break when timeout happens
			if s.State() != Candidate || timeout {
				break
			}
		}

		// break when we are not candidate
		if s.State() != Candidate {
			break
		}

		// continue when timeout happened
	}
}

// The event loop that is run when the server is in a Leader state.
func (s *Server) leaderLoop() {
	s.setState(Leader)
	s.syncedPeer = make(map[string]bool)
	logIndex, _ := s.log.lastInfo()

	// Update the peers prevLogIndex to leader's lastLogIndex and start heartbeat.
	s.debugln("leaderLoop.set.PrevIndex to ", logIndex)
	for _, peer := range s.peers {
		peer.setPrevLogIndex(logIndex)
		peer.startHeartbeat()
	}

	go s.Do(NOPCommand{})

	// Begin to collect response from followers
	for {
		var err error
		select {
		case e := <-s.c:
			if e.target == &stopValue {
				s.setState(Stopped)
			} else {
				switch req := e.target.(type) {
				case Command:
					s.processCommand(req, e)
					continue
				case *AppendEntriesRequest:
					e.returnValue, _ = s.processAppendEntriesRequest(req)
				case *AppendEntriesResponse:
					s.processAppendEntriesResponse(req)
				case *RequestVoteRequest:
					e.returnValue, _ = s.processRequestVoteRequest(req)
				}
			}

			// Callback to event.
			e.c <- err
		}

		// Exit loop on state change.
		if s.State() != Leader {
			break
		}
	}

	// Stop all peers.
	for _, peer := range s.peers {
		peer.stopHeartbeat(false)
	}
	s.syncedPeer = nil
}

func (s *Server) snapshotLoop() {
	s.setState(Snapshotting)

	for {
		var err error

		e := <-s.c

		if e.target == &stopValue {
			s.setState(Stopped)
		} else {
			switch req := e.target.(type) {
			case Command:
				err = NotLeaderError
			case *AppendEntriesRequest:
				e.returnValue, _ = s.processAppendEntriesRequest(req)
			case *RequestVoteRequest:
				e.returnValue, _ = s.processRequestVoteRequest(req)
			case *SnapshotRecoveryRequest:
				e.returnValue = s.processSnapshotRecoveryRequest(req)
			}
		}
		// Callback to event.
		e.c <- err

		// Exit loop on state change.
		if s.State() != Snapshotting {
			break
		}
	}
}

//--------------------------------------
// Commands
//--------------------------------------

// Attempts to execute a command and replicate it. The function will return
// when the command has been successfully committed or an error has occurred.

func (s *Server) Do(command Command) (interface{}, error) {
	return s.send(command)
}

// Processes a command.
func (s *Server) processCommand(command Command, e *event) {
	s.debugln("server.command.process")

	// Create an entry for the command in the log.
	entry, err := s.log.createEntry(s.currentTerm, command)

	if err != nil {
		s.debugln("server.command.log.entry.error:", err)
		e.c <- err
		return
	}

	if err := s.log.appendEntry(entry); err != nil {
		s.debugln("server.command.log.error:", err)
		e.c <- err
		return
	}

	// Issue a callback for the entry once it's committed.
	go func() {
		// Wait for the entry to be committed.
		select {
		case <-entry.commit:
			var err error
			s.debugln("server.command.commit")
			e.returnValue, err = s.log.getEntryResult(entry, true)
			e.c <- err
		case <-time.After(time.Second):
			s.debugln("server.command.timeout")
			e.c <- CommandTimeoutError
		}
	}()

	// Issue an append entries response for the server.
	resp := newAppendEntriesResponse(s.currentTerm, true, s.log.currentIndex(), s.log.CommitIndex())
	resp.append = true
	resp.peer = s.Name()

	// this must be async
	// sendAsync is not really async every time
	// when the sending speed of the user is larger than
	// the processing speed of the server, the buffered channel
	// will be full. Then sendAsync will become sync, which will
	// cause deadlock here.
	// so we use a goroutine to avoid the deadlock
	go s.sendAsync(resp)
}

//--------------------------------------
// Append Entries
//--------------------------------------

// Appends zero or more log entry from the leader to this server.
func (s *Server) AppendEntries(req *AppendEntriesRequest) *AppendEntriesResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*AppendEntriesResponse)
	return resp
}

// Processes the "append entries" request.
func (s *Server) processAppendEntriesRequest(req *AppendEntriesRequest) (*AppendEntriesResponse, bool) {

	s.traceln("server.ae.process")

	if req.Term < s.currentTerm {
		s.debugln("server.ae.error: stale term")
		return newAppendEntriesResponse(s.currentTerm, false, s.log.currentIndex(), s.log.CommitIndex()), false
	}

	// Update term and leader.
	s.setCurrentTerm(req.Term, req.LeaderName, true)

	// Reject if log doesn't contain a matching previous entry.
	if err := s.log.truncate(req.PrevLogIndex, req.PrevLogTerm); err != nil {
		s.debugln("server.ae.truncate.error: ", err)
		return newAppendEntriesResponse(s.currentTerm, false, s.log.currentIndex(), s.log.CommitIndex()), true
	}

	// Append entries to the log.
	if err := s.log.appendEntries(req.Entries); err != nil {
		s.debugln("server.ae.append.error: ", err)
		return newAppendEntriesResponse(s.currentTerm, false, s.log.currentIndex(), s.log.CommitIndex()), true
	}

	// Commit up to the commit index.
	if err := s.log.setCommitIndex(req.CommitIndex); err != nil {
		s.debugln("server.ae.commit.error: ", err)
		return newAppendEntriesResponse(s.currentTerm, false, s.log.currentIndex(), s.log.CommitIndex()), true
	}

	// once the server appended and commited all the log entries from the leader

	return newAppendEntriesResponse(s.currentTerm, true, s.log.currentIndex(), s.log.CommitIndex()), true
}

// Processes the "append entries" response from the peer. This is only
// processed when the server is a leader. Responses received during other
// states are dropped.
func (s *Server) processAppendEntriesResponse(resp *AppendEntriesResponse) {

	// If we find a higher term then change to a follower and exit.
	if resp.Term > s.currentTerm {
		s.setCurrentTerm(resp.Term, "", false)
		return
	}

	// panic response if it's not successful.
	if !resp.Success {
		return
	}

	// if one peer successfully append a log from the leader term,
	// we add it to the synced list
	if resp.append == true {
		s.syncedPeer[resp.peer] = true
	}

	// Increment the commit count to make sure we have a quorum before committing.
	if len(s.syncedPeer) < s.QuorumSize() {
		return
	}

	// Determine the committed index that a majority has.
	var indices []uint64
	indices = append(indices, s.log.currentIndex())
	for _, peer := range s.peers {
		indices = append(indices, peer.getPrevLogIndex())
	}
	sort.Sort(uint64Slice(indices))

	// We can commit up to the index which the majority of the members have appended.
	commitIndex := indices[s.QuorumSize()-1]
	committedIndex := s.log.commitIndex

	if commitIndex > committedIndex {
		s.log.setCommitIndex(commitIndex)
		s.debugln("commit index ", commitIndex)
		for i := committedIndex; i < commitIndex; i++ {
			if entry := s.log.getEntry(i + 1); entry != nil {
				// if the leader is a new one and the entry came from the
				// old leader, the commit channel will be nil and no go routine
				// is waiting from this channel
				// if we try to send to it, the new leader will get stuck
				if entry.commit != nil {
					select {
					case entry.commit <- true:
					default:
						panic("server unable to send signal to commit channel")
					}
				}
			}
		}
	}
}

//--------------------------------------
// Request Vote
//--------------------------------------

// Requests a vote from a server. A vote can be obtained if the vote's term is
// at the server's current term and the server has not made a vote yet. A vote
// can also be obtained if the term is greater than the server's current term.
func (s *Server) RequestVote(req *RequestVoteRequest) *RequestVoteResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*RequestVoteResponse)
	return resp
}

// Processes a "request vote" request.
func (s *Server) processRequestVoteRequest(req *RequestVoteRequest) (*RequestVoteResponse, bool) {

	// If the request is coming from an old term then reject it.
	if req.Term < s.currentTerm {
		s.debugln("server.rv.error: stale term")
		return newRequestVoteResponse(s.currentTerm, false), false
	}

	s.setCurrentTerm(req.Term, "", false)

	// If we've already voted for a different candidate then don't vote for this candidate.
	if s.votedFor != "" && s.votedFor != req.CandidateName {
		s.debugln("server.rv.error: duplicate vote: ", req.CandidateName,
			" already vote for ", s.votedFor)
		return newRequestVoteResponse(s.currentTerm, false), false
	}

	// If the candidate's log is not at least as up-to-date as our last log then don't vote.
	lastIndex, lastTerm := s.log.lastInfo()
	if lastIndex > req.LastLogIndex || lastTerm > req.LastLogTerm {
		s.debugln("server.rv.error: out of date log: ", req.CandidateName,
			"Index :[", lastIndex, "]", " [", req.LastLogIndex, "]",
			"Term :[", lastTerm, "]", " [", req.LastLogTerm, "]")
		return newRequestVoteResponse(s.currentTerm, false), false
	}

	// If we made it this far then cast a vote and reset our election time out.
	s.debugln("server.rv.vote: ", s.name, " votes for", req.CandidateName, "at term", req.Term)
	s.votedFor = req.CandidateName

	return newRequestVoteResponse(s.currentTerm, true), true
}

//--------------------------------------
// Membership
//--------------------------------------

// Adds a peer to the server.
func (s *Server) AddPeer(name string, connectiongString string) error {
	s.debugln("server.peer.add: ", name, len(s.peers))
	defer s.writeConf()
	// Do not allow peers to be added twice.
	if s.peers[name] != nil {
		return nil
	}

	// Skip the Peer if it has the same name as the Server
	if s.name == name {
		return nil
	}

	peer := newPeer(s, name, connectiongString, s.heartbeatTimeout)

	if s.State() == Leader {
		peer.startHeartbeat()
	}

	s.peers[peer.Name] = peer

	s.debugln("server.peer.conf.write: ", name)

	return nil
}

// Removes a peer from the server.
func (s *Server) RemovePeer(name string) error {
	s.debugln("server.peer.remove: ", name, len(s.peers))

	defer s.writeConf()

	if name == s.Name() {
		// when the removed node restart, it should be able
		// to know it has been removed before. So we need
		// to update knownCommitIndex
		return nil
	}
	// Return error if peer doesn't exist.
	peer := s.peers[name]
	if peer == nil {
		return fmt.Errorf("raft: Peer not found: %s", name)
	}

	// Stop peer and remove it.
	if s.State() == Leader {
		peer.stopHeartbeat(true)
	}

	delete(s.peers, name)

	return nil
}

//--------------------------------------
// Log compaction
//--------------------------------------

func (s *Server) TakeSnapshot() error {
	//TODO put a snapshot mutex
	s.debugln("take Snapshot")
	if s.currentSnapshot != nil {
		return errors.New("handling snapshot")
	}

	lastIndex, lastTerm := s.log.commitInfo()

	if lastIndex == 0 {
		return errors.New("No logs")
	}

	path := s.SnapshotPath(lastIndex, lastTerm)

	var state []byte
	var err error

	if s.stateMachine != nil {
		state, err = s.stateMachine.Save()

		if err != nil {
			return err
		}

	} else {
		state = []byte{0}
	}

	var peers []*Peer

	for _, peer := range s.peers {
		peers = append(peers, peer.clone())
	}

	s.currentSnapshot = &Snapshot{lastIndex, lastTerm, peers, state, path}

	s.saveSnapshot()

	// We keep some log entries after the snapshot
	// We do not want to send the whole snapshot
	// to the slightly slow machines
	if lastIndex-s.log.startIndex > NumberOfLogEntriesAfterSnapshot {
		compactIndex := lastIndex - NumberOfLogEntriesAfterSnapshot
		compactTerm := s.log.getEntry(compactIndex).Term
		s.log.compact(compactIndex, compactTerm)
	}

	return nil
}

// Retrieves the log path for the server.
func (s *Server) saveSnapshot() error {

	if s.currentSnapshot == nil {
		return errors.New("no snapshot to save")
	}

	err := s.currentSnapshot.save()

	if err != nil {
		return err
	}

	tmp := s.lastSnapshot
	s.lastSnapshot = s.currentSnapshot

	// delete the previous snapshot if there is any change
	if tmp != nil && !(tmp.LastIndex == s.lastSnapshot.LastIndex && tmp.LastTerm == s.lastSnapshot.LastTerm) {
		tmp.remove()
	}
	s.currentSnapshot = nil
	return nil
}

// Retrieves the log path for the server.
func (s *Server) SnapshotPath(lastIndex uint64, lastTerm uint64) string {
	return path.Join(s.path, "snapshot", fmt.Sprintf("%v_%v.ss", lastTerm, lastIndex))
}

func (s *Server) RequestSnapshot(req *SnapshotRequest) *SnapshotResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*SnapshotResponse)
	return resp
}

func (s *Server) processSnapshotRequest(req *SnapshotRequest) *SnapshotResponse {

	// If the follower’s log contains an entry at the snapshot’s last index with a term
	// that matches the snapshot’s last term
	// Then the follower already has all the information found in the snapshot
	// and can reply false

	entry := s.log.getEntry(req.LastIndex)

	if entry != nil && entry.Term == req.LastTerm {
		return newSnapshotResponse(false)
	}

	s.setState(Snapshotting)

	return newSnapshotResponse(true)
}

func (s *Server) SnapshotRecoveryRequest(req *SnapshotRecoveryRequest) *SnapshotRecoveryResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*SnapshotRecoveryResponse)
	return resp
}

func (s *Server) processSnapshotRecoveryRequest(req *SnapshotRecoveryRequest) *SnapshotRecoveryResponse {

	s.stateMachine.Recovery(req.State)

	// clear the peer map
	s.peers = make(map[string]*Peer)

	// recovery the cluster configuration
	for _, peer := range req.Peers {
		s.AddPeer(peer.Name, peer.ConnectionString)
	}

	//update term and index
	s.currentTerm = req.LastTerm

	s.log.updateCommitIndex(req.LastIndex)

	snapshotPath := s.SnapshotPath(req.LastIndex, req.LastTerm)

	s.currentSnapshot = &Snapshot{req.LastIndex, req.LastTerm, req.Peers, req.State, snapshotPath}

	s.saveSnapshot()

	// clear the previous log entries
	s.log.compact(req.LastIndex, req.LastTerm)

	return newSnapshotRecoveryResponse(req.LastTerm, true, req.LastIndex)

}

// Load a snapshot at restart
func (s *Server) LoadSnapshot() error {
	dir, err := os.OpenFile(path.Join(s.path, "snapshot"), os.O_RDONLY, 0)
	if err != nil {

		return err
	}

	filenames, err := dir.Readdirnames(-1)

	if err != nil {
		dir.Close()
		panic(err)
	}

	dir.Close()
	if len(filenames) == 0 {
		return errors.New("no snapshot")
	}

	// not sure how many snapshot we should keep
	sort.Strings(filenames)
	snapshotPath := path.Join(s.path, "snapshot", filenames[len(filenames)-1])

	// should not fail
	file, err := os.OpenFile(snapshotPath, os.O_RDONLY, 0)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	// TODO check checksum first

	var snapshotBytes []byte
	var checksum uint32

	n, err := fmt.Fscanf(file, "%08x\n", &checksum)

	if err != nil {
		return err
	}

	if n != 1 {
		return errors.New("Bad snapshot file")
	}

	snapshotBytes, _ = ioutil.ReadAll(file)
	s.debugln(string(snapshotBytes))

	// Generate checksum.
	byteChecksum := crc32.ChecksumIEEE(snapshotBytes)

	if uint32(checksum) != byteChecksum {
		s.debugln(checksum, " ", byteChecksum)
		return errors.New("bad snapshot file")
	}

	err = json.Unmarshal(snapshotBytes, &s.lastSnapshot)

	if err != nil {
		s.debugln("unmarshal error: ", err)
		return err
	}

	err = s.stateMachine.Recovery(s.lastSnapshot.State)

	if err != nil {
		s.debugln("recovery error: ", err)
		return err
	}

	for _, peer := range s.lastSnapshot.Peers {
		s.AddPeer(peer.Name, peer.ConnectionString)
	}

	s.log.startTerm = s.lastSnapshot.LastTerm
	s.log.startIndex = s.lastSnapshot.LastIndex
	s.log.updateCommitIndex(s.lastSnapshot.LastIndex)

	return err
}

//--------------------------------------
// Config File
//--------------------------------------

func (s *Server) writeConf() {

	peers := make([]*Peer, len(s.peers))

	i := 0
	for _, peer := range s.peers {
		peers[i] = peer.clone()
		i++
	}

	r := &Config{
		CommitIndex: s.log.commitIndex,
		Peers:       peers,
	}

	b, _ := json.Marshal(r)

	confPath := path.Join(s.path, "conf")
	tmpConfPath := path.Join(s.path, "conf.tmp")

	err := ioutil.WriteFile(tmpConfPath, b, 0600)

	if err != nil {
		panic(err)
	}

	os.Rename(tmpConfPath, confPath)
}

// Read the configuration for the server.
func (s *Server) readConf() error {
	confPath := path.Join(s.path, "conf")
	s.debugln("readConf.open ", confPath)

	// open conf file
	b, err := ioutil.ReadFile(confPath)

	if err != nil {
		return nil
	}

	conf := &Config{}

	if err = json.Unmarshal(b, conf); err != nil {
		return err
	}

	s.log.commitIndex = conf.CommitIndex

	return nil
}

//--------------------------------------
// Debugging
//--------------------------------------

func (s *Server) debugln(v ...interface{}) {
	debugf("[%s Term:%d] %s", s.name, s.currentTerm, fmt.Sprintln(v...))
}

func (s *Server) traceln(v ...interface{}) {
	tracef("[%s] %s", s.name, fmt.Sprintln(v...))
}
