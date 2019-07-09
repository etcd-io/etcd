// Copyright 2016 The etcd Authors
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

package adt

import (
	"math/rand"
	"testing"
	"time"
)

func TestIntervalTreeIntersects(t *testing.T) {
	ivt := NewIntervalTree()
	ivt.Insert(NewStringInterval("1", "3"), 123)

	if ivt.Intersects(NewStringPoint("0")) {
		t.Errorf("contains 0")
	}
	if !ivt.Intersects(NewStringPoint("1")) {
		t.Errorf("missing 1")
	}
	if !ivt.Intersects(NewStringPoint("11")) {
		t.Errorf("missing 11")
	}
	if !ivt.Intersects(NewStringPoint("2")) {
		t.Errorf("missing 2")
	}
	if ivt.Intersects(NewStringPoint("3")) {
		t.Errorf("contains 3")
	}
}

func TestIntervalTreeStringAffine(t *testing.T) {
	ivt := NewIntervalTree()
	ivt.Insert(NewStringAffineInterval("8", ""), 123)
	if !ivt.Intersects(NewStringAffinePoint("9")) {
		t.Errorf("missing 9")
	}
	if ivt.Intersects(NewStringAffinePoint("7")) {
		t.Errorf("contains 7")
	}
}

func TestIntervalTreeStab(t *testing.T) {
	ivt := NewIntervalTree()
	ivt.Insert(NewStringInterval("0", "1"), 123)
	ivt.Insert(NewStringInterval("0", "2"), 456)
	ivt.Insert(NewStringInterval("5", "6"), 789)
	ivt.Insert(NewStringInterval("6", "8"), 999)
	ivt.Insert(NewStringInterval("0", "3"), 0)

	if ivt.root.max.Compare(StringComparable("8")) != 0 {
		t.Fatalf("wrong root max got %v, expected 8", ivt.root.max)
	}
	if x := len(ivt.Stab(NewStringPoint("0"))); x != 3 {
		t.Errorf("got %d, expected 3", x)
	}
	if x := len(ivt.Stab(NewStringPoint("1"))); x != 2 {
		t.Errorf("got %d, expected 2", x)
	}
	if x := len(ivt.Stab(NewStringPoint("2"))); x != 1 {
		t.Errorf("got %d, expected 1", x)
	}
	if x := len(ivt.Stab(NewStringPoint("3"))); x != 0 {
		t.Errorf("got %d, expected 0", x)
	}
	if x := len(ivt.Stab(NewStringPoint("5"))); x != 1 {
		t.Errorf("got %d, expected 1", x)
	}
	if x := len(ivt.Stab(NewStringPoint("55"))); x != 1 {
		t.Errorf("got %d, expected 1", x)
	}
	if x := len(ivt.Stab(NewStringPoint("6"))); x != 1 {
		t.Errorf("got %d, expected 1", x)
	}
}

type xy struct {
	x int64
	y int64
}

func TestIntervalTreeRandom(t *testing.T) {
	// generate unique intervals
	ivs := make(map[xy]struct{})
	ivt := NewIntervalTree()
	maxv := 128
	rand.Seed(time.Now().UnixNano())

	for i := rand.Intn(maxv) + 1; i != 0; i-- {
		x, y := int64(rand.Intn(maxv)), int64(rand.Intn(maxv))
		if x > y {
			t := x
			x = y
			y = t
		} else if x == y {
			y++
		}
		iv := xy{x, y}
		if _, ok := ivs[iv]; ok {
			// don't double insert
			continue
		}
		ivt.Insert(NewInt64Interval(x, y), 123)
		ivs[iv] = struct{}{}
	}

	for ab := range ivs {
		for xy := range ivs {
			v := xy.x + int64(rand.Intn(int(xy.y-xy.x)))
			if slen := len(ivt.Stab(NewInt64Point(v))); slen == 0 {
				t.Fatalf("expected %v stab non-zero for [%+v)", v, xy)
			}
			if !ivt.Intersects(NewInt64Point(v)) {
				t.Fatalf("did not get %d as expected for [%+v)", v, xy)
			}
		}
		if !ivt.Delete(NewInt64Interval(ab.x, ab.y)) {
			t.Errorf("did not delete %v as expected", ab)
		}
		delete(ivs, ab)
	}

	if ivt.Len() != 0 {
		t.Errorf("got ivt.Len() = %v, expected 0", ivt.Len())
	}
}

