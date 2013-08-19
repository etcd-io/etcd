package raft

type Config struct {
	CommitIndex uint64 `json:"commitIndex"`
	// TODO decide what we need to store in peer struct
	Peers []*Peer `json:"peers"`
}
