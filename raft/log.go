package raft

import "fmt"

const (
	Normal int64 = iota

	ClusterInit
	AddNode
	RemoveNode
)

const (
	defaultCompactThreshold = 10000
)

func (e *Entry) isConfig() bool {
	return e.Type == AddNode || e.Type == RemoveNode
}

type raftLog struct {
	ents             []Entry
	unstable         int64
	committed        int64
	applied          int64
	offset           int64
	snapshot         Snapshot
	unstableSnapshot Snapshot

	// want a compact after the number of entries exceeds the threshold
	// TODO(xiangli) size might be a better criteria
	compactThreshold int64
}

func newLog() *raftLog {
	return &raftLog{
		ents:             make([]Entry, 1),
		unstable:         1,
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

func (l *raftLog) maybeAppend(index, logTerm, committed int64, ents ...Entry) bool {
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

func (l *raftLog) append(after int64, ents ...Entry) int64 {
	l.ents = append(l.slice(l.offset, after+1), ents...)
	l.unstable = min(l.unstable, after+1)
	return l.lastIndex()
}

func (l *raftLog) findConflict(from int64, ents []Entry) int64 {
	for i, ne := range ents {
		if oe := l.at(from + int64(i)); oe == nil || oe.Term != ne.Term {
			return from + int64(i)
		}
	}
	return -1
}

func (l *raftLog) unstableEnts() []Entry {
	ents := l.entries(l.unstable)
	l.unstable = l.lastIndex() + 1
	return ents
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

func (l *raftLog) entries(i int64) []Entry {
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

// nextEnts returns all the available entries for execution.
// all the returned entries will be marked as applied.
func (l *raftLog) nextEnts() (ents []Entry) {
	if l.committed > l.applied {
		ents = l.slice(l.applied+1, l.committed+1)
		l.applied = l.committed
	}
	return ents
}

// compact compacts all log entries until i.
// It removes the log entries before i, exclusive.
// i must be not smaller than the index of the first entry
// and not greater than the index of the last entry.
// the number of entries after compaction will be returned.
func (l *raftLog) compact(i int64) int64 {
	if l.isOutOfBounds(i) {
		panic(fmt.Sprintf("compact %d out of bounds [%d:%d]", i, l.offset, l.lastIndex()))
	}
	l.ents = l.slice(i, l.lastIndex()+1)
	l.unstable = max(i+1, l.unstable)
	l.offset = i
	return int64(len(l.ents))
}

func (l *raftLog) snap(d []byte, clusterId, index, term int64, nodes []int64) {
	l.snapshot = Snapshot{clusterId, d, nodes, index, term}
}

func (l *raftLog) shouldCompact() bool {
	return (l.applied - l.offset) > l.compactThreshold
}

func (l *raftLog) restore(s Snapshot) {
	l.ents = []Entry{{Term: s.Term}}
	l.unstable = s.Index + 1
	l.committed = s.Index
	l.applied = s.Index
	l.offset = s.Index
	l.snapshot = s
	l.unstableSnapshot = s
}

func (l *raftLog) at(i int64) *Entry {
	if l.isOutOfBounds(i) {
		return nil
	}
	return &l.ents[i-l.offset]
}

// slice get a slice of log entries from lo through hi-1, inclusive.
func (l *raftLog) slice(lo int64, hi int64) []Entry {
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
