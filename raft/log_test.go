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
	"reflect"
	"testing"

	pb "github.com/coreos/etcd/raft/raftpb"
)

func TestFindConflict(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1}, {Term: 2}, {Term: 3}}
	tests := []struct {
		from      uint64
		ents      []pb.Entry
		wconflict uint64
	}{
		// no conflict, empty ent
		{1, []pb.Entry{}, 0},
		{3, []pb.Entry{}, 0},
		// no conflict
		{1, []pb.Entry{{Term: 1}, {Term: 2}, {Term: 3}}, 0},
		{2, []pb.Entry{{Term: 2}, {Term: 3}}, 0},
		{3, []pb.Entry{{Term: 3}}, 0},
		// no conflict, but has new entries
		{1, []pb.Entry{{Term: 1}, {Term: 2}, {Term: 3}, {Term: 4}, {Term: 4}}, 4},
		{2, []pb.Entry{{Term: 2}, {Term: 3}, {Term: 4}, {Term: 4}}, 4},
		{3, []pb.Entry{{Term: 3}, {Term: 4}, {Term: 4}}, 4},
		{4, []pb.Entry{{Term: 4}, {Term: 4}}, 4},
		// conflicts with existing entries
		{1, []pb.Entry{{Term: 4}, {Term: 4}}, 1},
		{2, []pb.Entry{{Term: 1}, {Term: 4}, {Term: 4}}, 2},
		{3, []pb.Entry{{Term: 1}, {Term: 2}, {Term: 4}, {Term: 4}}, 3},
	}

	for i, tt := range tests {
		raftLog := newLog()
		raftLog.append(raftLog.lastIndex(), previousEnts...)

		gconflict := raftLog.findConflict(tt.from, tt.ents)
		if gconflict != tt.wconflict {
			t.Errorf("#%d: conflict = %d, want %d", i, gconflict, tt.wconflict)
		}
	}
}

func TestIsUpToDate(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1}, {Term: 2}, {Term: 3}}
	raftLog := newLog()
	raftLog.append(raftLog.lastIndex(), previousEnts...)
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
		// equal term, lager lastIndex wins
		{raftLog.lastIndex() - 1, 3, false},
		{raftLog.lastIndex(), 3, true},
		{raftLog.lastIndex() + 1, 3, true},
	}

	for i, tt := range tests {
		gUpToDate := raftLog.isUpToDate(tt.lastIndex, tt.term)
		if gUpToDate != tt.wUpToDate {
			t.Errorf("#%d: uptodate = %v, want %v", i, gUpToDate, tt.wUpToDate)
		}
	}
}

func TestAppend(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1}, {Term: 2}}
	previousUnstable := uint64(3)
	tests := []struct {
		after     uint64
		ents      []pb.Entry
		windex    uint64
		wents     []pb.Entry
		wunstable uint64
	}{
		{
			2,
			[]pb.Entry{},
			2,
			[]pb.Entry{{Term: 1}, {Term: 2}},
			3,
		},
		{
			2,
			[]pb.Entry{{Term: 2}},
			3,
			[]pb.Entry{{Term: 1}, {Term: 2}, {Term: 2}},
			3,
		},
		// conflicts with index 1
		{
			0,
			[]pb.Entry{{Term: 2}},
			1,
			[]pb.Entry{{Term: 2}},
			1,
		},
		// conflicts with index 2
		{
			1,
			[]pb.Entry{{Term: 3}, {Term: 3}},
			3,
			[]pb.Entry{{Term: 1}, {Term: 3}, {Term: 3}},
			2,
		},
	}

	for i, tt := range tests {
		raftLog := newLog()
		raftLog.append(raftLog.lastIndex(), previousEnts...)
		raftLog.unstable = previousUnstable
		index := raftLog.append(tt.after, tt.ents...)
		if index != tt.windex {
			t.Errorf("#%d: lastIndex = %d, want %d", i, index, tt.windex)
		}
		if g := raftLog.entries(1); !reflect.DeepEqual(g, tt.wents) {
			t.Errorf("#%d: logEnts = %+v, want %+v", i, g, tt.wents)
		}
		if g := raftLog.unstable; g != tt.wunstable {
			t.Errorf("#%d: unstable = %d, want %d", i, g, tt.wunstable)
		}
	}
}

