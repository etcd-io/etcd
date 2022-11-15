// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/raft/v3/raftpb"
)

func TestFindConflict(t *testing.T) {
	previousEnts := []pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}
	tests := []struct {
		ents      []pb.Entry
		wconflict uint64
	}{
		// no conflict, empty ent
		{[]pb.Entry{}, 0},
		// no conflict
		{[]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}, 0},
		{[]pb.Entry{{Index: 2, Term: 2}, {Index: 3, Term: 3}}, 0},
		{[]pb.Entry{{Index: 3, Term: 3}}, 0},
		// no conflict, but has new entries
		{[]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 4}}, 4},
		{[]pb.Entry{{Index: 2, Term: 2}, {Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 4}}, 4},
		{[]pb.Entry{{Index: 3, Term: 3}, {Index: 4, Term: 4}, {Index: 5, Term: 4}}, 4},
		{[]pb.Entry{{Index: 4, Term: 4}, {Index: 5, Term: 4}}, 4},
		// conflicts with existing entries
		{[]pb.Entry{{Index: 1, Term: 4}, {Index: 2, Term: 4}}, 1},
		{[]pb.Entry{{Index: 2, Term: 1}, {Index: 3, Term: 4}, {Index: 4, Term: 4}}, 2},
		{[]pb.Entry{{Index: 3, Term: 1}, {Index: 4, Term: 2}, {Index: 5, Term: 4}, {Index: 6, Term: 4}}, 3},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			raftLog := newLog(NewMemoryStorage(), raftLogger)
			raftLog.append(previousEnts...)
			require.Equal(t, tt.wconflict, raftLog.findConflict(tt.ents))
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	previousEnts := []pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}
	raftLog := newLog(NewMemoryStorage(), raftLogger)
	raftLog.append(previousEnts...)
	tests := []struct {
		lastIndex uint64
		term      uint64
		wUpToDate bool
	}{
		// greater term, ignore lastIndex
		{raftLog.lastIndex() - 1, 4, true},
		{raftLog.lastIndex(), 4, true},
		{raftLog.lastIndex() + 1, 4, true},
		// smaller term, ignore lastIndex
		{raftLog.lastIndex() - 1, 2, false},
		{raftLog.lastIndex(), 2, false},
		{raftLog.lastIndex() + 1, 2, false},
		// equal term, equal or lager lastIndex wins
		{raftLog.lastIndex() - 1, 3, false},
		{raftLog.lastIndex(), 3, true},
		{raftLog.lastIndex() + 1, 3, true},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			require.Equal(t, tt.wUpToDate, raftLog.isUpToDate(tt.lastIndex, tt.term))
		})
	}
}

func TestAppend(t *testing.T) {
	previousEnts := []pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}
	tests := []struct {
		ents      []pb.Entry
		windex    uint64
		wents     []pb.Entry
		wunstable uint64
	}{
		{
			[]pb.Entry{},
			2,
			[]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}},
			3,
		},
		{
			[]pb.Entry{{Index: 3, Term: 2}},
			3,
			[]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 2}},
			3,
		},
		// conflicts with index 1
		{
			[]pb.Entry{{Index: 1, Term: 2}},
			1,
			[]pb.Entry{{Index: 1, Term: 2}},
			1,
		},
		// conflicts with index 2
		{
			[]pb.Entry{{Index: 2, Term: 3}, {Index: 3, Term: 3}},
			3,
			[]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 3}, {Index: 3, Term: 3}},
			2,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			storage := NewMemoryStorage()
			storage.Append(previousEnts)
			raftLog := newLog(storage, raftLogger)
			require.Equal(t, tt.windex, raftLog.append(tt.ents...))
			g, err := raftLog.entries(1, noLimit)
			require.NoError(t, err)
			require.Equal(t, tt.wents, g)
			require.Equal(t, tt.wunstable, raftLog.unstable.offset)
		})
	}
}