// TestIntervalTreeSortedVisit tests that intervals are visited in sorted order.
func TestIntervalTreeSortedVisit(t *testing.T) {
	tests := []struct {
		ivls       []Interval
		visitRange Interval
	}{
		{
			ivls:       []Interval{NewInt64Interval(1, 10), NewInt64Interval(2, 5), NewInt64Interval(3, 6)},
			visitRange: NewInt64Interval(0, 100),
		},
		{
			ivls:       []Interval{NewInt64Interval(1, 10), NewInt64Interval(10, 12), NewInt64Interval(3, 6)},
			visitRange: NewInt64Interval(0, 100),
		},
		{
			ivls:       []Interval{NewInt64Interval(2, 3), NewInt64Interval(3, 4), NewInt64Interval(6, 7), NewInt64Interval(5, 6)},
			visitRange: NewInt64Interval(0, 100),
		},
		{
			ivls: []Interval{
				NewInt64Interval(2, 3),
				NewInt64Interval(2, 4),
				NewInt64Interval(3, 7),
				NewInt64Interval(2, 5),
				NewInt64Interval(3, 8),
				NewInt64Interval(3, 5),
			},
			visitRange: NewInt64Interval(0, 100),
		},
	}
	for i, tt := range tests {
		ivt := NewIntervalTree()
		for _, ivl := range tt.ivls {
			ivt.Insert(ivl, struct{}{})
		}
		last := tt.ivls[0].Begin
		count := 0
		chk := func(iv *IntervalValue) bool {
			if last.Compare(iv.Ivl.Begin) > 0 {
				t.Errorf("#%d: expected less than %d, got interval %+v", i, last, iv.Ivl)
			}
			last = iv.Ivl.Begin
			count++
			return true
		}
		ivt.Visit(tt.visitRange, chk)
		if count != len(tt.ivls) {
			t.Errorf("#%d: did not cover all intervals. expected %d, got %d", i, len(tt.ivls), count)
		}
	}
}

// TestIntervalTreeVisitExit tests that visiting can be stopped.
func TestIntervalTreeVisitExit(t *testing.T) {
	ivls := []Interval{NewInt64Interval(1, 10), NewInt64Interval(2, 5), NewInt64Interval(3, 6), NewInt64Interval(4, 8)}
	ivlRange := NewInt64Interval(0, 100)
	tests := []struct {
		f IntervalVisitor

		wcount int
	}{
		{
			f:      func(n *IntervalValue) bool { return false },
			wcount: 1,
		},
		{
			f:      func(n *IntervalValue) bool { return n.Ivl.Begin.Compare(ivls[0].Begin) <= 0 },
			wcount: 2,
		},
		{
			f:      func(n *IntervalValue) bool { return n.Ivl.Begin.Compare(ivls[2].Begin) < 0 },
			wcount: 3,
		},
		{
			f:      func(n *IntervalValue) bool { return true },
			wcount: 4,
		},
	}

	for i, tt := range tests {
		ivt := NewIntervalTree()
		for _, ivl := range ivls {
			ivt.Insert(ivl, struct{}{})
		}
		count := 0
		ivt.Visit(ivlRange, func(n *IntervalValue) bool {
			count++
			return tt.f(n)
		})
		if count != tt.wcount {
			t.Errorf("#%d: expected count %d, got %d", i, tt.wcount, count)
		}
	}
}