// TestLogMaybeAppend ensures:
// If the given (index, term) matches with the existing log:
// 	1. If an existing entry conflicts with a new one (same index
// 	but different terms), delete the existing entry and all that
// 	follow it
// 	2.Append any new entries not already in the log
// If the given (index, term) does not match with the existing log:
// 	return false
func TestLogMaybeAppend(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1}, {Term: 2}, {Term: 3}}
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
			lastterm - 1, lastindex, lastindex, []pb.Entry{{Term: 4}},
			0, false, commit, false,
		},
		// not match: index out of bound
		{
			lastterm, lastindex + 1, lastindex, []pb.Entry{{Term: 4}},
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
			lastterm, lastindex, lastindex, []pb.Entry{{Term: 4}},
			lastindex + 1, true, lastindex, false,
		},
		{
			lastterm, lastindex, lastindex + 1, []pb.Entry{{Term: 4}},
			lastindex + 1, true, lastindex + 1, false,
		},
		{
			lastterm, lastindex, lastindex + 2, []pb.Entry{{Term: 4}},
			lastindex + 1, true, lastindex + 1, false, // do not increase commit higher than lastnewi
		},
		{
			lastterm, lastindex, lastindex + 2, []pb.Entry{{Term: 4}, {Term: 4}},
			lastindex + 2, true, lastindex + 2, false,
		},
		// match with the the entry in the middle
		{
			lastterm - 1, lastindex - 1, lastindex, []pb.Entry{{Term: 4}},
			lastindex, true, lastindex, false,
		},
		{
			lastterm - 2, lastindex - 2, lastindex, []pb.Entry{{Term: 4}},
			lastindex - 1, true, lastindex - 1, false,
		},
		{
			lastterm - 3, lastindex - 3, lastindex, []pb.Entry{{Term: 4}},
			lastindex - 2, true, lastindex - 2, true, // conflict with existing committed entry
		},
		{
			lastterm - 2, lastindex - 2, lastindex, []pb.Entry{{Term: 4}, {Term: 4}},
			lastindex, true, lastindex, false,
		},
	}

	for i, tt := range tests {
		raftLog := newLog()
		raftLog.append(raftLog.lastIndex(), previousEnts...)
		raftLog.committed = commit
		func() {
			defer func() {
				if r := recover(); r != nil {
					if tt.wpanic != true {
						t.Errorf("%d: panic = %v, want %v", i, true, tt.wpanic)
					}
				}
			}()
			glasti, gappend := raftLog.maybeAppend(tt.index, tt.logTerm, tt.committed, tt.ents...)
			gcommit := raftLog.committed

			if glasti != tt.wlasti {
				t.Errorf("#%d: lastindex = %d, want %d", i, glasti, tt.wlasti)
			}
			if gappend != tt.wappend {
				t.Errorf("#%d: append = %v, want %v", i, gappend, tt.wappend)
			}
			if gcommit != tt.wcommit {
				t.Errorf("#%d: committed = %d, want %d", i, gcommit, tt.wcommit)
			}
			if gappend {
				gents := raftLog.slice(raftLog.lastIndex()-uint64(len(tt.ents))+1, raftLog.lastIndex()+1)
				if !reflect.DeepEqual(tt.ents, gents) {
					t.Errorf("%d: appended entries = %v, want %v", i, gents, tt.ents)
				}
			}
		}()
	}
}

