package raft

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

//------------------------------------------------------------------------------
//
// Tests
//
//------------------------------------------------------------------------------

//--------------------------------------
// Request Vote
//--------------------------------------

// Ensure that we can request a vote from a server that has not voted.
func TestServerRequestVote(t *testing.T) {
	server := newTestServer("1", &testTransporter{})

	server.Start()
	if _, err := server.Do(&DefaultJoinCommand{Name: server.Name()}); err != nil {
		t.Fatalf("Server %s unable to join: %v", server.Name(), err)
	}

	defer server.Stop()
	resp := server.RequestVote(newRequestVoteRequest(1, "foo", 1, 0))
	if resp.Term != 1 || !resp.VoteGranted {
		t.Fatalf("Invalid request vote response: %v/%v", resp.Term, resp.VoteGranted)
	}
}

// // Ensure that a vote request is denied if it comes from an old term.
func TestServerRequestVoteDeniedForStaleTerm(t *testing.T) {
	server := newTestServer("1", &testTransporter{})

	server.Start()
	if _, err := server.Do(&DefaultJoinCommand{Name: server.Name()}); err != nil {
		t.Fatalf("Server %s unable to join: %v", server.Name(), err)
	}

	server.currentTerm = 2
	defer server.Stop()
	resp := server.RequestVote(newRequestVoteRequest(1, "foo", 1, 0))
	if resp.Term != 2 || resp.VoteGranted {
		t.Fatalf("Invalid request vote response: %v/%v", resp.Term, resp.VoteGranted)
	}
	if server.currentTerm != 2 && server.State() != Follower {
		t.Fatalf("Server did not update term and demote: %v / %v", server.currentTerm, server.State())
	}
}

// Ensure that a vote request is denied if we've already voted for a different candidate.
func TestServerRequestVoteDeniedIfAlreadyVoted(t *testing.T) {
	server := newTestServer("1", &testTransporter{})

	server.Start()
	if _, err := server.Do(&DefaultJoinCommand{Name: server.Name()}); err != nil {
		t.Fatalf("Server %s unable to join: %v", server.Name(), err)
	}

	server.currentTerm = 2
	defer server.Stop()
	resp := server.RequestVote(newRequestVoteRequest(2, "foo", 1, 0))
	if resp.Term != 2 || !resp.VoteGranted {
		t.Fatalf("First vote should not have been denied")
	}
	resp = server.RequestVote(newRequestVoteRequest(2, "bar", 1, 0))
	if resp.Term != 2 || resp.VoteGranted {
		t.Fatalf("Second vote should have been denied")
	}
}

// Ensure that a vote request is approved if vote occurs in a new term.
func TestServerRequestVoteApprovedIfAlreadyVotedInOlderTerm(t *testing.T) {
	server := newTestServer("1", &testTransporter{})

	server.Start()
	if _, err := server.Do(&DefaultJoinCommand{Name: server.Name()}); err != nil {
		t.Fatalf("Server %s unable to join: %v", server.Name(), err)
	}

	time.Sleep(time.Millisecond * 100)

	server.currentTerm = 2
	defer server.Stop()
	resp := server.RequestVote(newRequestVoteRequest(2, "foo", 2, 1))
	if resp.Term != 2 || !resp.VoteGranted || server.VotedFor() != "foo" {
		t.Fatalf("First vote should not have been denied")
	}
	resp = server.RequestVote(newRequestVoteRequest(3, "bar", 2, 1))

	if resp.Term != 3 || !resp.VoteGranted || server.VotedFor() != "bar" {
		t.Fatalf("Second vote should have been approved")
	}
}

// Ensure that a vote request is denied if the log is out of date.
func TestServerRequestVoteDenyIfCandidateLogIsBehind(t *testing.T) {
	tmpLog := newLog()
	e0, _ := newLogEntry(tmpLog, 1, 1, &testCommand1{Val: "foo", I: 20})
	e1, _ := newLogEntry(tmpLog, 2, 1, &testCommand2{X: 100})
	e2, _ := newLogEntry(tmpLog, 3, 2, &testCommand1{Val: "bar", I: 0})
	server := newTestServerWithLog("1", &testTransporter{}, []*LogEntry{e0, e1, e2})

	// start as a follower with term 2 and index 3
	server.Start()

	defer server.Stop()

	// request vote from term 3 with last log entry 2, 2
	resp := server.RequestVote(newRequestVoteRequest(3, "foo", 2, 2))
	if resp.Term != 3 || resp.VoteGranted {
		t.Fatalf("Stale index vote should have been denied [%v/%v]", resp.Term, resp.VoteGranted)
	}

	// request vote from term 2 with last log entry 2, 3
	resp = server.RequestVote(newRequestVoteRequest(2, "foo", 3, 2))
	if resp.Term != 3 || resp.VoteGranted {
		t.Fatalf("Stale term vote should have been denied [%v/%v]", resp.Term, resp.VoteGranted)
	}

	// request vote from term 3 with last log entry 2, 3
	resp = server.RequestVote(newRequestVoteRequest(3, "foo", 3, 2))
	if resp.Term != 3 || !resp.VoteGranted {
		t.Fatalf("Matching log vote should have been granted")
	}

	// request vote from term 3 with last log entry 2, 4
	resp = server.RequestVote(newRequestVoteRequest(3, "foo", 4, 2))
	if resp.Term != 3 || !resp.VoteGranted {
		t.Fatalf("Ahead-of-log vote should have been granted")
	}
}