// TestLogMaybeAppend ensures:
// If the given (index, term) matches with the existing log:
//  1. If an existing entry conflicts with a new one (same index
//     but different terms), delete the existing entry and all that
//     follow it
//     2.Append any new entries not already in the log
//
// If the given (index, term) does not match with the existing log:
//
//	return false
func TestLogMaybeAppend(t *testing.T) {
	previousEnts := []pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}, {Index: 3, Term: 3}}
	lastindex := uint64(3)
	lastterm := uint64(3)
	commit := uint64(1)

	tests := []struct {
		logTerm   uint64
		index     uint64
		committed uint64
		ents      []pb.Entry

		wlasti  uint64
		wappend bool
		wcommit uint64
		wpanic  bool
	}{
		// not match: term is different
		{
			lastterm - 1, lastindex, lastindex, []pb.Entry{{Index: lastindex + 1, Term: 4}},
			0, false, commit, false,
		},
		// not match: index out of bound
		{
			lastterm, lastindex + 1, lastindex, []pb.Entry{{Index: lastindex + 2, Term: 4}},
			0, false, commit, false,
		},
		// match with the last existing entry
		{
			lastterm, lastindex, lastindex, nil,
			lastindex, true, lastindex, false,
		},
		{
			lastterm, lastindex, lastindex + 1, nil,
			lastindex, true, lastindex, false, // do not increase commit higher than lastnewi
		},
		{
			lastterm, lastindex, lastindex - 1, nil,
			lastindex, true, lastindex - 1, false, // commit up to the commit in the message
		},
		{
			lastterm, lastindex, 0, nil,
			lastindex, true, commit, false, // commit do not decrease
		},
		{
			0, 0, lastindex, nil,
			0, true, commit, false, // commit do not decrease
		},
		{
			lastterm, lastindex, lastindex, []pb.Entry{{Index: lastindex + 1, Term: 4}},
			lastindex + 1, true, lastindex, false,
		},
		{
			lastterm, lastindex, lastindex + 1, []pb.Entry{{Index: lastindex + 1, Term: 4}},
			lastindex + 1, true, lastindex + 1, false,
		},
		{
			lastterm, lastindex, lastindex + 2, []pb.Entry{{Index: lastindex + 1, Term: 4}},
			lastindex + 1, true, lastindex + 1, false, // do not increase commit higher than lastnewi
		},
		{
			lastterm, lastindex, lastindex + 2, []pb.Entry{{Index: lastindex + 1, Term: 4}, {Index: lastindex + 2, Term: 4}},
			lastindex + 2, true, lastindex + 2, false,
		},
		// match with the entry in the middle
		{
			lastterm - 1, lastindex - 1, lastindex, []pb.Entry{{Index: lastindex, Term: 4}},
			lastindex, true, lastindex, false,
		},
		{
			lastterm - 2, lastindex - 2, lastindex, []pb.Entry{{Index: lastindex - 1, Term: 4}},
			lastindex - 1, true, lastindex - 1, false,
		},
		{
			lastterm - 3, lastindex - 3, lastindex, []pb.Entry{{Index: lastindex - 2, Term: 4}},
			lastindex - 2, true, lastindex - 2, true, // conflict with existing committed entry
		},
		{
			lastterm - 2, lastindex - 2, lastindex, []pb.Entry{{Index: lastindex - 1, Term: 4}, {Index: lastindex, Term: 4}},
			lastindex, true, lastindex, false,
		},
	}

	for i, tt := range tests {
		raftLog := newLog(NewMemoryStorage(), raftLogger)
		raftLog.append(previousEnts...)
		raftLog.committed = commit

		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					require.True(t, tt.wpanic)
				}
			}()
			glasti, gappend := raftLog.maybeAppend(tt.index, tt.logTerm, tt.committed, tt.ents...)
			require.Equal(t, tt.wlasti, glasti)
			require.Equal(t, tt.wappend, gappend)
			require.Equal(t, tt.wcommit, raftLog.committed)
			if gappend && len(tt.ents) != 0 {
				gents, err := raftLog.slice(raftLog.lastIndex()-uint64(len(tt.ents))+1, raftLog.lastIndex()+1, noLimit)
				require.NoError(t, err)
				require.Equal(t, tt.ents, gents)
			}
		})
	}
}