// TestCompactionSideEffects ensures that all the log related funcationality works correctly after
// a compaction.
func TestCompactionSideEffects(t *testing.T) {
	var i uint64
	lastIndex := uint64(1000)
	lastTerm := lastIndex
	raftLog := newLog()

	for i = 0; i < lastIndex; i++ {
		raftLog.append(uint64(i), pb.Entry{Term: uint64(i + 1), Index: uint64(i + 1)})
	}
	raftLog.maybeCommit(lastIndex, lastTerm)
	raftLog.appliedTo(raftLog.committed)

	raftLog.compact(500)

	if raftLog.lastIndex() != lastIndex {
		t.Errorf("lastIndex = %d, want %d", raftLog.lastIndex(), lastIndex)
	}

	for i := raftLog.offset; i <= raftLog.lastIndex(); i++ {
		if raftLog.term(i) != i {
			t.Errorf("term(%d) = %d, want %d", i, raftLog.term(i), i)
		}
	}

	for i := raftLog.offset; i <= raftLog.lastIndex(); i++ {
		if !raftLog.matchTerm(i, i) {
			t.Errorf("matchTerm(%d) = false, want true", i)
		}
	}

	unstableEnts := raftLog.unstableEnts()
	if g := len(unstableEnts); g != 500 {
		t.Errorf("len(unstableEntries) = %d, want = %d", g, 500)
	}
	if unstableEnts[0].Index != 501 {
		t.Errorf("Index = %d, want = %d", unstableEnts[0].Index, 501)
	}

	prev := raftLog.lastIndex()
	raftLog.append(raftLog.lastIndex(), pb.Entry{Term: raftLog.lastIndex() + 1})
	if raftLog.lastIndex() != prev+1 {
		t.Errorf("lastIndex = %d, want = %d", raftLog.lastIndex(), prev+1)
	}

	ents := raftLog.entries(raftLog.lastIndex())
	if len(ents) != 1 {
		t.Errorf("len(entries) = %d, want = %d", len(ents), 1)
	}
}

func TestUnstableEnts(t *testing.T) {
	previousEnts := []pb.Entry{{Term: 1, Index: 1}, {Term: 2, Index: 2}}
	tests := []struct {
		unstable  uint64
		wents     []pb.Entry
		wunstable uint64
	}{
		{3, nil, 3},
		{1, previousEnts, 3},
	}

	for i, tt := range tests {
		raftLog := newLog()
		raftLog.append(0, previousEnts...)
		raftLog.unstable = tt.unstable
		ents := raftLog.unstableEnts()
		raftLog.stableTo(raftLog.lastIndex())
		if !reflect.DeepEqual(ents, tt.wents) {
			t.Errorf("#%d: unstableEnts = %+v, want %+v", i, ents, tt.wents)
		}
		if g := raftLog.unstable; g != tt.wunstable {
			t.Errorf("#%d: unstable = %d, want %d", i, g, tt.wunstable)
		}
	}
}

//TestCompaction ensures that the number of log entreis is correct after compactions.
func TestCompaction(t *testing.T) {
	tests := []struct {
		applied   uint64
		lastIndex uint64
		compact   []uint64
		wleft     []int
		wallow    bool
	}{
		// out of upper bound
		{1000, 1000, []uint64{1001}, []int{-1}, false},
		{1000, 1000, []uint64{300, 500, 800, 900}, []int{701, 501, 201, 101}, true},
		// out of lower bound
		{1000, 1000, []uint64{300, 299}, []int{701, -1}, false},
		{0, 1000, []uint64{1}, []int{-1}, false},
	}

	for i, tt := range tests {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if tt.wallow == true {
						t.Errorf("%d: allow = %v, want %v", i, false, true)
					}
				}
			}()

			raftLog := newLog()
			for i := uint64(0); i < tt.lastIndex; i++ {
				raftLog.append(uint64(i), pb.Entry{})
			}
			raftLog.maybeCommit(tt.applied, 0)
			raftLog.appliedTo(raftLog.committed)

			for j := 0; j < len(tt.compact); j++ {
				raftLog.compact(tt.compact[j])
				if len(raftLog.ents) != tt.wleft[j] {
					t.Errorf("#%d.%d len = %d, want %d", i, j, len(raftLog.ents), tt.wleft[j])
				}
			}
		}()
	}
}