// //--------------------------------------
// // Promotion
// //--------------------------------------

// // Ensure that we can self-promote a server to candidate, obtain votes and become a fearless leader.
func TestServerPromoteSelf(t *testing.T) {
	e0, _ := newLogEntry(newLog(), 1, 1, &testCommand1{Val: "foo", I: 20})
	server := newTestServerWithLog("1", &testTransporter{}, []*LogEntry{e0})

	// start as a follower
	server.Start()

	defer server.Stop()

	time.Sleep(2 * testElectionTimeout)

	if server.State() != Leader {
		t.Fatalf("Server self-promotion failed: %v", server.State())
	}
}

//Ensure that we can promote a server within a cluster to a leader.
func TestServerPromote(t *testing.T) {
	lookup := map[string]*Server{}
	transporter := &testTransporter{}
	transporter.sendVoteRequestFunc = func(server *Server, peer *Peer, req *RequestVoteRequest) *RequestVoteResponse {
		return lookup[peer.Name()].RequestVote(req)
	}
	transporter.sendAppendEntriesRequestFunc = func(server *Server, peer *Peer, req *AppendEntriesRequest) *AppendEntriesResponse {
		return lookup[peer.Name()].AppendEntries(req)
	}
	servers := newTestCluster([]string{"1", "2", "3"}, transporter, lookup)

	servers[0].Start()
	servers[1].Start()
	servers[2].Start()

	time.Sleep(2 * testElectionTimeout)

	if servers[0].State() != Leader && servers[1].State() != Leader && servers[2].State() != Leader {
		t.Fatalf("No leader elected: (%s, %s, %s)", servers[0].State(), servers[1].State(), servers[2].State())
	}
	for _, server := range servers {
		server.Stop()
	}
}

//--------------------------------------
// Append Entries
//--------------------------------------

// Ensure we can append entries to a server.
func TestServerAppendEntries(t *testing.T) {
	server := newTestServer("1", &testTransporter{})

	server.SetHeartbeatTimeout(time.Second * 10)
	server.Start()
	defer server.Stop()

	// Append single entry.
	e, _ := newLogEntry(nil, 1, 1, &testCommand1{Val: "foo", I: 10})
	entries := []*LogEntry{e}
	resp := server.AppendEntries(newAppendEntriesRequest(1, 0, 0, 0, "ldr", entries))
	if resp.Term != 1 || !resp.Success {
		t.Fatalf("AppendEntries failed: %v/%v", resp.Term, resp.Success)
	}
	if index, term := server.log.commitInfo(); index != 0 || term != 0 {
		t.Fatalf("Invalid commit info [IDX=%v, TERM=%v]", index, term)
	}

	// Append multiple entries + commit the last one.
	e1, _ := newLogEntry(nil, 2, 1, &testCommand1{Val: "bar", I: 20})
	e2, _ := newLogEntry(nil, 3, 1, &testCommand1{Val: "baz", I: 30})
	entries = []*LogEntry{e1, e2}
	resp = server.AppendEntries(newAppendEntriesRequest(1, 1, 1, 1, "ldr", entries))
	if resp.Term != 1 || !resp.Success {
		t.Fatalf("AppendEntries failed: %v/%v", resp.Term, resp.Success)
	}
	if index, term := server.log.commitInfo(); index != 1 || term != 1 {
		t.Fatalf("Invalid commit info [IDX=%v, TERM=%v]", index, term)
	}

	// Send zero entries and commit everything.
	resp = server.AppendEntries(newAppendEntriesRequest(2, 3, 1, 3, "ldr", []*LogEntry{}))
	if resp.Term != 2 || !resp.Success {
		t.Fatalf("AppendEntries failed: %v/%v", resp.Term, resp.Success)
	}
	if index, term := server.log.commitInfo(); index != 3 || term != 1 {
		t.Fatalf("Invalid commit info [IDX=%v, TERM=%v]", index, term)
	}
}