// TestCompactionSideEffects ensures that all the log related functionality works correctly after
// a compaction.
func TestCompactionSideEffects(t *testing.T) {
	var i uint64
	// Populate the log with 1000 entries; 750 in stable storage and 250 in unstable.
	lastIndex := uint64(1000)
	unstableIndex := uint64(750)
	lastTerm := lastIndex
	storage := NewMemoryStorage()
	for i = 1; i <= unstableIndex; i++ {
		storage.Append([]pb.Entry{{Term: i, Index: i}})
	}
	raftLog := newLog(storage, raftLogger)
	for i = unstableIndex; i < lastIndex; i++ {
		raftLog.append(pb.Entry{Term: i + 1, Index: i + 1})
	}

	require.True(t, raftLog.maybeCommit(lastIndex, lastTerm))
	raftLog.appliedTo(raftLog.committed)

	offset := uint64(500)
	storage.Compact(offset)
	require.Equal(t, lastIndex, raftLog.lastIndex())

	for j := offset; j <= raftLog.lastIndex(); j++ {
		require.Equal(t, j, mustTerm(raftLog.term(j)))
	}

	for j := offset; j <= raftLog.lastIndex(); j++ {
		require.True(t, raftLog.matchTerm(j, j))
	}

	unstableEnts := raftLog.unstableEntries()
	require.Equal(t, 250, len(unstableEnts))
	require.Equal(t, uint64(751), unstableEnts[0].Index)

	prev := raftLog.lastIndex()
	raftLog.append(pb.Entry{Index: raftLog.lastIndex() + 1, Term: raftLog.lastIndex() + 1})
	require.Equal(t, prev+1, raftLog.lastIndex())

	ents, err := raftLog.entries(raftLog.lastIndex(), noLimit)
	require.NoError(t, err)
	require.Equal(t, 1, len(ents))
}

func TestHasNextCommittedEnts(t *testing.T) {
	snap := pb.Snapshot{
		Metadata: pb.SnapshotMetadata{Term: 1, Index: 3},
	}
	ents := []pb.Entry{
		{Term: 1, Index: 4},
		{Term: 1, Index: 5},
		{Term: 1, Index: 6},
	}
	tests := []struct {
		applied  uint64
		snap     bool
		whasNext bool
	}{
		{applied: 0, snap: false, whasNext: true},
		{applied: 3, snap: false, whasNext: true},
		{applied: 4, snap: false, whasNext: true},
		{applied: 5, snap: false, whasNext: false},
		// With snapshot.
		{applied: 3, snap: true, whasNext: false},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			storage := NewMemoryStorage()
			require.NoError(t, storage.ApplySnapshot(snap))

			raftLog := newLog(storage, raftLogger)
			raftLog.append(ents...)
			raftLog.maybeCommit(5, 1)
			raftLog.appliedTo(tt.applied)
			if tt.snap {
				newSnap := snap
				newSnap.Metadata.Index++
				raftLog.restore(newSnap)
			}
			require.Equal(t, tt.whasNext, raftLog.hasNextCommittedEnts())
		})
	}
}

func TestNextCommittedEnts(t *testing.T) {
	snap := pb.Snapshot{
		Metadata: pb.SnapshotMetadata{Term: 1, Index: 3},
	}
	ents := []pb.Entry{
		{Term: 1, Index: 4},
		{Term: 1, Index: 5},
		{Term: 1, Index: 6},
	}
	tests := []struct {
		applied uint64
		snap    bool
		wents   []pb.Entry
	}{
		{applied: 0, snap: false, wents: ents[:2]},
		{applied: 3, snap: false, wents: ents[:2]},
		{applied: 4, snap: false, wents: ents[1:2]},
		{applied: 5, snap: false, wents: nil},
		// With snapshot.
		{applied: 3, snap: true, wents: nil},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			storage := NewMemoryStorage()
			require.NoError(t, storage.ApplySnapshot(snap))

			raftLog := newLog(storage, raftLogger)
			raftLog.append(ents...)
			raftLog.maybeCommit(5, 1)
			raftLog.appliedTo(tt.applied)
			if tt.snap {
				newSnap := snap
				newSnap.Metadata.Index++
				raftLog.restore(newSnap)
			}
			require.Equal(t, tt.wents, raftLog.nextCommittedEnts())
		})

	}
}

