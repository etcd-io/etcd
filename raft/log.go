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
	applied uint64
}

// newLog returns log using the given storage. It recovers the log to the state
// that it just commits and applies the lastest snapshot.
func newLog(storage Storage) *raftLog {
	if storage == nil {
		log.Panic("storage must not be nil")
	}
	log := &raftLog{
		storage: storage,
	}
	firstIndex, err := storage.FirstIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	lastIndex, err := storage.LastIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	log.unstable = lastIndex + 1
	// Initialize our committed and applied pointers to the time of the last compaction.
	log.committed = firstIndex - 1
	log.applied = firstIndex - 1

	return log
}

func (l *raftLog) String() string {
	return fmt.Sprintf("unstable=%d committed=%d applied=%d len(unstableEntries)=%d", l.unstable, l.committed, l.applied, len(l.unstableEnts))
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
			log.Panicf("conflict(%d) with committed entry [committed(%d)]", ci, l.committed)
		default:
			l.append(ci-1, ents[ci-from:]...)
		}
		l.commitTo(min(committed, lastnewi))
		return lastnewi, true
	}
	return 0, false
}

func (l *raftLog) append(after uint64, ents ...pb.Entry) uint64 {
	if after < l.committed {
		log.Panicf("after(%d) out of range [committed(%d)]", after, l.committed)
	}
	if after < l.unstable {
		// The log is being truncated to before our current unstable
		// portion, so discard it and reset unstable.
		l.unstableEnts = nil
		l.unstable = after + 1
	}
	// Truncate any unstable entries that are being replaced, then
	// append the new ones.
	l.unstableEnts = append(l.unstableEnts[:after+1-l.unstable], ents...)
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
	// copy unstable entries to an empty slice
	return append([]pb.Entry{}, l.unstableEnts...)
}

// nextEnts returns all the available entries for execution.
// If applied is smaller than the index of snapshot, it returns all committed
// entries after the index of snapshot.
func (l *raftLog) nextEnts() (ents []pb.Entry) {
	off := max(l.applied+1, l.firstIndex())
	if l.committed+1 > off {
		return l.slice(off, l.committed+1)
	}
	return nil
}

func (l *raftLog) firstIndex() uint64 {
	index, err := l.storage.FirstIndex()
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	return index
}

func (l *raftLog) lastIndex() uint64 {
	return l.unstable + uint64(len(l.unstableEnts)) - 1
}

func (l *raftLog) commitTo(tocommit uint64) {
	// never decrease commit
	if l.committed < tocommit {
		if l.lastIndex() < tocommit {
			log.Panicf("tocommit(%d) is out of range [lastIndex(%d)]", tocommit, l.lastIndex())
		}
		l.committed = tocommit
	}
}

func (l *raftLog) appliedTo(i uint64) {
	if i == 0 {
		return
	}
	if l.committed < i || i < l.applied {
		log.Panicf("applied(%d) is out of range [prevApplied(%d), committed(%d)]", i, l.applied, l.committed)
	}
	l.applied = i
}

func (l *raftLog) stableTo(i uint64) {
	if i < l.unstable || i+1-l.unstable > uint64(len(l.unstableEnts)) {
		log.Panicf("stableTo(%d) is out of range [unstable(%d), len(unstableEnts)(%d)]",
			i, l.unstable, len(l.unstableEnts))
	}
	l.unstableEnts = l.unstableEnts[i+1-l.unstable:]
	l.unstable = i + 1
}

func (l *raftLog) lastTerm() uint64 {
	return l.term(l.lastIndex())
}

func (l *raftLog) term(i uint64) uint64 {
	switch {
	case i > l.lastIndex():
		return 0
	case i < l.unstable:
		t, err := l.storage.Term(i)
		switch err {
		case nil:
			return t
		case ErrCompacted:
			return 0
		default:
			panic(err) // TODO(bdarnell)
		}
	default:
		return l.unstableEnts[i-l.unstable].Term
	}
}

func (l *raftLog) entries(i uint64) []pb.Entry {
	return l.slice(i, l.lastIndex()+1)
}

// allEntries returns all entries in the log.
func (l *raftLog) allEntries() []pb.Entry {
	return l.entries(l.firstIndex())
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
	return l.term(i) == term
}

func (l *raftLog) maybeCommit(maxIndex, term uint64) bool {
	if maxIndex > l.committed && l.term(maxIndex) == term {
		l.commitTo(maxIndex)
		return true
	}
	return false
}

func (l *raftLog) restore(s pb.Snapshot) {
	err := l.storage.ApplySnapshot(s)
	if err != nil {
		panic(err) // TODO(bdarnell)
	}
	l.committed = s.Metadata.Index
	l.applied = s.Metadata.Index
	l.unstable = l.committed + 1
	l.unstableEnts = nil
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
		storedEnts, err := l.storage.Entries(lo, min(hi, l.unstable))
		if err == ErrCompacted {
			return nil
		} else if err != nil {
			panic(err) // TODO(bdarnell)
		}
		ents = append(ents, storedEnts...)
	}
	if len(l.unstableEnts) > 0 && hi > l.unstable {
		firstUnstable := max(lo, l.unstable)
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