// TestIntervalTreeContains tests that contains returns true iff the ivt maps the entire interval.
func TestIntervalTreeContains(t *testing.T) {
	tests := []struct {
		ivls   []Interval
		chkIvl Interval

		wContains bool
	}{
		{
			ivls:   []Interval{NewInt64Interval(1, 10)},
			chkIvl: NewInt64Interval(0, 100),

			wContains: false,
		},
		{
			ivls:   []Interval{NewInt64Interval(1, 10)},
			chkIvl: NewInt64Interval(1, 10),

			wContains: true,
		},
		{
			ivls:   []Interval{NewInt64Interval(1, 10)},
			chkIvl: NewInt64Interval(2, 8),

			wContains: true,
		},
		{
			ivls:   []Interval{NewInt64Interval(1, 5), NewInt64Interval(6, 10)},
			chkIvl: NewInt64Interval(1, 10),

			wContains: false,
		},
		{
			ivls:   []Interval{NewInt64Interval(1, 5), NewInt64Interval(3, 10)},
			chkIvl: NewInt64Interval(1, 10),

			wContains: true,
		},
		{
			ivls:   []Interval{NewInt64Interval(1, 4), NewInt64Interval(4, 7), NewInt64Interval(3, 10)},
			chkIvl: NewInt64Interval(1, 10),

			wContains: true,
		},
		{
			ivls:   []Interval{},
			chkIvl: NewInt64Interval(1, 10),

			wContains: false,
		},
	}
	for i, tt := range tests {
		ivt := NewIntervalTree()
		for _, ivl := range tt.ivls {
			ivt.Insert(ivl, struct{}{})
		}
		if v := ivt.Contains(tt.chkIvl); v != tt.wContains {
			t.Errorf("#%d: ivt.Contains got %v, expected %v", i, v, tt.wContains)
		}
	}
}

