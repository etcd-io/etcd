package raft

import (
	"fmt"

	pb "github.com/coreos/etcd/raft/raftpb"
)

const (
	defaultCompactThreshold = 10000
)

type raftLog struct {
	ents             []pb.Entry
	unstable         int64
	committed        int64
	applied          int64
	offset           int64
	snapshot         pb.Snapshot
	unstableSnapshot pb.Snapshot

	// want a compact after the number of entries exceeds the threshold
	// TODO(xiangli) size might be a better criteria
	compactThreshold int64
}

func newLog() *raftLog {
	return &raftLog{
		ents:             make([]pb.Entry, 1),
		unstable:         0,
		committed:        0,
		applied:          0,
		compactThreshold: defaultCompactThreshold,
	}
}

func (l *raftLog) isEmpty() bool {
	return l.offset == 0 && len(l.ents) == 1
}

func (l *raftLog) String() string {
	return fmt.Sprintf("offset=%d committed=%d applied=%d len(ents)=%d", l.offset, l.committed, l.applied, len(l.ents))
}

func (l *raftLog) maybeAppend(index, logTerm, committed int64, ents ...pb.Entry) bool {
	if l.matchTerm(index, logTerm) {
		from := index + 1
		ci := l.findConflict(from, ents)
		switch {
		case ci == -1:
		case ci <= l.committed:
			panic("conflict with committed entry")
		default:
			l.append(ci-1, ents[ci-from:]...)
		}
		if l.committed < committed {
			l.committed = min(committed, l.lastIndex())
		}
		return true
	}
	return false
}

func (l *raftLog) append(after int64, ents ...pb.Entry) int64 {
	l.ents = append(l.slice(l.offset, after+1), ents...)
	l.unstable = min(l.unstable, after+1)
	return l.lastIndex()
}

func (l *raftLog) findConflict(from int64, ents []pb.Entry) int64 {
	for i, ne := range ents {
		if oe := l.at(from + int64(i)); oe == nil || oe.Term != ne.Term {
			return from + int64(i)
		}
	}
	return -1
}

func (l *raftLog) unstableEnts() []pb.Entry {
	ents := l.slice(l.unstable, l.lastIndex()+1)
	if ents == nil {
		return nil
	}
	cpy := make([]pb.Entry, len(ents))
	copy(cpy, ents)
	return cpy
}

func (l *raftLog) resetUnstable() {
	l.unstable = l.lastIndex() + 1
}

// nextEnts returns all the available entries for execution.
// all the returned entries will be marked as applied.
func (l *raftLog) nextEnts() (ents []pb.Entry) {
	if l.committed > l.applied {
		return l.slice(l.applied+1, l.committed+1)
	}
	return nil
}

func (l *raftLog) resetNextEnts() {
	if l.committed > l.applied {
		l.applied = l.committed
	}
}

func (l *raftLog) lastIndex() int64 {
	return int64(len(l.ents)) - 1 + l.offset
}

func (l *raftLog) term(i int64) int64 {
	if e := l.at(i); e != nil {
		return e.Term
	}
	return -1
}

func (l *raftLog) entries(i int64) []pb.Entry {
	// never send out the first entry
	// first entry is only used for matching
	// prevLogTerm
	if i == l.offset {
		panic("cannot return the first entry in log")
	}
	return l.slice(i, l.lastIndex()+1)
}

func (l *raftLog) isUpToDate(i, term int64) bool {
	e := l.at(l.lastIndex())
	return term > e.Term || (term == e.Term && i >= l.lastIndex())
}

func (l *raftLog) matchTerm(i, term int64) bool {
	if e := l.at(i); e != nil {
		return e.Term == term
	}
	return false
}

func (l *raftLog) maybeCommit(maxIndex, term int64) bool {
	if maxIndex > l.committed && l.term(maxIndex) == term {
		l.committed = maxIndex
		return true
	}
	return false
}

// compact compacts all log entries until i.
// It removes the log entries before i, exclusive.
// i must be not smaller than the index of the first entry
// and not greater than the index of the last entry.
// the number of entries after compaction will be returned.
func (l *raftLog) compact(i int64) int64 {
	if l.isOutOfAppliedBounds(i) {
		panic(fmt.Sprintf("compact %d out of bounds [%d:%d]", i, l.offset, l.applied))
	}
	l.ents = l.slice(i, l.lastIndex()+1)
	l.unstable = max(i+1, l.unstable)
	l.offset = i
	return int64(len(l.ents))
}

func (l *raftLog) snap(d []byte, index, term int64, nodes []int64) {
	l.snapshot = pb.Snapshot{
		Data:  d,
		Nodes: nodes,
		Index: index,
		Term:  term,
	}
}

func (l *raftLog) shouldCompact() bool {
	return (l.applied - l.offset) > l.compactThreshold
}

func (l *raftLog) restore(s pb.Snapshot) {
	l.ents = []pb.Entry{{Term: s.Term}}
	l.unstable = s.Index + 1
	l.committed = s.Index
	l.applied = s.Index
	l.offset = s.Index
	l.snapshot = s
}

func (l *raftLog) at(i int64) *pb.Entry {
	if l.isOutOfBounds(i) {
		return nil
	}
	return &l.ents[i-l.offset]
}

// slice returns a slice of log entries from lo through hi-1, inclusive.
func (l *raftLog) slice(lo int64, hi int64) []pb.Entry {
	if lo >= hi {
		return nil
	}
	if l.isOutOfBounds(lo) || l.isOutOfBounds(hi-1) {
		return nil
	}
	return l.ents[lo-l.offset : hi-l.offset]
}

func (l *raftLog) isOutOfBounds(i int64) bool {
	if i < l.offset || i > l.lastIndex() {
		return true
	}
	return false
}

func (l *raftLog) isOutOfAppliedBounds(i int64) bool {
	if i < l.offset || i > l.applied {
		return true
	}
	return false
}

func min(a, b int64) int64 {
	if a > b {
		return b
	}
	return a
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
