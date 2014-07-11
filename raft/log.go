package raft

import "fmt"

const (
	Normal int64 = iota

	AddNode
	RemoveNode
)

const (
	defaultCompactThreshold = 10000
)

type Entry struct {
	Type int64
	Term int64
	Data []byte
}

func (e *Entry) isConfig() bool {
	return e.Type == AddNode || e.Type == RemoveNode
}

type log struct {
	ents      []Entry
	committed int64
	applied   int64
	offset    int64

	// want a compact after the number of entries exceeds the threshold
	// TODO(xiangli) size might be a better criteria
	compactThreshold int64
}

func newLog() *log {
	return &log{
		ents:             make([]Entry, 1),
		committed:        0,
		applied:          0,
		compactThreshold: defaultCompactThreshold,
	}
}

func (l *log) maybeAppend(index, logTerm, committed int64, ents ...Entry) bool {
	if l.matchTerm(index, logTerm) {
		l.append(index, ents...)
		l.committed = committed
		return true
	}
	return false
}

func (l *log) append(after int64, ents ...Entry) int64 {
	l.ents = append(l.slice(l.offset, after+1), ents...)
	return l.lastIndex()
}

func (l *log) lastIndex() int64 {
	return int64(len(l.ents)) - 1 + l.offset
}

func (l *log) term(i int64) int64 {
	if e := l.at(i); e != nil {
		return e.Term
	}
	return -1
}

func (l *log) entries(i int64) []Entry {
	// never send out the first entry
	// first entry is only used for matching
	// prevLogTerm
	if i == l.offset {
		panic("cannot return the first entry in log")
	}
	return l.slice(i, l.lastIndex()+1)
}

func (l *log) isUpToDate(i, term int64) bool {
	e := l.at(l.lastIndex())
	return term > e.Term || (term == e.Term && i >= l.lastIndex())
}

func (l *log) matchTerm(i, term int64) bool {
	if e := l.at(i); e != nil {
		return e.Term == term
	}
	return false
}

func (l *log) maybeCommit(maxIndex, term int64) bool {
	if maxIndex > l.committed && l.term(maxIndex) == term {
		l.committed = maxIndex
		return true
	}
	return false
}

// nextEnts returns all the available entries for execution.
// all the returned entries will be marked as applied.
func (l *log) nextEnts() (ents []Entry) {
	if l.committed > l.applied {
		ents = l.slice(l.applied+1, l.committed+1)
		l.applied = l.committed
	}
	return ents
}

// compact removes the log entries before i, exclusive.
// i must be not smaller than the index of the first entry
// and not greater than the index of the last entry.
// the number of entries after compaction will be returned.
func (l *log) compact(i int64) int64 {
	if l.isOutOfBounds(i) {
		panic(fmt.Sprintf("compact %d out of bounds [%d:%d]", i, l.offset, l.lastIndex()))
	}
	l.ents = l.slice(i, l.lastIndex()+1)
	l.offset = i
	return int64(len(l.ents))
}

func (l *log) shouldCompact() bool {
	return (l.applied - l.offset) > l.compactThreshold
}

func (l *log) restore(index, term int64) {
	l.ents = []Entry{{Term: term}}
	l.committed = index
	l.applied = index
	l.offset = index
}

func (l *log) at(i int64) *Entry {
	if l.isOutOfBounds(i) {
		return nil
	}
	return &l.ents[i-l.offset]
}

// slice get a slice of log entries from lo through hi-1, inclusive.
func (l *log) slice(lo int64, hi int64) []Entry {
	if lo >= hi {
		return nil
	}
	if l.isOutOfBounds(lo) || l.isOutOfBounds(hi-1) {
		return nil
	}
	return l.ents[lo-l.offset : hi-l.offset]
}

func (l *log) isOutOfBounds(i int64) bool {
	if i < l.offset || i > l.lastIndex() {
		return true
	}
	return false
}
