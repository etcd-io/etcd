package raft

type State struct {
	CommitIndex int64
}

type Message struct {
	State State
	To    string
	Data  []byte
}

type Entry struct {
	Id    int64
	Index int64
	Data  []byte
}

type raft struct {
	name string

	State

	election  int
	heartbeat int

	msgs []Message
	ents []Entry
}

func (sm *raft) hasLeader() bool               { return false }
func (sm *raft) step(m Message)                {}
func (sm *raft) resetState()                   {}
func (sm *raft) propose(id int64, data []byte) {}
