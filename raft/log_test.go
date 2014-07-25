package raft

import (
	"reflect"
	"testing"
)

// TestAppend ensures:
// 1. If an existing entry conflicts with a new one (same index
// but different terms), delete the existing entry and all that
// follow it
// 2.Append any new entries not already in the log
func TestAppend(t *testing.T) {
	previousEnts := []Entry{{Term: 1}, {Term: 2}}
	previousUnstable := int64(3)
	tests := []struct {
		after     int64
		ents      []Entry
		windex    int64
		wents     []Entry
		wunstable int64
	}{
		{
			2,
			[]Entry{},
			2,
			[]Entry{{Term: 1}, {Term: 2}},
			3,
		},
		{
			2,
			[]Entry{{Term: 2}},
			3,
			[]Entry{{Term: 1}, {Term: 2}, {Term: 2}},
			3,
		},
		// conflicts with index 1
		{
			0,
			[]Entry{{Term: 2}},
			1,
			[]Entry{{Term: 2}},
			1,
		},
		// conflicts with index 2
		{
			1,
			[]Entry{{Term: 3}, {Term: 3}},
			3,
			[]Entry{{Term: 1}, {Term: 3}, {Term: 3}},
			2,
		},
	}

	for i, tt := range tests {
		raftLog := newLog()
		raftLog.ents = append(raftLog.ents, previousEnts...)
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

// TestCompactionSideEffects ensures that all the log related funcationality works correctly after
// a compaction.
func TestCompactionSideEffects(t *testing.T) {
	var i int64
	lastIndex := int64(1000)
	raftLog := newLog()

	for i = 0; i < lastIndex; i++ {
		raftLog.append(int64(i), Entry{Term: int64(i + 1)})
	}

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

	offset, unstableEnts := raftLog.unstableEnts()
	if offset != 501 {
		t.Errorf("offset(unstableEntries) = %d, want = %d", offset, 500)
	}
	if g := len(unstableEnts); g != 500 {
		t.Errorf("len(unstableEntries) = %d, want = %d", g, 500)
	}

	prev := raftLog.lastIndex()
	raftLog.append(raftLog.lastIndex(), Entry{Term: raftLog.lastIndex() + 1})
	if raftLog.lastIndex() != prev+1 {
		t.Errorf("lastIndex = %d, want = %d", raftLog.lastIndex(), prev+1)
	}

	ents := raftLog.entries(raftLog.lastIndex())
	if len(ents) != 1 {
		t.Errorf("len(entries) = %d, want = %d", len(ents), 1)
	}
}

func TestUnstableEnts(t *testing.T) {
	previousEnts := []Entry{{Term: 1}, {Term: 2}}
	tests := []struct {
		unstable  int64
		woffset   int64
		wents     []Entry
		wunstable int64
	}{
		{3, 3, nil, 3},
		{1, 1, []Entry{{Term: 1}, {Term: 2}}, 3},
	}

	for i, tt := range tests {
		raftLog := newLog()
		raftLog.ents = append(raftLog.ents, previousEnts...)
		raftLog.unstable = tt.unstable
		offset, ents := raftLog.unstableEnts()
		if offset != tt.woffset {
			t.Errorf("#%d: offset = %d, want = %d", i, offset, tt.woffset)
		}
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
		app     int
		compact []int64
		wleft   []int
		wallow  bool
	}{
		// out of upper bound
		{1000, []int64{1001}, []int{-1}, false},
		{1000, []int64{300, 500, 800, 900}, []int{701, 501, 201, 101}, true},
		// out of lower bound
		{1000, []int64{300, 299}, []int{701, -1}, false},
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
			for i := 0; i < tt.app; i++ {
				raftLog.append(int64(i), Entry{})
			}

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
	var i int64
	raftLog := newLog()
	for i = 0; i < 100; i++ {
		raftLog.append(i, Entry{Term: i + 1})
	}

	index := int64(1000)
	term := int64(1000)
	raftLog.restore(index, term)

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
	offset := int64(100)
	num := int64(100)
	l := &raftLog{offset: offset, ents: make([]Entry, num)}

	tests := []struct {
		index int64
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
	var i int64
	offset := int64(100)
	num := int64(100)

	l := &raftLog{offset: offset}
	for i = 0; i < num; i++ {
		l.ents = append(l.ents, Entry{Term: i})
	}

	tests := []struct {
		index int64
		w     *Entry
	}{
		{offset - 1, nil},
		{offset, &Entry{Term: 0}},
		{offset + num/2, &Entry{Term: num / 2}},
		{offset + num - 1, &Entry{Term: num - 1}},
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
	var i int64
	offset := int64(100)
	num := int64(100)

	l := &raftLog{offset: offset}
	for i = 0; i < num; i++ {
		l.ents = append(l.ents, Entry{Term: i})
	}

	tests := []struct {
		from int64
		to   int64
		w    []Entry
	}{
		{offset - 1, offset + 1, nil},
		{offset, offset + 1, []Entry{{Term: 0}}},
		{offset + num/2, offset + num/2 + 1, []Entry{{Term: num / 2}}},
		{offset + num - 1, offset + num, []Entry{{Term: num - 1}}},
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
