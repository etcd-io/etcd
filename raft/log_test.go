package raft

import (
	"reflect"
	"testing"
)

func TestIsOutOfBounds(t *testing.T) {
	offset := 100
	num := 100
	l := &log{offset: offset, ents: make([]Entry, num)}

	tests := []struct {
		index int
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
	offset := 100
	num := 100

	l := &log{offset: offset}
	for i := 0; i < num; i++ {
		l.ents = append(l.ents, Entry{Term: i})
	}

	tests := []struct {
		index int
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
	offset := 100
	num := 100

	l := &log{offset: offset}
	for i := 0; i < num; i++ {
		l.ents = append(l.ents, Entry{Term: i})
	}

	tests := []struct {
		from int
		to   int
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