//Ensure that entries with stale terms are rejected.
func TestServerAppendEntriesWithStaleTermsAreRejected(t *testing.T) {
	server := newTestServer("1", &testTransporter{})

	server.Start()

	defer server.Stop()
	server.currentTerm = 2

	// Append single entry.
	e, _ := newLogEntry(nil, 1, 1, &testCommand1{Val: "foo", I: 10})
	entries := []*LogEntry{e}
	resp := server.AppendEntries(newAppendEntriesRequest(1, 0, 0, 0, "ldr", entries))
	if resp.Term != 2 || resp.Success {
		t.Fatalf("AppendEntries should have failed: %v/%v", resp.Term, resp.Success)
	}
	if index, term := server.log.commitInfo(); index != 0 || term != 0 {
		t.Fatalf("Invalid commit info [IDX=%v, TERM=%v]", index, term)
	}
}

// Ensure that we reject entries if the commit log is different.
func TestServerAppendEntriesRejectedIfAlreadyCommitted(t *testing.T) {
	server := newTestServer("1", &testTransporter{})
	server.Start()

	defer server.Stop()

	// Append single entry + commit.
	e1, _ := newLogEntry(nil, 1, 1, &testCommand1{Val: "foo", I: 10})
	e2, _ := newLogEntry(nil, 2, 1, &testCommand1{Val: "foo", I: 15})
	entries := []*LogEntry{e1, e2}
	resp := server.AppendEntries(newAppendEntriesRequest(1, 0, 0, 2, "ldr", entries))
	if resp.Term != 1 || !resp.Success {
		t.Fatalf("AppendEntries failed: %v/%v", resp.Term, resp.Success)
	}

	// Append entry again (post-commit).
	e, _ := newLogEntry(nil, 2, 1, &testCommand1{Val: "bar", I: 20})
	entries = []*LogEntry{e}
	resp = server.AppendEntries(newAppendEntriesRequest(1, 2, 1, 1, "ldr", entries))
	if resp.Term != 1 || resp.Success {
		t.Fatalf("AppendEntries should have failed: %v/%v", resp.Term, resp.Success)
	}
}

// Ensure that we uncommitted entries are rolled back if new entries overwrite them.
func TestServerAppendEntriesOverwritesUncommittedEntries(t *testing.T) {
	server := newTestServer("1", &testTransporter{})
	server.Start()
	defer server.Stop()

	entry1, _ := newLogEntry(nil, 1, 1, &testCommand1{Val: "foo", I: 10})
	entry2, _ := newLogEntry(nil, 2, 1, &testCommand1{Val: "foo", I: 15})
	entry3, _ := newLogEntry(nil, 2, 2, &testCommand1{Val: "bar", I: 20})

	// Append single entry + commit.
	entries := []*LogEntry{entry1, entry2}
	resp := server.AppendEntries(newAppendEntriesRequest(1, 0, 0, 1, "ldr", entries))
	if resp.Term != 1 || !resp.Success || server.log.commitIndex != 1 || !reflect.DeepEqual(server.log.entries, []*LogEntry{entry1, entry2}) {
		t.Fatalf("AppendEntries failed: %v/%v", resp.Term, resp.Success)
	}

	// Append entry that overwrites the second (uncommitted) entry.
	entries = []*LogEntry{entry3}
	resp = server.AppendEntries(newAppendEntriesRequest(2, 1, 1, 2, "ldr", entries))
	if resp.Term != 2 || !resp.Success || server.log.commitIndex != 2 || !reflect.DeepEqual(server.log.entries, []*LogEntry{entry1, entry3}) {
		t.Fatalf("AppendEntries should have succeeded: %v/%v", resp.Term, resp.Success)
	}
}

//--------------------------------------
// Command Execution
//--------------------------------------

// Ensure that a follower cannot execute a command.
func TestServerDenyCommandExecutionWhenFollower(t *testing.T) {
	server := newTestServer("1", &testTransporter{})
	server.Start()
	defer server.Stop()
	var err error
	if _, err = server.Do(&testCommand1{Val: "foo", I: 10}); err != NotLeaderError {
		t.Fatalf("Expected error: %v, got: %v", NotLeaderError, err)
	}
}

//--------------------------------------
// Membership
//--------------------------------------

// Ensure that we can start a single server and append to its log.
func TestServerSingleNode(t *testing.T) {
	server := newTestServer("1", &testTransporter{})
	if server.State() != Stopped {
		t.Fatalf("Unexpected server state: %v", server.State())
	}

	server.Start()

	time.Sleep(testHeartbeatTimeout)

	// Join the server to itself.
	if _, err := server.Do(&DefaultJoinCommand{Name: "1"}); err != nil {
		t.Fatalf("Unable to join: %v", err)
	}
	debugln("finish command")

	if server.State() != Leader {
		t.Fatalf("Unexpected server state: %v", server.State())
	}

	server.Stop()

	if server.State() != Stopped {
		t.Fatalf("Unexpected server state: %v", server.State())
	}
}