// TestUnstableEnts ensures unstableEntries returns the unstable part of the
// entries correctly.
func TestUnstableEnts(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1, Index: 1}, {Term: 2, Index: 2}}
	tests := []struct {
		unstable uint64
		wents    []pb.Entry
	}{
		{3, nil},
		{1, previousEnts},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			// append stable entries to storage
			storage := NewMemoryStorage()
			require.NoError(t, storage.Append(previousEnts[:tt.unstable-1]))

			// append unstable entries to raftlog
			raftLog := newLog(storage, raftLogger)
			raftLog.append(previousEnts[tt.unstable-1:]...)

			ents := raftLog.unstableEntries()
			if l := len(ents); l > 0 {
				raftLog.stableTo(ents[l-1].Index, ents[l-1].Term)
			}
			require.Equal(t, tt.wents, ents)
			require.Equal(t, previousEnts[len(previousEnts)-1].Index+1, raftLog.unstable.offset)
		})
	}
}

func TestCommitTo(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1, Index: 1}, {Term: 2, Index: 2}, {Term: 3, Index: 3}}
	commit := uint64(2)
	tests := []struct {
		commit  uint64
		wcommit uint64
		wpanic  bool
	}{
		{3, 3, false},
		{1, 2, false}, // never decrease
		{4, 0, true},  // commit out of range -> panic
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					require.True(t, tt.wpanic)
				}
			}()
			raftLog := newLog(NewMemoryStorage(), raftLogger)
			raftLog.append(previousEnts...)
			raftLog.committed = commit
			raftLog.commitTo(tt.commit)
			require.Equal(t, tt.wcommit, raftLog.committed)
		})
	}
}

func TestStableTo(t *testing.T) {
	tests := []struct {
		stablei   uint64
		stablet   uint64
		wunstable uint64
	}{
		{1, 1, 2},
		{2, 2, 3},
		{2, 1, 1}, // bad term
		{3, 1, 1}, // bad index
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			raftLog := newLog(NewMemoryStorage(), raftLogger)
			raftLog.append([]pb.Entry{{Index: 1, Term: 1}, {Index: 2, Term: 2}}...)
			raftLog.stableTo(tt.stablei, tt.stablet)
			require.Equal(t, tt.wunstable, raftLog.unstable.offset)
		})
	}
}

func TestStableToWithSnap(t *testing.T) {
	snapi, snapt := uint64(5), uint64(2)
	tests := []struct {
		stablei uint64
		stablet uint64
		newEnts []pb.Entry

		wunstable uint64
	}{
		{snapi + 1, snapt, nil, snapi + 1},
		{snapi, snapt, nil, snapi + 1},
		{snapi - 1, snapt, nil, snapi + 1},

		{snapi + 1, snapt + 1, nil, snapi + 1},
		{snapi, snapt + 1, nil, snapi + 1},
		{snapi - 1, snapt + 1, nil, snapi + 1},

		{snapi + 1, snapt, []pb.Entry{{Index: snapi + 1, Term: snapt}}, snapi + 2},
		{snapi, snapt, []pb.Entry{{Index: snapi + 1, Term: snapt}}, snapi + 1},
		{snapi - 1, snapt, []pb.Entry{{Index: snapi + 1, Term: snapt}}, snapi + 1},

		{snapi + 1, snapt + 1, []pb.Entry{{Index: snapi + 1, Term: snapt}}, snapi + 1},
		{snapi, snapt + 1, []pb.Entry{{Index: snapi + 1, Term: snapt}}, snapi + 1},
		{snapi - 1, snapt + 1, []pb.Entry{{Index: snapi + 1, Term: snapt}}, snapi + 1},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			s := NewMemoryStorage()
			require.NoError(t, s.ApplySnapshot(pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: snapi, Term: snapt}}))
			raftLog := newLog(s, raftLogger)
			raftLog.append(tt.newEnts...)
			raftLog.stableTo(tt.stablei, tt.stablet)
			require.Equal(t, tt.wunstable, raftLog.unstable.offset)
		})

	}
}

