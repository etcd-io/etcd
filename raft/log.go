/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package raft

import (
	"fmt"
	"log"

	pb "github.com/coreos/etcd/raft/raftpb"
)

type raftLog struct {
	// storage contains all stable entries since the last snapshot.
	storage Storage
	// unstableEnts contains all entries that have not yet been written
	// to storage.
	unstableEnts []pb.Entry
	// unstableEnts[i] has raft log position i+unstable.  Note that
	// unstable may be less than the highest log position in storage;
	// this means that the next write to storage will truncate the log
	// before persisting unstableEnts.
	unstable uint64
	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	// Invariant: committed < unstable
	committed uint64
	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	applied  uint64
	snapshot pb.Snapshot
}

func newLog(storage Storage) *raftLog {
	if storage == nil {
		panic("storage must not be nil")
	}
	log := &raftLog{
		storage: storage,
	}
	lastIndex, err := storage.GetLastIndex()
	if err == ErrStorageEmpty {
		// When starting from scratch populate the list with a dummy entry at term zero.
		log.unstableEnts = make([]pb.Entry, 1)
	} else if err == nil {
		log.unstable = lastIndex + 1
	} else {
		panic(err) // TODO(bdarnell)
	}
	return log
}

func (l *raftLog) String() string {
	return fmt.Sprintf("unstable=%d committed=%d applied=%d", l.unstable, l.committed, l.applied)

}

// maybeAppend returns (0, false) if the entries cannot be appended. Otherwise,
// it returns (last index of new entries, true).
func (l *raftLog) maybeAppend(index, logTerm, committed uint64, ents ...pb.Entry) (lastnewi uint64, ok bool) {
	lastnewi = index + uint64(len(ents))
	if l.matchTerm(index, logTerm) {
		from := index + 1
		ci := l.findConflict(from, ents)
		switch {
		case ci == 0:
		case ci <= l.committed:
			panic("conflict with committed entry")
		default:
			l.append(ci-1, ents[ci-from:]...)
		}
		tocommit := min(committed, lastnewi)
		// if toCommit > commitIndex, set commitIndex = toCommit
		if l.committed < tocommit {
			l.committed = tocommit
		}
		return lastnewi, true
	}
	return 0, false
}

func (l *raftLog) append(after uint64, ents ...pb.Entry) uint64 {
	if after < l.unstable {
		// The log is being truncated to before our current unstable
		// portion, so discard it and reset unstable.
		l.unstableEnts = nil
		l.unstable = after + 1
	}
	// Truncate any unstable entries that are being replaced, then
	// append the new ones.
	l.unstableEnts = append(l.unstableEnts[0:1+after-l.unstable], ents...)
	l.unstable = min(l.unstable, after+1)
	return l.lastIndex()
}

// findConflict finds the index of the conflict.
// It returns the first pair of conflicting entries between the existing
// entries and the given entries, if there are any.
// If there is no conflicting entries, and the existing entries contains
// all the given entries, zero will be returned.
// If there is no conflicting entries, but the given entries contains new
// entries, the index of the first new entry will be returned.
// An entry is considered to be conflicting if it has the same index but
// a different term.
// The first entry MUST have an index equal to the argument 'from'.
// The index of the given entries MUST be continuously increasing.
func (l *raftLog) findConflict(from uint64, ents []pb.Entry) uint64 {
	// TODO(xiangli): validate the index of ents
	for i, ne := range ents {
		if oe := l.at(from + uint64(i)); oe == nil || oe.Term != ne.Term {
			return from + uint64(i)
		}
	}
	return 0
}

func (l *raftLog) unstableEntries() []pb.Entry {
	if len(l.unstableEnts) == 0 {
		return nil
	}
	cpy := make([]pb.Entry, len(l.unstableEnts))
	copy(cpy, l.unstableEnts)
	return cpy
}

// nextEnts returns all the available entries for execution.
// all the returned entries will be marked as applied.
func (l *raftLog) nextEnts() (ents []pb.Entry) {
	if l.committed > l.applied {
		return l.slice(l.applied+1, l.committed+1)
	}
	return nil
}

func (l *raftLog) firstIndex() uint64 {
	index, err := l.storage.GetFirstIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	return index
}

func (l *raftLog) lastIndex() uint64 {
	if len(l.unstableEnts) > 0 {
		return l.unstable + uint64(len(l.unstableEnts)) - 1
	}
	index, err := l.storage.GetLastIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	return index
}