// Ensure that we can start multiple servers and determine a leader.
func TestServerMultiNode(t *testing.T) {
	// Initialize the servers.
	var mutex sync.RWMutex
	servers := map[string]*Server{}

	transporter := &testTransporter{}
	transporter.sendVoteRequestFunc = func(server *Server, peer *Peer, req *RequestVoteRequest) *RequestVoteResponse {
		mutex.RLock()
		s := servers[peer.name]
		mutex.RUnlock()
		return s.RequestVote(req)
	}
	transporter.sendAppendEntriesRequestFunc = func(server *Server, peer *Peer, req *AppendEntriesRequest) *AppendEntriesResponse {
		mutex.RLock()
		s := servers[peer.name]
		mutex.RUnlock()
		return s.AppendEntries(req)
	}

	disTransporter := &testTransporter{}
	disTransporter.sendVoteRequestFunc = func(server *Server, peer *Peer, req *RequestVoteRequest) *RequestVoteResponse {
		return nil
	}
	disTransporter.sendAppendEntriesRequestFunc = func(server *Server, peer *Peer, req *AppendEntriesRequest) *AppendEntriesResponse {
		return nil
	}

	var names []string

	n := 5

	// add n servers
	for i := 1; i <= n; i++ {
		names = append(names, strconv.Itoa(i))
	}

	var leader *Server
	for _, name := range names {
		server := newTestServer(name, transporter)
		defer server.Stop()

		mutex.Lock()
		servers[name] = server
		mutex.Unlock()

		if name == "1" {
			leader = server
			server.SetHeartbeatTimeout(testHeartbeatTimeout)
			server.Start()
			time.Sleep(testHeartbeatTimeout)
		} else {
			server.SetElectionTimeout(testElectionTimeout)
			server.SetHeartbeatTimeout(testHeartbeatTimeout)
			server.Start()
			time.Sleep(testHeartbeatTimeout)
		}
		if _, err := leader.Do(&DefaultJoinCommand{Name: name}); err != nil {
			t.Fatalf("Unable to join server[%s]: %v", name, err)
		}

	}
	time.Sleep(2 * testElectionTimeout)

	// Check that two peers exist on leader.
	mutex.RLock()
	if leader.MemberCount() != n {
		t.Fatalf("Expected member count to be %v, got %v", n, leader.MemberCount())
	}
	if servers["2"].State() == Leader || servers["3"].State() == Leader {
		t.Fatalf("Expected leader should be 1: 2=%v, 3=%v\n", servers["2"].state, servers["3"].state)
	}
	mutex.RUnlock()

	for i := 0; i < 20; i++ {
		retry := 0
		fmt.Println("Round ", i)

		num := strconv.Itoa(i%(len(servers)) + 1)
		num_1 := strconv.Itoa((i+3)%(len(servers)) + 1)
		toStop := servers[num]
		toStop_1 := servers[num_1]

		// Stop the first server and wait for a re-election.
		time.Sleep(2 * testElectionTimeout)
		debugln("Disconnect ", toStop.Name())
		debugln("disconnect ", num, " ", num_1)
		toStop.SetTransporter(disTransporter)
		toStop_1.SetTransporter(disTransporter)
		time.Sleep(2 * testElectionTimeout)
		// Check that either server 2 or 3 is the leader now.
		//mutex.Lock()

		leader := 0

		for key, value := range servers {
			debugln("Play begin")
			if key != num && key != num_1 {
				if value.State() == Leader {
					debugln("Found leader")
					for i := 0; i < 10; i++ {
						debugln("[Test] do ", value.Name())
						if _, err := value.Do(&testCommand2{X: 1}); err != nil {
							break
						}
						debugln("[Test] Done")
					}
					debugln("Leader is ", value.Name(), " Index ", value.log.commitIndex)
				}
				debugln("Not Found leader")
			}
		}
		for {
			for key, value := range servers {
				if key != num && key != num_1 {
					if value.State() == Leader {
						leader++
					}
					debugln(value.Name(), " ", value.currentTerm, " ", value.state)
				}
			}

			if leader > 1 {
				if retry < 300 {
					debugln("retry")
					retry++
					leader = 0
					time.Sleep(2 * testElectionTimeout)
					continue
				}
				t.Fatalf("wrong leader number %v", leader)
			}
			if leader == 0 {
				if retry < 300 {
					retry++
					fmt.Println("retry 0")
					leader = 0
					time.Sleep(2 * testElectionTimeout)
					continue
				}
				t.Fatalf("wrong leader number %v", leader)
			}
			if leader == 1 {
				break
			}
		}

		//mutex.Unlock()

		toStop.SetTransporter(transporter)
		toStop_1.SetTransporter(transporter)
	}

}
