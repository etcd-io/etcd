package raft

import (
	"log"

	"code.google.com/p/go.net/context"
)

func apply(e Entry)               {}
func sendMessages(msgs []Message) {}
func saveToDisk(ents []Entry)     {}

func Example_Node() {
	n := Start(context.Background(), "", 0, 0)

	// stuff to n happens in other gorotines

	// a cache of entries that have been saved to disk, but not yet
	// committed the the store
	var cents []Entry
	for {
		// ReadState blocks until there is new state ready.
		st, ents, msgs, err := n.ReadState()
		if err != nil {
			log.Fatal(err)
		}

		saveToDisk(ents)

		cents = append(cents, ents...)
		for i, e := range cents {
			if e.Index > st.Commit {
				cents = cents[i:]
				break
			}
			apply(e)
		}

		sendMessages(msgs)
	}
}
