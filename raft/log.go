package raft

const (
	Normal int = iota

	AddNode
	RemoveNode
)

type Entry struct {
	Type int
	Term int
	Data []byte
}

func (e *Entry) isConfig() bool {
	return e.Type == AddNode || e.Type == RemoveNode
}

type log struct {
	ents      []Entry
	committed int
	applied   int
	offset    int
}

func newLog() *log {
	return &log{
		ents:      make([]Entry, 1),
		committed: 0,
		applied:   0,
	}
}

func (l *log) maybeAppend(index, logTerm, committed int, ents ...Entry) bool {
	if l.matchTerm(index, logTerm) {
		l.append(index, ents...)
		l.committed = committed
		return true
	}
	return false
}

func (l *log) append(after int, ents ...Entry) int {
	l.ents = append(l.slice(0, after+1), ents...)
	return l.lastIndex()
}

func (l *log) lastIndex() int {
	return len(l.ents) - 1 + l.offset
}

func (l *log) term(i int) int {
	if e := l.at(i); e != nil {
		return e.Term
	}
	return -1
}

func (l *log) entries(i int) []Entry {
	return l.slice(i, l.lastIndex()+1)
}

func (l *log) isUpToDate(i, term int) bool {
	e := l.at(l.lastIndex())
	return term > e.Term || (term == e.Term && i >= l.lastIndex())
}

func (l *log) matchTerm(i, term int) bool {
	if e := l.at(i); e != nil {
		return e.Term == term
	}
	return false
}

func (l *log) maybeCommit(maxIndex, term int) bool {
	if maxIndex > l.committed && l.term(maxIndex) == term {
		l.committed = maxIndex
		return true
	}
	return false
}

// nextEnts returns all the avaliable entries for execution.
// all the returned entries will be marked as applied.
func (l *log) nextEnts() (ents []Entry) {
	if l.committed > l.applied {
		ents = l.slice(l.applied+1, l.committed+1)
		l.applied = l.committed
	}
	return ents
}

func (l *log) at(i int) *Entry {
	if l.isOutOfBounds(i) {
		return nil
	}
	return &l.ents[i-l.offset]
}

// slice get a slice of log entries from lo through hi-1, inclusive.
func (l *log) slice(lo int, hi int) []Entry {
	if lo >= hi {
		return nil
	}
	if l.isOutOfBounds(lo) || l.isOutOfBounds(hi-1) {
		return nil
	}
	return l.ents[lo-l.offset : hi-l.offset]
}

func (l *log) isOutOfBounds(i int) bool {
	if i < l.offset || i > l.lastIndex() {
		return true
	}
	return false
}
