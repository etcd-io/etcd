package raft

import "code.google.com/p/go.net/context"

func applyToStore(ents []Entry)   {}
func sendMessages(msgs []Message) {}
func saveStateToDisk(st State)    {}
func saveToDisk(ents []Entry)     {}

func Example_Node() {
	n := Start(context.Background(), 0, nil)

	// stuff to n happens in other goroutines

	// the last known state
	var prev State
	for {
		// ReadState blocks until there is new state ready.
		rd := <-n.Ready()
		if !prev.Equal(rd.State) {
			saveStateToDisk(rd.State)
			prev = rd.State
		}

		saveToDisk(rd.Entries)
		go applyToStore(rd.CommittedEntries)
		sendMessages(rd.Messages)
	}
}
