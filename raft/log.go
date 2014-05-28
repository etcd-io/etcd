package raft

type Entry struct {
	Term int
	Data []byte
}

type log struct {
	ents    []Entry
	commit  int
	applied int
}

func newLog() *log {
	return &log{
		ents:    make([]Entry, 1),
		commit:  0,
		applied: 0,
	}
}

func (l *log) maybeAppend(index, logTerm, commit int, ents ...Entry) bool {
	if l.matchTerm(index, logTerm) {
		l.append(index, ents...)
		l.commit = commit
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
	if maxIndex > l.commit && l.term(maxIndex) == term {
		l.commit = maxIndex
		return true
	}
	return false
}

func (l *log) nextEnts() (ents []Entry) {
	if l.commit > l.applied {
		ents = l.ents[l.applied+1 : l.commit+1]
		l.applied = l.commit
	}
	return ents
}
