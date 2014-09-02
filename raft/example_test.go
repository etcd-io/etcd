package raft

import (
	pb "github.com/coreos/etcd/raft/raftpb"
)

func applyToStore(ents []pb.Entry)   {}
func sendMessages(msgs []pb.Message) {}
func saveStateToDisk(st pb.State)    {}
func saveToDisk(ents []pb.Entry)     {}

func Example_Node() {
	n := Start(0, nil, 0, 0)

	// stuff to n happens in other goroutines

	// the last known state
	var prev pb.State
	for {
		// ReadState blocks until there is new state ready.
		rd := <-n.Ready()
		if !isStateEqual(prev, rd.State) {
			saveStateToDisk(rd.State)
			prev = rd.State
		}

		saveToDisk(rd.Entries)
		go applyToStore(rd.CommittedEntries)
		sendMessages(rd.Messages)
	}
}
