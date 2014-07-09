package raft

type Snapshot struct {
	Data []byte

	// the configuration
	Nodes []int64
	// the index at which the snapshot was taken.
	Index int
	// the log term of the index
	Term int
}

// A snapshoter can make a snapshot of its current state atomically.
// It can restore from a snapshot and get the latest snapshot it took.
type Snapshoter interface {
	Snap(index, term int, nodes []int64)
	Restore(snap Snapshot)
	GetSnap() Snapshot
}
