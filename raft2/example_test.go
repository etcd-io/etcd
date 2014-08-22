package raft

import (
	"log"

	"code.google.com/p/go.net/context"
)

func applyToStore(ents []Entry)        {}
func sendMessages(msgs []Message)      {}
func saveStateToDisk(st State)         {}
func saveToDisk(ents []Entry)          {}
func stateChanged(prev, st State) bool { return false }

func Example_Node() {
	n := Start(context.Background(), "", 0, 0)

	// stuff to n happens in other gorotines

	// the last known state
	var prev State
	for {
		// ReadState blocks until there is new state ready.
		st, ents, cents, msgs, err := n.ReadState()
		if err != nil {
			log.Fatal(err)
		}

		if stateChanged(prev, st) {
			saveStateToDisk(st)
			prev = st
		}

		saveToDisk(ents)
		applyToStore(cents)
		sendMessages(msgs)
	}
}