func (l *raftLog) appliedTo(i uint64) {
	if i == 0 {
		return
	}
	if l.committed < i || i < l.applied {
		log.Panicf("applied[%d] is out of range [prevApplied(%d), committed(%d)]", i, l.applied, l.committed)
	}
	l.applied = i
}

func (l *raftLog) stableTo(i uint64) {
	l.unstableEnts = l.unstableEnts[i+1-l.unstable:]
	l.unstable = i + 1
}

func (l *raftLog) lastTerm() uint64 {
	return l.term(l.lastIndex())
}

func (l *raftLog) term(i uint64) uint64 {
	if e := l.at(i); e != nil {
		return e.Term
	}
	return 0
}

func (l *raftLog) entries(i uint64) []pb.Entry {
	// never send out the first entry
	// first entry is only used for matching
	// prevLogTerm
	if i == 0 {
		panic("cannot return the first entry in log")
	}
	return l.slice(i, l.lastIndex()+1)
}

// allEntries returns all entries in the log, including the initial
// entry that is only used for prevLogTerm validation. This method
// should only be used for testing.
func (l *raftLog) allEntries() []pb.Entry {
	return l.slice(l.firstIndex(), l.lastIndex()+1)
}

// isUpToDate determines if the given (lastIndex,term) log is more up-to-date
// by comparing the index and term of the last entries in the existing logs.
// If the logs have last entries with different terms, then the log with the
// later term is more up-to-date. If the logs end with the same term, then
// whichever log has the larger lastIndex is more up-to-date. If the logs are
// the same, the given log is up-to-date.
func (l *raftLog) isUpToDate(lasti, term uint64) bool {
	return term > l.lastTerm() || (term == l.lastTerm() && lasti >= l.lastIndex())
}

func (l *raftLog) matchTerm(i, term uint64) bool {
	if e := l.at(i); e != nil {
		return e.Term == term
	}
	return false
}

func (l *raftLog) maybeCommit(maxIndex, term uint64) bool {
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
func (l *raftLog) compact(i uint64) uint64 {
	if l.isOutOfAppliedBounds(i) {
		panic(fmt.Sprintf("compact %d out of bounds (applied up to %d)", i, l.applied))
	}
	err := l.storage.Compact(i)
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	l.unstable = max(i+1, l.unstable)
	firstIndex, err := l.storage.GetFirstIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	lastIndex, err := l.storage.GetLastIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	return lastIndex - firstIndex
}

func (l *raftLog) snap(d []byte, index, term uint64, nodes []uint64) {
	l.snapshot = pb.Snapshot{
		Data:  d,
		Nodes: nodes,
		Index: index,
		Term:  term,
	}
}

func (l *raftLog) restore(s pb.Snapshot) {
	l.storage = &MemoryStorage{
		ents:   []pb.Entry{{Term: s.Term}},
		offset: s.Index,
	}
	l.unstable = s.Index + 1
	l.unstableEnts = nil
	l.committed = s.Index
	l.applied = s.Index
	l.snapshot = s
}

func (l *raftLog) at(i uint64) *pb.Entry {
	ents := l.slice(i, i+1)
	if len(ents) == 0 {
		return nil
	}
	return &ents[0]
}

// slice returns a slice of log entries from lo through hi-1, inclusive.
func (l *raftLog) slice(lo uint64, hi uint64) []pb.Entry {
	if lo >= hi {
		return nil
	}
	if l.isOutOfBounds(lo) || l.isOutOfBounds(hi-1) {
		return nil
	}
	var ents []pb.Entry
	if lo < l.unstable {
		storedEnts, err := l.storage.GetEntries(lo, min(hi, l.unstable))
		if err != nil {
			panic(err) // TODO(bdarnell)
		}
		ents = append(ents, storedEnts...)
	}
	if len(l.unstableEnts) > 0 && hi > l.unstable {
		var firstUnstable uint64
		if lo < l.unstable {
			firstUnstable = l.unstable
		} else {
			firstUnstable = lo
		}
		ents = append(ents, l.unstableEnts[firstUnstable-l.unstable:hi-l.unstable]...)
	}
	return ents
}

func (l *raftLog) isOutOfBounds(i uint64) bool {
	if i < l.firstIndex() || i > l.lastIndex() {
		return true
	}
	return false
}

func (l *raftLog) isOutOfAppliedBounds(i uint64) bool {
	if i < l.firstIndex() || i > l.applied {
		return true
	}
	return false
}

func min(a, b uint64) uint64 {
	if a > b {
		return b
	}
	return a
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}