// TestCompaction ensures that the number of log entries is correct after compactions.
func TestCompaction(t *testing.T) {
	tests := []struct {
		lastIndex uint64
		compact   []uint64
		wleft     []int
		wallow    bool
	}{
		// out of upper bound
		{1000, []uint64{1001}, []int{-1}, false},
		{1000, []uint64{300, 500, 800, 900}, []int{700, 500, 200, 100}, true},
		// out of lower bound
		{1000, []uint64{300, 299}, []int{700, -1}, false},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					require.False(t, tt.wallow)
				}
			}()
			storage := NewMemoryStorage()
			for i := uint64(1); i <= tt.lastIndex; i++ {
				storage.Append([]pb.Entry{{Index: i}})
			}
			raftLog := newLog(storage, raftLogger)
			raftLog.maybeCommit(tt.lastIndex, 0)

			raftLog.appliedTo(raftLog.committed)
			for j := 0; j < len(tt.compact); j++ {
				err := storage.Compact(tt.compact[j])
				if err != nil {
					require.False(t, tt.wallow)
					continue
				}
				require.Equal(t, tt.wleft[j], len(raftLog.allEntries()))
			}

		})
	}
}

func TestLogRestore(t *testing.T) {
	index := uint64(1000)
	term := uint64(1000)
	snap := pb.SnapshotMetadata{Index: index, Term: term}
	storage := NewMemoryStorage()
	storage.ApplySnapshot(pb.Snapshot{Metadata: snap})
	raftLog := newLog(storage, raftLogger)

	require.Zero(t, len(raftLog.allEntries()))
	require.Equal(t, index+1, raftLog.firstIndex())
	require.Equal(t, index, raftLog.committed)
	require.Equal(t, index+1, raftLog.unstable.offset)
	require.Equal(t, term, mustTerm(raftLog.term(index)))
}

func TestIsOutOfBounds(t *testing.T) {
	offset := uint64(100)
	num := uint64(100)
	storage := NewMemoryStorage()
	storage.ApplySnapshot(pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: offset}})
	l := newLog(storage, raftLogger)
	for i := uint64(1); i <= num; i++ {
		l.append(pb.Entry{Index: i + offset})
	}

	first := offset + 1
	tests := []struct {
		lo, hi        uint64
		wpanic        bool
		wErrCompacted bool
	}{
		{
			first - 2, first + 1,
			false,
			true,
		},
		{
			first - 1, first + 1,
			false,
			true,
		},
		{
			first, first,
			false,
			false,
		},
		{
			first + num/2, first + num/2,
			false,
			false,
		},
		{
			first + num - 1, first + num - 1,
			false,
			false,
		},
		{
			first + num, first + num,
			false,
			false,
		},
		{
			first + num, first + num + 1,
			true,
			false,
		},
		{
			first + num + 1, first + num + 1,
			true,
			false,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					require.True(t, tt.wpanic)
				}
			}()
			err := l.mustCheckOutOfBounds(tt.lo, tt.hi)
			require.False(t, tt.wpanic)
			require.False(t, tt.wErrCompacted && err != ErrCompacted)
			require.False(t, !tt.wErrCompacted && err != nil)
		})
	}
}

func TestTerm(t *testing.T) {
	var i uint64
	offset := uint64(100)
	num := uint64(100)

	storage := NewMemoryStorage()
	storage.ApplySnapshot(pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: offset, Term: 1}})
	l := newLog(storage, raftLogger)
	for i = 1; i < num; i++ {
		l.append(pb.Entry{Index: offset + i, Term: i})
	}

	tests := []struct {
		index uint64
		w     uint64
	}{
		{offset - 1, 0},
		{offset, 1},
		{offset + num/2, num / 2},
		{offset + num - 1, num - 1},
		{offset + num, 0},
	}

	for j, tt := range tests {
		t.Run(fmt.Sprint(j), func(t *testing.T) {
			require.Equal(t, tt.w, mustTerm(l.term(tt.index)))
		})
	}
}

