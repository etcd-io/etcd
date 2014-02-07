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
	// DefaultHeartbeatInterval is the interval that the leader will send
	// AppendEntriesRequests to followers to maintain leadership.
	DefaultHeartbeatInterval = 50 * time.Millisecond

	DefaultElectionTimeout = 150 * time.Millisecond
)

// ElectionTimeoutThresholdPercent specifies the threshold at which the server
// will dispatch warning events that the heartbeat RTT is too close to the
// election timeout.
const ElectionTimeoutThresholdPercent = 0.8

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
type Server interface {
	Name() string
	Context() interface{}
	StateMachine() StateMachine
	Leader() string
	State() string
	Path() string
	LogPath() string
	SnapshotPath(lastIndex uint64, lastTerm uint64) string
	Term() uint64
	CommitIndex() uint64
	VotedFor() string
	MemberCount() int
	QuorumSize() int
	IsLogEmpty() bool
	LogEntries() []*LogEntry
	LastCommandName() string
	GetState() string
	ElectionTimeout() time.Duration
	SetElectionTimeout(duration time.Duration)
	HeartbeatInterval() time.Duration
	SetHeartbeatInterval(duration time.Duration)
	Transporter() Transporter
	SetTransporter(t Transporter)
	AppendEntries(req *AppendEntriesRequest) *AppendEntriesResponse
	RequestVote(req *RequestVoteRequest) *RequestVoteResponse
	RequestSnapshot(req *SnapshotRequest) *SnapshotResponse
	SnapshotRecoveryRequest(req *SnapshotRecoveryRequest) *SnapshotRecoveryResponse
	AddPeer(name string, connectiongString string) error
	RemovePeer(name string) error
	Peers() map[string]*Peer
	Start() error
	Stop()
	Running() bool
	Do(command Command) (interface{}, error)
	TakeSnapshot() error
	LoadSnapshot() error
	AddEventListener(string, EventListener)
}

type server struct {
	*eventDispatcher

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

	stopped           chan bool
	c                 chan *ev
	electionTimeout   time.Duration
	heartbeatInterval time.Duration

	currentSnapshot         *Snapshot
	lastSnapshot            *Snapshot
	stateMachine            StateMachine
	maxLogEntriesPerRequest uint64

	connectionString string
}

// An internal event to be processed by the server's event loop.
type ev struct {
	target      interface{}
	returnValue interface{}
	c           chan error
}

//------------------------------------------------------------------------------
//
// Constructor
//
//------------------------------------------------------------------------------

// Creates a new server with a log at the given path. transporter must
// not be nil. stateMachine can be nil if snapshotting and log
// compaction is to be disabled. context can be anything (including nil)
// and is not used by the raft package except returned by
// Server.Context(). connectionString can be anything.
func NewServer(name string, path string, transporter Transporter, stateMachine StateMachine, ctx interface{}, connectionString string) (Server, error) {
	if name == "" {
		return nil, errors.New("raft.Server: Name cannot be blank")
	}
	if transporter == nil {
		panic("raft: Transporter required")
	}

	s := &server{
		name:                    name,
		path:                    path,
		transporter:             transporter,
		stateMachine:            stateMachine,
		context:                 ctx,
		state:                   Stopped,
		peers:                   make(map[string]*Peer),
		log:                     newLog(),
		stopped:                 make(chan bool),
		c:                       make(chan *ev, 256),
		electionTimeout:         DefaultElectionTimeout,
		heartbeatInterval:       DefaultHeartbeatInterval,
		maxLogEntriesPerRequest: MaxLogEntriesPerRequest,
		connectionString:        connectionString,
	}
	s.eventDispatcher = newEventDispatcher(s)

	// Setup apply function.
	s.log.ApplyFunc = func(c Command) (interface{}, error) {
		switch c := c.(type) {
		case CommandApply:
			return c.Apply(&context{
				server:       s,
				currentTerm:  s.currentTerm,
				currentIndex: s.log.internalCurrentIndex(),
				commitIndex:  s.log.commitIndex,
			})
		case deprecatedCommandApply:
			return c.Apply(s)
		default:
			return nil, fmt.Errorf("Command does not implement Apply()")
		}
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
func (s *server) Name() string {
	return s.name
}

// Retrieves the storage path for the server.
func (s *server) Path() string {
	return s.path
}

// The name of the current leader.
func (s *server) Leader() string {
	return s.leader
}

// Retrieves a copy of the peer data.
func (s *server) Peers() map[string]*Peer {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	peers := make(map[string]*Peer)
	for name, peer := range s.peers {
		peers[name] = peer.clone()
	}
	return peers
}

// Retrieves the object that transports requests.
func (s *server) Transporter() Transporter {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.transporter
}

func (s *server) SetTransporter(t Transporter) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.transporter = t
}

// Retrieves the context passed into the constructor.
func (s *server) Context() interface{} {
	return s.context
}

// Retrieves the state machine passed into the constructor.
func (s *server) StateMachine() StateMachine {
	return s.stateMachine
}

// Retrieves the log path for the server.
func (s *server) LogPath() string {
	return path.Join(s.path, "log")
}

// Retrieves the current state of the server.
func (s *server) State() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state
}

