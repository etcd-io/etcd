package raft

const (
	Normal int = iota

	ConfigAdd
	ConfigRemove
)

type Entry struct {
	Type int
	Term int
	Data []byte
}

type log struct {
	ents      []Entry
	committed int
	applied   int
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
	l.ents = append(l.ents[:after+1], ents...)
	return l.lastIndex()
}

func (l *log) lastIndex() int {
	return len(l.ents) - 1
}

func (l *log) term(i int) int {
	if i > l.lastIndex() {
		return -1
	}
	return l.ents[i].Term
}

func (l *log) entries(i int) []Entry {
	if i > l.lastIndex() {
		return nil
	}
	return l.ents[i:]
}

func (l *log) isUpToDate(i, term int) bool {
	e := l.ents[l.lastIndex()]
	return term > e.Term || (term == e.Term && i >= l.lastIndex())
}

func (l *log) matchTerm(i, term int) bool {
	if i > l.lastIndex() {
		return false
	}
	return l.ents[i].Term == term
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
		ents = l.ents[l.applied+1 : l.committed+1]
		l.applied = l.committed
	}
	return ents
}
