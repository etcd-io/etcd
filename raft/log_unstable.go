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

import pb "github.com/coreos/etcd/raft/raftpb"

// unstable.entris[i] has raft log position i+unstable.offset.
// Note that unstable.offset may be less than the highest log
// position in storage; this means that the next write to storage
// might need to truncate the log before persisting unstable.entries.
type unstable struct {
	// the incoming unstable snapshot, if any.
	snapshot *pb.Snapshot
	// all entries that have not yet been written to storage.
	entries []pb.Entry
	offset  uint64
}

// maybeFirstIndex returns the first index if it has a snapshot.
func (u *unstable) maybeFirstIndex() (uint64, bool) {
	if u.snapshot != nil {
		return u.snapshot.Metadata.Index, true
	}
	return 0, false
}

// maybeLastIndex returns the last index if it has at least one
// unstable entry or snapshot.
func (u *unstable) maybeLastIndex() (uint64, bool) {
	if l := len(u.entries); l != 0 {
		return u.offset + uint64(l) - 1, true
	}
	if u.snapshot != nil {
		return u.snapshot.Metadata.Index, true
	}
	return 0, false
}

// myabeTerm returns the term of the entry at index i, if there
// is any.
func (u *unstable) maybeTerm(i uint64) (uint64, bool) {
	if i < u.offset {
		if u.snapshot == nil {
			return 0, false
		}
		if u.snapshot.Metadata.Index == i {
			return u.snapshot.Metadata.Term, true
		}
		return 0, false
	}

	last, ok := u.maybeLastIndex()
	if !ok {
		return 0, false
	}
	if i > last {
		return 0, false
	}
	return u.entries[i-u.offset].Term, true
}

func (u *unstable) stableTo(i, t uint64) {
	if gt, ok := u.maybeTerm(i); ok {
		if gt == t && i >= u.offset {
			u.entries = u.entries[i+1-u.offset:]
			u.offset = i + 1
		}
	}
}

func (u *unstable) stableSnapTo(i uint64) {
	if u.snapshot != nil && u.snapshot.Metadata.Index == i {
		u.snapshot = nil
	}
}

func (u *unstable) restore(s pb.Snapshot) {
	u.offset = s.Metadata.Index + 1
	u.entries = nil
	u.snapshot = &s
}

func (u *unstable) resetEntries(offset uint64) {
	u.entries = nil
	u.offset = offset
}

func (u *unstable) truncateAndAppend(after uint64, ents []pb.Entry) {
	if after < u.offset {
		// The log is being truncated to before our current unstable
		// portion, so discard it and reset unstable.
		u.resetEntries(after + 1)
	}
	u.entries = append(u.slice(u.offset, after+1), ents...)
}

func (u *unstable) slice(lo uint64, hi uint64) []pb.Entry {
	if lo >= hi {
		return nil
	}
	if u.isOutOfBounds(lo) || u.isOutOfBounds(hi-1) {
		return nil
	}
	return u.entries[lo-u.offset : hi-u.offset]
}

func (u *unstable) isOutOfBounds(i uint64) bool {
	if len(u.entries) == 0 {
		return true
	}
	last := u.offset + uint64(len(u.entries)) - 1
	if i < u.offset || i > last {
		return true
	}
	return false
}