// Sets the state of the server.
func (s *server) setState(state string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Temporarily store previous values.
	prevState := s.state
	prevLeader := s.leader

	// Update state and leader.
	s.state = state
	if state == Leader {
		s.leader = s.Name()
		s.syncedPeer = make(map[string]bool)
	}

	// Dispatch state and leader change events.
	if prevState != state {
		s.DispatchEvent(newEvent(StateChangeEventType, s.state, prevState))
	}
	if prevLeader != s.leader {
		s.DispatchEvent(newEvent(LeaderChangeEventType, s.leader, prevLeader))
	}
}

// Retrieves the current term of the server.
func (s *server) Term() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.currentTerm
}

// Retrieves the current commit index of the server.
func (s *server) CommitIndex() uint64 {
	s.log.mutex.RLock()
	defer s.log.mutex.RUnlock()
	return s.log.commitIndex
}

// Retrieves the name of the candidate this server voted for in this term.
func (s *server) VotedFor() string {
	return s.votedFor
}

// Retrieves whether the server's log has no entries.
func (s *server) IsLogEmpty() bool {
	return s.log.isEmpty()
}

// A list of all the log entries. This should only be used for debugging purposes.
func (s *server) LogEntries() []*LogEntry {
	return s.log.entries
}

// A reference to the command name of the last entry.
func (s *server) LastCommandName() string {
	return s.log.lastCommandName()
}

// Get the state of the server for debugging
func (s *server) GetState() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return fmt.Sprintf("Name: %s, State: %s, Term: %v, CommitedIndex: %v ", s.name, s.state, s.currentTerm, s.log.commitIndex)
}

// Check if the server is promotable
func (s *server) promotable() bool {
	return s.log.currentIndex() > 0
}

//--------------------------------------
// Membership
//--------------------------------------

// Retrieves the number of member servers in the consensus.
func (s *server) MemberCount() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.peers) + 1
}

// Retrieves the number of servers required to make a quorum.
func (s *server) QuorumSize() int {
	return (s.MemberCount() / 2) + 1
}

//--------------------------------------
// Election timeout
//--------------------------------------

// Retrieves the election timeout.
func (s *server) ElectionTimeout() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.electionTimeout
}

// Sets the election timeout.
func (s *server) SetElectionTimeout(duration time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.electionTimeout = duration
}

//--------------------------------------
// Heartbeat timeout
//--------------------------------------

// Retrieves the heartbeat timeout.
func (s *server) HeartbeatInterval() time.Duration {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.heartbeatInterval
}