func TestIntervalTreeDeleteFixUp(t *testing.T) {
	ivt := NewIntervalTree()
	ivt.Insert(NewInt64Interval(510, 511), 123)
	ivt.Insert(NewInt64Interval(82, 83), 456)
	ivt.Insert(NewInt64Interval(830, 831), 789)
	ivt.Insert(NewInt64Interval(11, 12), 999)
	ivt.Insert(NewInt64Interval(383, 384), 1)
	ivt.Insert(NewInt64Interval(647, 648), 2)
	ivt.Insert(NewInt64Interval(899, 900), 3)
	ivt.Insert(NewInt64Interval(261, 262), 4)
	ivt.Insert(NewInt64Interval(410, 411), 5)
	ivt.Insert(NewInt64Interval(514, 515), 6)
	ivt.Insert(NewInt64Interval(815, 816), 7)
	ivt.Insert(NewInt64Interval(888, 889), 8)
	ivt.Insert(NewInt64Interval(972, 973), 9)
	ivt.Insert(NewInt64Interval(238, 239), 10)
	ivt.Insert(NewInt64Interval(292, 293), 11)
	ivt.Insert(NewInt64Interval(953, 954), 12)

	type intervalNodeValue struct {
		node *intervalNode
		c    rbcolor
	}

	rawTreeLevels := make([][]*intervalNodeValue, ivt.Height())

	rawTreeLevels[0] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(510, 511), 123}}, black},
	}

	rawTreeLevels[1] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(82, 83), 456}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(830, 831), 789}}, black},
	}

	rawTreeLevels[2] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(11, 12), 999}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(383, 384), 1}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(647, 648), 2}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(899, 900), 3}}, red},
	}

	rawTreeLevels[3] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(261, 262), 4}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(410, 411), 5}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(514, 515), 6}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(815, 816), 7}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(888, 889), 8}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(972, 973), 9}}, black},
	}

	rawTreeLevels[4] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(238, 239), 10}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(292, 293), 11}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(953, 954), 12}}, red},
	}

	/*
		{Ivl:{Begin:510 End:511} Val:123}
		{Ivl:{Begin:82 End:83} Val:456}{Ivl:{Begin:830 End:831} Val:789}
		{Ivl:{Begin:11 End:12} Val:999}{Ivl:{Begin:383 End:384} Val:1}{Ivl:{Begin:647 End:648} Val:2}{Ivl:{Begin:899 End:900} Val:3}
		{Ivl:{Begin:261 End:262} Val:4}{Ivl:{Begin:410 End:411} Val:5}{Ivl:{Begin:514 End:515} Val:6}{Ivl:{Begin:815 End:816} Val:7}{Ivl:{Begin:888 End:889} Val:8}{Ivl:{Begin:972 End:973} Val:9}
		{Ivl:{Begin:238 End:239} Val:10}{Ivl:{Begin:292 End:293} Val:11}{Ivl:{Begin:953 End:954} Val:12}
	*/

	levels := ivt.LevelOrder()
	for i, curLevels := range levels {
		if len(curLevels) != len(rawTreeLevels[i]) {
			t.Errorf("#%d: interval tree level, expected %d=%d", i, len(curLevels), len(rawTreeLevels[i]))
		}
		for j, node := range curLevels {
			if node.iv.Ivl.Begin.Compare(rawTreeLevels[i][j].node.iv.Ivl.Begin) != 0 ||
				node.iv.Ivl.End.Compare(rawTreeLevels[i][j].node.iv.Ivl.End) != 0 ||
				node.c != rawTreeLevels[i][j].c {
				t.Errorf("interval node expected same, but %+v != %+v", node, rawTreeLevels[i][j].node)
			}
		}
	}

	ivt.Delete(NewInt64Interval(514, 515))

	/*
		After Delete (514, 515) node:

		{Ivl:{Begin:510 End:511} Val:123}
		{Ivl:{Begin:82 End:83} Val:456}{Ivl:{Begin:830 End:831} Val:789}
		{Ivl:{Begin:11 End:12} Val:999}{Ivl:{Begin:383 End:384} Val:1}{Ivl:{Begin:647 End:648} Val:2}{Ivl:{Begin:899 End:900} Val:3}
		{Ivl:{Begin:261 End:262} Val:4}{Ivl:{Begin:410 End:411} Val:5}{Ivl:{Begin:815 End:816} Val:7}{Ivl:{Begin:888 End:889} Val:8}{Ivl:{Begin:972 End:973} Val:9}
		{Ivl:{Begin:238 End:239} Val:10}{Ivl:{Begin:292 End:293} Val:11}{Ivl:{Begin:953 End:954} Val:12}
	*/

	ivt.Delete(NewInt64Interval(11, 12))

	/*
		After Delete (11, 12) node:

		{Ivl:{Begin:510 End:511} Val:123}
		{Ivl:{Begin:383 End:384} Val:1}{Ivl:{Begin:830 End:831} Val:789}
		{Ivl:{Begin:261 End:262} Val:4}{Ivl:{Begin:410 End:411} Val:5}{Ivl:{Begin:647 End:648} Val:2}{Ivl:{Begin:899 End:900} Val:3}
		{Ivl:{Begin:82 End:83} Val:456}{Ivl:{Begin:292 End:293} Val:11}{Ivl:{Begin:815 End:816} Val:7}{Ivl:{Begin:888 End:889} Val:8}{Ivl:{Begin:972 End:973} Val:9}
		{Ivl:{Begin:238 End:239} Val:10}{Ivl:{Begin:953 End:954} Val:12}
	*/

	delTreeLevels := make([][]*intervalNodeValue, ivt.Height())

	delTreeLevels[0] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(510, 511), 123}}, black},
	}

	delTreeLevels[1] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(383, 384), 1}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(830, 831), 789}}, black},
	}

	delTreeLevels[2] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(261, 262), 4}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(410, 411), 5}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(647, 648), 2}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(899, 900), 3}}, red},
	}

	delTreeLevels[3] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(82, 83), 456}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(292, 293), 11}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(815, 816), 7}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(888, 889), 8}}, black},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(972, 973), 9}}, black},
	}

	delTreeLevels[4] = []*intervalNodeValue{
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(238, 239), 10}}, red},
		&intervalNodeValue{&intervalNode{iv: IntervalValue{NewInt64Interval(953, 954), 12}}, red},
	}

	levels = ivt.LevelOrder()

	for i, curLevels := range levels {
		if len(curLevels) != len(delTreeLevels[i]) {
			t.Errorf("#%d: interval tree level, expected %d=%d", i, len(curLevels), len(delTreeLevels[i]))
		}
		for j, node := range curLevels {
			if node.iv.Ivl.Begin.Compare(delTreeLevels[i][j].node.iv.Ivl.Begin) != 0 ||
				node.iv.Ivl.End.Compare(delTreeLevels[i][j].node.iv.Ivl.End) != 0 ||
				node.c != delTreeLevels[i][j].c {
				t.Errorf("interval node expected same, but %+v != %+v", node, delTreeLevels[i][j].node)
			}
		}
	}
}