func TestLogRestore(t *testing.T) {
	var i uint64
	raftLog := newLog()
	for i = 0; i < 100; i++ {
		raftLog.append(i, pb.Entry{Term: i + 1})
	}

	index := uint64(1000)
	term := uint64(1000)
	raftLog.restore(pb.Snapshot{Index: index, Term: term})

	// only has the guard entry
	if len(raftLog.ents) != 1 {
		t.Errorf("len = %d, want 0", len(raftLog.ents))
	}
	if raftLog.offset != index {
		t.Errorf("offset = %d, want %d", raftLog.offset, index)
	}
	if raftLog.applied != index {
		t.Errorf("applied = %d, want %d", raftLog.applied, index)
	}
	if raftLog.committed != index {
		t.Errorf("comitted = %d, want %d", raftLog.committed, index)
	}
	if raftLog.unstable != index+1 {
		t.Errorf("unstable = %d, want %d", raftLog.unstable, index+1)
	}
	if raftLog.term(index) != term {
		t.Errorf("term = %d, want %d", raftLog.term(index), term)
	}
}

func TestIsOutOfBounds(t *testing.T) {
	offset := uint64(100)
	num := uint64(100)
	l := &raftLog{offset: offset, ents: make([]pb.Entry, num)}

	tests := []struct {
		index uint64
		w     bool
	}{
		{offset - 1, true},
		{offset, false},
		{offset + num/2, false},
		{offset + num - 1, false},
		{offset + num, true},
	}

	for i, tt := range tests {
		g := l.isOutOfBounds(tt.index)
		if g != tt.w {
			t.Errorf("#%d: isOutOfBounds = %v, want %v", i, g, tt.w)
		}
	}
}

func TestAt(t *testing.T) {
	var i uint64
	offset := uint64(100)
	num := uint64(100)

	l := &raftLog{offset: offset}
	for i = 0; i < num; i++ {
		l.ents = append(l.ents, pb.Entry{Term: i})
	}

	tests := []struct {
		index uint64
		w     *pb.Entry
	}{
		{offset - 1, nil},
		{offset, &pb.Entry{Term: 0}},
		{offset + num/2, &pb.Entry{Term: num / 2}},
		{offset + num - 1, &pb.Entry{Term: num - 1}},
		{offset + num, nil},
	}

	for i, tt := range tests {
		g := l.at(tt.index)
		if !reflect.DeepEqual(g, tt.w) {
			t.Errorf("#%d: at = %v, want %v", i, g, tt.w)
		}
	}
}

func TestSlice(t *testing.T) {
	var i uint64
	offset := uint64(100)
	num := uint64(100)

	l := &raftLog{offset: offset}
	for i = 0; i < num; i++ {
		l.ents = append(l.ents, pb.Entry{Term: i})
	}

	tests := []struct {
		from uint64
		to   uint64
		w    []pb.Entry
	}{
		{offset - 1, offset + 1, nil},
		{offset, offset + 1, []pb.Entry{{Term: 0}}},
		{offset + num/2, offset + num/2 + 1, []pb.Entry{{Term: num / 2}}},
		{offset + num - 1, offset + num, []pb.Entry{{Term: num - 1}}},
		{offset + num, offset + num + 1, nil},

		{offset + num/2, offset + num/2, nil},
		{offset + num/2, offset + num/2 - 1, nil},
	}

	for i, tt := range tests {
		g := l.slice(tt.from, tt.to)
		if !reflect.DeepEqual(g, tt.w) {
			t.Errorf("#%d: from %d to %d = %v, want %v", i, tt.from, tt.to, g, tt.w)
		}
	}
}