// Sets the heartbeat timeout.
func (s *server) SetHeartbeatInterval(duration time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.heartbeatInterval = duration
	for _, peer := range s.peers {
		peer.setHeartbeatInterval(duration)
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

func (s *server) Start() error {
	// Exit if the server is already running.
	if s.State() != Stopped {
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
func (s *server) Stop() {
	s.send(&stopValue)

	// make sure the server has stopped before we close the log
	<-s.stopped
	s.log.close()
}

// Checks if the server is currently running.
func (s *server) Running() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state != Stopped
}

//--------------------------------------
// Term
//--------------------------------------

// Sets the current term for the server. This is only used when an external
// current term is found.
func (s *server) setCurrentTerm(term uint64, leaderName string, append bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Store previous values temporarily.
	prevState := s.state
	prevTerm := s.currentTerm
	prevLeader := s.leader

	if term > s.currentTerm {
		// stop heartbeats before step-down
		if s.state == Leader {
			s.mutex.Unlock()
			for _, peer := range s.peers {
				peer.stopHeartbeat(false)
			}
			s.mutex.Lock()
		}
		// update the term and clear vote for
		s.state = Follower
		s.currentTerm = term
		s.leader = leaderName
		s.votedFor = ""
	} else if term == s.currentTerm && s.state != Leader && append {
		// discover new leader when candidate
		// save leader name when follower
		s.state = Follower
		s.leader = leaderName
	}

	// Dispatch change events.
	if prevState != s.state {
		s.DispatchEvent(newEvent(StateChangeEventType, s.state, prevState))
	}
	if prevLeader != s.leader {
		s.DispatchEvent(newEvent(LeaderChangeEventType, s.leader, prevLeader))
	}
	if prevTerm != s.currentTerm {
		s.DispatchEvent(newEvent(TermChangeEventType, s.currentTerm, prevTerm))
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
func (s *server) loop() {
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
			s.stopped <- true
			return
		}
	}
}

// Sends an event to the event loop to be processed. The function will wait
// until the event is actually processed before returning.
func (s *server) send(value interface{}) (interface{}, error) {
	event := &ev{target: value, c: make(chan error, 1)}
	s.c <- event
	err := <-event.c
	return event.returnValue, err
}

func (s *server) sendAsync(value interface{}) {
	event := &ev{target: value, c: make(chan error, 1)}
	// try a non-blocking send first
	// in most cases, this should not be blocking
	// avoid create unnecessary go routines
	select {
	case s.c <- event:
		return
	default:
	}

	go func() {
		s.c <- event
	}()
}

// The event loop that is run when the server is in a Follower state.
// Responds to RPCs from candidates and leaders.
// Converts to candidate if election timeout elapses without either:
//   1.Receiving valid AppendEntries RPC, or
//   2.Granting vote to candidate
func (s *server) followerLoop() {
	s.setState(Follower)
	since := time.Now()
	electionTimeout := s.ElectionTimeout()
	timeoutChan := afterBetween(s.ElectionTimeout(), s.ElectionTimeout()*2)

	for s.State() == Follower {
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
					// If heartbeats get too close to the election timeout then send an event.
					elapsedTime := time.Now().Sub(since)
					if elapsedTime > time.Duration(float64(electionTimeout)*ElectionTimeoutThresholdPercent) {
						s.DispatchEvent(newEvent(ElectionTimeoutThresholdEventType, elapsedTime, nil))
					}
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
			since = time.Now()
			timeoutChan = afterBetween(s.ElectionTimeout(), s.ElectionTimeout()*2)
		}
	}
}

// The event loop that is run when the server is in a Candidate state.
func (s *server) candidateLoop() {
	lastLogIndex, lastLogTerm := s.log.lastInfo()

	// Clear leader value.
	prevLeader := s.leader
	s.leader = ""
	if prevLeader != s.leader {
		s.DispatchEvent(newEvent(LeaderChangeEventType, s.leader, prevLeader))
	}

	for s.State() == Candidate {
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
		// continue when timeout happened
	}
}

// The event loop that is run when the server is in a Leader state.
func (s *server) leaderLoop() {
	s.setState(Leader)
	logIndex, _ := s.log.lastInfo()

	// Update the peers prevLogIndex to leader's lastLogIndex and start heartbeat.
	s.debugln("leaderLoop.set.PrevIndex to ", logIndex)
	for _, peer := range s.peers {
		peer.setPrevLogIndex(logIndex)
		peer.startHeartbeat()
	}

	// Commit a NOP after the server becomes leader. From the Raft paper:
	// "Upon election: send initial empty AppendEntries RPCs (heartbeat) to
	// each server; repeat during idle periods to prevent election timeouts
	// (§5.2)". The heartbeats started above do the "idle" period work.
	go s.Do(NOPCommand{})

	// Begin to collect response from followers
	for s.State() == Leader {
		var err error
		select {
		case e := <-s.c:
			if e.target == &stopValue {
				// Stop all peers before stop
				for _, peer := range s.peers {
					peer.stopHeartbeat(false)
				}
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
	}

	s.syncedPeer = nil
}

func (s *server) snapshotLoop() {
	s.setState(Snapshotting)

	for s.State() == Snapshotting {
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
	}
}

//--------------------------------------
// Commands
//--------------------------------------

// Attempts to execute a command and replicate it. The function will return
// when the command has been successfully committed or an error has occurred.

func (s *server) Do(command Command) (interface{}, error) {
	return s.send(command)
}

// Processes a command.
func (s *server) processCommand(command Command, e *ev) {
	s.debugln("server.command.process")

	// Create an entry for the command in the log.
	entry, err := s.log.createEntry(s.currentTerm, command, e)

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

	s.syncedPeer[s.Name()] = true
	if len(s.peers) == 0 {
		commitIndex := s.log.currentIndex()
		s.log.setCommitIndex(commitIndex)
		s.debugln("commit index ", commitIndex)
	}
}

//--------------------------------------
// Append Entries
//--------------------------------------

// Appends zero or more log entry from the leader to this server.
func (s *server) AppendEntries(req *AppendEntriesRequest) *AppendEntriesResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*AppendEntriesResponse)
	return resp
}

// Processes the "append entries" request.
func (s *server) processAppendEntriesRequest(req *AppendEntriesRequest) (*AppendEntriesResponse, bool) {
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

	// once the server appended and committed all the log entries from the leader

	return newAppendEntriesResponse(s.currentTerm, true, s.log.currentIndex(), s.log.CommitIndex()), true
}

// Processes the "append entries" response from the peer. This is only
// processed when the server is a leader. Responses received during other
// states are dropped.
func (s *server) processAppendEntriesResponse(resp *AppendEntriesResponse) {
	// If we find a higher term then change to a follower and exit.
	if resp.Term() > s.Term() {
		s.setCurrentTerm(resp.Term(), "", false)
		return
	}

	// panic response if it's not successful.
	if !resp.Success() {
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
	sort.Sort(sort.Reverse(uint64Slice(indices)))

	// We can commit up to the index which the majority of the members have appended.
	commitIndex := indices[s.QuorumSize()-1]
	committedIndex := s.log.commitIndex

	if commitIndex > committedIndex {
		// leader needs to do a fsync before committing log entries
		s.log.sync()
		s.log.setCommitIndex(commitIndex)
		s.debugln("commit index ", commitIndex)
	}
}

//--------------------------------------
// Request Vote
//--------------------------------------

// Requests a vote from a server. A vote can be obtained if the vote's term is
// at the server's current term and the server has not made a vote yet. A vote
// can also be obtained if the term is greater than the server's current term.
func (s *server) RequestVote(req *RequestVoteRequest) *RequestVoteResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*RequestVoteResponse)
	return resp
}

// Processes a "request vote" request.
func (s *server) processRequestVoteRequest(req *RequestVoteRequest) (*RequestVoteResponse, bool) {

	// If the request is coming from an old term then reject it.
	if req.Term < s.Term() {
		s.debugln("server.rv.deny.vote: cause stale term")
		return newRequestVoteResponse(s.currentTerm, false), false
	}

	s.setCurrentTerm(req.Term, "", false)

	// If we've already voted for a different candidate then don't vote for this candidate.
	if s.votedFor != "" && s.votedFor != req.CandidateName {
		s.debugln("server.deny.vote: cause duplicate vote: ", req.CandidateName,
			" already vote for ", s.votedFor)
		return newRequestVoteResponse(s.currentTerm, false), false
	}

	// If the candidate's log is not at least as up-to-date as our last log then don't vote.
	lastIndex, lastTerm := s.log.lastInfo()
	if lastIndex > req.LastLogIndex || lastTerm > req.LastLogTerm {
		s.debugln("server.deny.vote: cause out of date log: ", req.CandidateName,
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
func (s *server) AddPeer(name string, connectiongString string) error {
	s.debugln("server.peer.add: ", name, len(s.peers))

	// Do not allow peers to be added twice.
	if s.peers[name] != nil {
		return nil
	}

	// Skip the Peer if it has the same name as the Server
	if s.name != name {
		peer := newPeer(s, name, connectiongString, s.heartbeatInterval)

		if s.State() == Leader {
			peer.startHeartbeat()
		}

		s.peers[peer.Name] = peer

		s.DispatchEvent(newEvent(AddPeerEventType, name, nil))
	}

	// Write the configuration to file.
	s.writeConf()

	return nil
}

// Removes a peer from the server.
func (s *server) RemovePeer(name string) error {
	s.debugln("server.peer.remove: ", name, len(s.peers))

	// Skip the Peer if it has the same name as the Server
	if name != s.Name() {
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

		s.DispatchEvent(newEvent(RemovePeerEventType, name, nil))
	}

	// Write the configuration to file.
	s.writeConf()

	return nil
}

//--------------------------------------
// Log compaction
//--------------------------------------

func (s *server) TakeSnapshot() error {
	// TODO: put a snapshot mutex
	s.debugln("take Snapshot")

	// Exit if the server is currently creating a snapshot.
	if s.currentSnapshot != nil {
		return errors.New("handling snapshot")
	}

	// Exit if there are no logs yet in the system.
	lastIndex, lastTerm := s.log.commitInfo()
	path := s.SnapshotPath(lastIndex, lastTerm)
	if lastIndex == 0 {
		return errors.New("No logs")
	}

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

	// Clone the list of peers.
	peers := make([]*Peer, 0, len(s.peers)+1)
	for _, peer := range s.peers {
		peers = append(peers, peer.clone())
	}
	peers = append(peers, &Peer{Name: s.Name(), ConnectionString: s.connectionString})

	// Attach current snapshot and save it to disk.
	s.currentSnapshot = &Snapshot{lastIndex, lastTerm, peers, state, path}
	s.saveSnapshot()

	// We keep some log entries after the snapshot.
	// We do not want to send the whole snapshot to the slightly slow machines
	if lastIndex-s.log.startIndex > NumberOfLogEntriesAfterSnapshot {
		compactIndex := lastIndex - NumberOfLogEntriesAfterSnapshot
		compactTerm := s.log.getEntry(compactIndex).Term()
		s.log.compact(compactIndex, compactTerm)
	}

	return nil
}

// Retrieves the log path for the server.
func (s *server) saveSnapshot() error {
	if s.currentSnapshot == nil {
		return errors.New("no snapshot to save")
	}

	// Write snapshot to disk.
	if err := s.currentSnapshot.save(); err != nil {
		return err
	}

	// Swap the current and last snapshots.
	tmp := s.lastSnapshot
	s.lastSnapshot = s.currentSnapshot

	// Delete the previous snapshot if there is any change
	if tmp != nil && !(tmp.LastIndex == s.lastSnapshot.LastIndex && tmp.LastTerm == s.lastSnapshot.LastTerm) {
		tmp.remove()
	}
	s.currentSnapshot = nil

	return nil
}

// Retrieves the log path for the server.
func (s *server) SnapshotPath(lastIndex uint64, lastTerm uint64) string {
	return path.Join(s.path, "snapshot", fmt.Sprintf("%v_%v.ss", lastTerm, lastIndex))
}

func (s *server) RequestSnapshot(req *SnapshotRequest) *SnapshotResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*SnapshotResponse)
	return resp
}

func (s *server) processSnapshotRequest(req *SnapshotRequest) *SnapshotResponse {
	// If the follower’s log contains an entry at the snapshot’s last index with a term
	// that matches the snapshot’s last term, then the follower already has all the
	// information found in the snapshot and can reply false.
	entry := s.log.getEntry(req.LastIndex)

	if entry != nil && entry.Term() == req.LastTerm {
		return newSnapshotResponse(false)
	}

	// Update state.
	s.setState(Snapshotting)

	return newSnapshotResponse(true)
}

func (s *server) SnapshotRecoveryRequest(req *SnapshotRecoveryRequest) *SnapshotRecoveryResponse {
	ret, _ := s.send(req)
	resp, _ := ret.(*SnapshotRecoveryResponse)
	return resp
}

func (s *server) processSnapshotRecoveryRequest(req *SnapshotRecoveryRequest) *SnapshotRecoveryResponse {
	// Recover state sent from request.
	if err := s.stateMachine.Recovery(req.State); err != nil {
		return newSnapshotRecoveryResponse(req.LastTerm, false, req.LastIndex)
	}

	// Recover the cluster configuration.
	s.peers = make(map[string]*Peer)
	for _, peer := range req.Peers {
		s.AddPeer(peer.Name, peer.ConnectionString)
	}

	// Update log state.
	s.currentTerm = req.LastTerm
	s.log.updateCommitIndex(req.LastIndex)

	// Create local snapshot.
	s.currentSnapshot = &Snapshot{req.LastIndex, req.LastTerm, req.Peers, req.State, s.SnapshotPath(req.LastIndex, req.LastTerm)}
	s.saveSnapshot()

	// Clear the previous log entries.
	s.log.compact(req.LastIndex, req.LastTerm)

	return newSnapshotRecoveryResponse(req.LastTerm, true, req.LastIndex)

}

// Load a snapshot at restart
func (s *server) LoadSnapshot() error {
	// Open snapshot/ directory.
	dir, err := os.OpenFile(path.Join(s.path, "snapshot"), os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	// Retrieve a list of all snapshots.
	filenames, err := dir.Readdirnames(-1)
	if err != nil {
		dir.Close()
		panic(err)
	}
	dir.Close()

	if len(filenames) == 0 {
		return errors.New("no snapshot")
	}

	// Grab the latest snapshot.
	sort.Strings(filenames)
	snapshotPath := path.Join(s.path, "snapshot", filenames[len(filenames)-1])

	// Read snapshot data.
	file, err := os.OpenFile(snapshotPath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check checksum.
	var checksum uint32
	n, err := fmt.Fscanf(file, "%08x\n", &checksum)
	if err != nil {
		return err
	} else if n != 1 {
		return errors.New("Bad snapshot file")
	}

	// Load remaining snapshot contents.
	b, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	// Generate checksum.
	byteChecksum := crc32.ChecksumIEEE(b)
	if uint32(checksum) != byteChecksum {
		s.debugln(checksum, " ", byteChecksum)
		return errors.New("bad snapshot file")
	}

	// Decode snapshot.
	if err = json.Unmarshal(b, &s.lastSnapshot); err != nil {
		s.debugln("unmarshal error: ", err)
		return err
	}

	// Recover snapshot into state machine.
	if err = s.stateMachine.Recovery(s.lastSnapshot.State); err != nil {
		s.debugln("recovery error: ", err)
		return err
	}

	// Recover cluster configuration.
	for _, peer := range s.lastSnapshot.Peers {
		s.AddPeer(peer.Name, peer.ConnectionString)
	}

	// Update log state.
	s.log.startTerm = s.lastSnapshot.LastTerm
	s.log.startIndex = s.lastSnapshot.LastIndex
	s.log.updateCommitIndex(s.lastSnapshot.LastIndex)

	return err
}

//--------------------------------------
// Config File
//--------------------------------------

func (s *server) writeConf() {

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

	err := writeFileSynced(tmpConfPath, b, 0600)

	if err != nil {
		panic(err)
	}

	os.Rename(tmpConfPath, confPath)
}

// Read the configuration for the server.
func (s *server) readConf() error {
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

	s.log.updateCommitIndex(conf.CommitIndex)

	return nil
}

//--------------------------------------
// Debugging
//--------------------------------------

func (s *server) debugln(v ...interface{}) {
	if logLevel > Debug {
		debugf("[%s Term:%d] %s", s.name, s.Term(), fmt.Sprintln(v...))
	}
}

func (s *server) traceln(v ...interface{}) {
	if logLevel > Trace {
		tracef("[%s] %s", s.name, fmt.Sprintln(v...))
	}
}