func TestTermWithUnstableSnapshot(t *testing.T) {
	storagesnapi := uint64(100)
	unstablesnapi := storagesnapi + 5

	storage := NewMemoryStorage()
	storage.ApplySnapshot(pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: storagesnapi, Term: 1}})
	l := newLog(storage, raftLogger)
	l.restore(pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: unstablesnapi, Term: 1}})

	tests := []struct {
		index uint64
		w     uint64
	}{
		// cannot get term from storage
		{storagesnapi, 0},
		// cannot get term from the gap between storage ents and unstable snapshot
		{storagesnapi + 1, 0},
		{unstablesnapi - 1, 0},
		// get term from unstable snapshot index
		{unstablesnapi, 1},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			require.Equal(t, tt.w, mustTerm(l.term(tt.index)))
		})
	}
}

func TestSlice(t *testing.T) {
	var i uint64
	offset := uint64(100)
	num := uint64(100)
	last := offset + num
	half := offset + num/2
	halfe := pb.Entry{Index: half, Term: half}

	storage := NewMemoryStorage()
	storage.ApplySnapshot(pb.Snapshot{Metadata: pb.SnapshotMetadata{Index: offset}})
	for i = 1; i < num/2; i++ {
		storage.Append([]pb.Entry{{Index: offset + i, Term: offset + i}})
	}
	l := newLog(storage, raftLogger)
	for i = num / 2; i < num; i++ {
		l.append(pb.Entry{Index: offset + i, Term: offset + i})
	}

	tests := []struct {
		from  uint64
		to    uint64
		limit uint64

		w      []pb.Entry
		wpanic bool
	}{
		// test no limit
		{offset - 1, offset + 1, noLimit, nil, false},
		{offset, offset + 1, noLimit, nil, false},
		{half - 1, half + 1, noLimit, []pb.Entry{{Index: half - 1, Term: half - 1}, {Index: half, Term: half}}, false},
		{half, half + 1, noLimit, []pb.Entry{{Index: half, Term: half}}, false},
		{last - 1, last, noLimit, []pb.Entry{{Index: last - 1, Term: last - 1}}, false},
		{last, last + 1, noLimit, nil, true},

		// test limit
		{half - 1, half + 1, 0, []pb.Entry{{Index: half - 1, Term: half - 1}}, false},
		{half - 1, half + 1, uint64(halfe.Size() + 1), []pb.Entry{{Index: half - 1, Term: half - 1}}, false},
		{half - 2, half + 1, uint64(halfe.Size() + 1), []pb.Entry{{Index: half - 2, Term: half - 2}}, false},
		{half - 1, half + 1, uint64(halfe.Size() * 2), []pb.Entry{{Index: half - 1, Term: half - 1}, {Index: half, Term: half}}, false},
		{half - 1, half + 2, uint64(halfe.Size() * 3), []pb.Entry{{Index: half - 1, Term: half - 1}, {Index: half, Term: half}, {Index: half + 1, Term: half + 1}}, false},
		{half, half + 2, uint64(halfe.Size()), []pb.Entry{{Index: half, Term: half}}, false},
		{half, half + 2, uint64(halfe.Size() * 2), []pb.Entry{{Index: half, Term: half}, {Index: half + 1, Term: half + 1}}, false},
	}

	for j, tt := range tests {
		t.Run(fmt.Sprint(j), func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					require.True(t, tt.wpanic)
				}
			}()
			g, err := l.slice(tt.from, tt.to, tt.limit)
			require.False(t, tt.from <= offset && err != ErrCompacted)
			require.False(t, tt.from > offset && err != nil)
			require.Equal(t, tt.w, g)
		})
	}
}

func mustTerm(term uint64, err error) uint64 {
	if err != nil {
		panic(err)
	}
	return term
}
