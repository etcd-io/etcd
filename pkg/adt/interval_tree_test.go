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
	"reflect"
	"testing"
	"time"
)

// TestIntervalTreeInsert tests interval tree insertion.
func TestIntervalTreeInsert(t *testing.T) {
	// "Introduction to Algorithms" (Cormen et al, 3rd ed.) chapter 14, Figure 14.4
	ivt := NewIntervalTree()
	ivt.Insert(NewInt64Interval(16, 21), 30)
	ivt.Insert(NewInt64Interval(8, 9), 23)
	ivt.Insert(NewInt64Interval(0, 3), 3)
	ivt.Insert(NewInt64Interval(5, 8), 10)
	ivt.Insert(NewInt64Interval(6, 10), 10)
	ivt.Insert(NewInt64Interval(15, 23), 23)
	ivt.Insert(NewInt64Interval(17, 19), 20)
	ivt.Insert(NewInt64Interval(25, 30), 30)
	ivt.Insert(NewInt64Interval(26, 26), 26)
	ivt.Insert(NewInt64Interval(19, 20), 20)

	expected := []visitedInterval{
		{root: NewInt64Interval(16, 21), color: black, left: NewInt64Interval(8, 9), right: NewInt64Interval(25, 30), depth: 0},

		{root: NewInt64Interval(8, 9), color: red, left: NewInt64Interval(5, 8), right: NewInt64Interval(15, 23), depth: 1},
		{root: NewInt64Interval(25, 30), color: red, left: NewInt64Interval(17, 19), right: NewInt64Interval(26, 26), depth: 1},

		{root: NewInt64Interval(5, 8), color: black, left: NewInt64Interval(0, 3), right: NewInt64Interval(6, 10), depth: 2},
		{root: NewInt64Interval(15, 23), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 2},
		{root: NewInt64Interval(17, 19), color: black, left: newInt64EmptyInterval(), right: NewInt64Interval(19, 20), depth: 2},
		{root: NewInt64Interval(26, 26), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 2},

		{root: NewInt64Interval(0, 3), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(6, 10), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(19, 20), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
	}

	tr := ivt.(*intervalTree)
	visits := tr.visitLevel()
	if !reflect.DeepEqual(expected, visits) {
		t.Fatalf("level order expected %v, got %v", expected, visits)
	}
}

// TestIntervalTreeSelfBalanced ensures range tree is self-balanced after inserting ranges to the tree.
// Use https://www.cs.usfca.edu/~galles/visualization/RedBlack.html for test case creation.
//
// Regular Binary Search Tree
//   [0,1]
//       \
//       [1,2]
//          \
//         [3,4]
//            \
//           [5,6]
//               \
//              [7,8]
//                 \
//                [8,9]
//
// Self-Balancing Binary Search Tree
//          [1,2]
//        /       \
//   [0,1]        [5,6]
//                 /   \
//            [3,4]    [7,8]
//                         \
//                         [8,9]
//
func TestIntervalTreeSelfBalanced(t *testing.T) {
	ivt := NewIntervalTree()
	ivt.Insert(NewInt64Interval(0, 1), 0)
	ivt.Insert(NewInt64Interval(1, 2), 0)
	ivt.Insert(NewInt64Interval(3, 4), 0)
	ivt.Insert(NewInt64Interval(5, 6), 0)
	ivt.Insert(NewInt64Interval(7, 8), 0)
	ivt.Insert(NewInt64Interval(8, 9), 0)

	expected := []visitedInterval{
		{root: NewInt64Interval(1, 2), color: black, left: NewInt64Interval(0, 1), right: NewInt64Interval(5, 6), depth: 0},

		{root: NewInt64Interval(0, 1), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 1},
		{root: NewInt64Interval(5, 6), color: red, left: NewInt64Interval(3, 4), right: NewInt64Interval(7, 8), depth: 1},

		{root: NewInt64Interval(3, 4), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 2},
		{root: NewInt64Interval(7, 8), color: black, left: newInt64EmptyInterval(), right: NewInt64Interval(8, 9), depth: 2},

		{root: NewInt64Interval(8, 9), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
	}

	tr := ivt.(*intervalTree)
	visits := tr.visitLevel()
	if !reflect.DeepEqual(expected, visits) {
		t.Fatalf("level order expected %v, got %v", expected, visits)
	}

	if visits[len(visits)-1].depth != 3 {
		t.Fatalf("expected self-balanced tree with last level 3, but last level got %d", visits[len(visits)-1].depth)
	}
}

// TestIntervalTreeDelete ensures delete operation maintains red-black tree properties.
// Use https://www.cs.usfca.edu/~galles/visualization/RedBlack.html for test case creation.
// See https://github.com/etcd-io/etcd/issues/10877 for more detail.
//
//
// After insertion:
//                         [510,511]
//                          /      \
//                ----------        -----------------------
//               /                                          \
//           [82,83]                                      [830,831]
//           /    \                                    /            \
//          /      \                                  /               \
//    [11,12]    [383,384](red)               [647,648]              [899,900](red)
//                 /   \                      /      \                      /    \
//                /     \                    /        \                    /      \
//          [261,262]  [410,411]  [514,515](red)  [815,816](red)  [888,889]      [972,973]
//          /       \                                                           /
//         /         \                                                         /
//  [238,239](red)  [292,293](red)                                    [953,954](red)
//
//
// After deleting 514 (no rebalance):
//                         [510,511]
//                          /      \
//                ----------        -----------------------
//               /                                          \
//           [82,83]                                      [830,831]
//           /    \                                    /            \
//          /      \                                  /               \
//    [11,12]    [383,384](red)               [647,648]              [899,900](red)
//                 /   \                            \                      /    \
//                /     \                            \                    /      \
//          [261,262]  [410,411]                  [815,816](red)  [888,889]      [972,973]
//          /       \                                                           /
//         /         \                                                         /
//  [238,239](red)  [292,293](red)                                    [953,954](red)
//
//
// After deleting 11 (requires rebalancing):
//                         [510,511]
//                          /      \
//                ----------        --------------------------
//               /                                            \
//           [383,384]                                       [830,831]
//           /       \                                      /          \
//          /         \                                    /            \
//   [261,262](red)  [410,411]                     [647,648]           [899,900](red)
//       /               \                              \                      /    \
//      /                 \                              \                    /      \
//   [82,83]           [292,293]                      [815,816](red)   [888,889]    [972,973]
//         \                                                           /
//          \                                                         /
//       [238,239](red)                                       [953,954](red)
//
//
func TestIntervalTreeDelete(t *testing.T) {
	ivt := NewIntervalTree()
	ivt.Insert(NewInt64Interval(510, 511), 0)
	ivt.Insert(NewInt64Interval(82, 83), 0)
	ivt.Insert(NewInt64Interval(830, 831), 0)
	ivt.Insert(NewInt64Interval(11, 12), 0)
	ivt.Insert(NewInt64Interval(383, 384), 0)
	ivt.Insert(NewInt64Interval(647, 648), 0)
	ivt.Insert(NewInt64Interval(899, 900), 0)
	ivt.Insert(NewInt64Interval(261, 262), 0)
	ivt.Insert(NewInt64Interval(410, 411), 0)
	ivt.Insert(NewInt64Interval(514, 515), 0)
	ivt.Insert(NewInt64Interval(815, 816), 0)
	ivt.Insert(NewInt64Interval(888, 889), 0)
	ivt.Insert(NewInt64Interval(972, 973), 0)
	ivt.Insert(NewInt64Interval(238, 239), 0)
	ivt.Insert(NewInt64Interval(292, 293), 0)
	ivt.Insert(NewInt64Interval(953, 954), 0)

	tr := ivt.(*intervalTree)

	expectedBeforeDelete := []visitedInterval{
		{root: NewInt64Interval(510, 511), color: black, left: NewInt64Interval(82, 83), right: NewInt64Interval(830, 831), depth: 0},

		{root: NewInt64Interval(82, 83), color: black, left: NewInt64Interval(11, 12), right: NewInt64Interval(383, 384), depth: 1},
		{root: NewInt64Interval(830, 831), color: black, left: NewInt64Interval(647, 648), right: NewInt64Interval(899, 900), depth: 1},

		{root: NewInt64Interval(11, 12), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 2},
		{root: NewInt64Interval(383, 384), color: red, left: NewInt64Interval(261, 262), right: NewInt64Interval(410, 411), depth: 2},
		{root: NewInt64Interval(647, 648), color: black, left: NewInt64Interval(514, 515), right: NewInt64Interval(815, 816), depth: 2},
		{root: NewInt64Interval(899, 900), color: red, left: NewInt64Interval(888, 889), right: NewInt64Interval(972, 973), depth: 2},

		{root: NewInt64Interval(261, 262), color: black, left: NewInt64Interval(238, 239), right: NewInt64Interval(292, 293), depth: 3},
		{root: NewInt64Interval(410, 411), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(514, 515), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(815, 816), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(888, 889), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(972, 973), color: black, left: NewInt64Interval(953, 954), right: newInt64EmptyInterval(), depth: 3},

		{root: NewInt64Interval(238, 239), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
		{root: NewInt64Interval(292, 293), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
		{root: NewInt64Interval(953, 954), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
	}
	visitsBeforeDelete := tr.visitLevel()
	if !reflect.DeepEqual(expectedBeforeDelete, visitsBeforeDelete) {
		t.Fatalf("level order after insertion expected %v, got %v", expectedBeforeDelete, visitsBeforeDelete)
	}

	// delete the node "514"
	range514 := NewInt64Interval(514, 515)
	if deleted := tr.Delete(NewInt64Interval(514, 515)); !deleted {
		t.Fatalf("range %v not deleted", range514)
	}

	expectedAfterDelete514 := []visitedInterval{
		{root: NewInt64Interval(510, 511), color: black, left: NewInt64Interval(82, 83), right: NewInt64Interval(830, 831), depth: 0},

		{root: NewInt64Interval(82, 83), color: black, left: NewInt64Interval(11, 12), right: NewInt64Interval(383, 384), depth: 1},
		{root: NewInt64Interval(830, 831), color: black, left: NewInt64Interval(647, 648), right: NewInt64Interval(899, 900), depth: 1},

		{root: NewInt64Interval(11, 12), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 2},
		{root: NewInt64Interval(383, 384), color: red, left: NewInt64Interval(261, 262), right: NewInt64Interval(410, 411), depth: 2},
		{root: NewInt64Interval(647, 648), color: black, left: newInt64EmptyInterval(), right: NewInt64Interval(815, 816), depth: 2},
		{root: NewInt64Interval(899, 900), color: red, left: NewInt64Interval(888, 889), right: NewInt64Interval(972, 973), depth: 2},

		{root: NewInt64Interval(261, 262), color: black, left: NewInt64Interval(238, 239), right: NewInt64Interval(292, 293), depth: 3},
		{root: NewInt64Interval(410, 411), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(815, 816), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(888, 889), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(972, 973), color: black, left: NewInt64Interval(953, 954), right: newInt64EmptyInterval(), depth: 3},

		{root: NewInt64Interval(238, 239), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
		{root: NewInt64Interval(292, 293), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
		{root: NewInt64Interval(953, 954), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
	}
	visitsAfterDelete514 := tr.visitLevel()
	if !reflect.DeepEqual(expectedAfterDelete514, visitsAfterDelete514) {
		t.Fatalf("level order after deleting '514' expected %v, got %v", expectedAfterDelete514, visitsAfterDelete514)
	}

	// delete the node "11"
	range11 := NewInt64Interval(11, 12)
	if deleted := tr.Delete(NewInt64Interval(11, 12)); !deleted {
		t.Fatalf("range %v not deleted", range11)
	}

	expectedAfterDelete11 := []visitedInterval{
		{root: NewInt64Interval(510, 511), color: black, left: NewInt64Interval(383, 384), right: NewInt64Interval(830, 831), depth: 0},

		{root: NewInt64Interval(383, 384), color: black, left: NewInt64Interval(261, 262), right: NewInt64Interval(410, 411), depth: 1},
		{root: NewInt64Interval(830, 831), color: black, left: NewInt64Interval(647, 648), right: NewInt64Interval(899, 900), depth: 1},

		{root: NewInt64Interval(261, 262), color: red, left: NewInt64Interval(82, 83), right: NewInt64Interval(292, 293), depth: 2},
		{root: NewInt64Interval(410, 411), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 2},
		{root: NewInt64Interval(647, 648), color: black, left: newInt64EmptyInterval(), right: NewInt64Interval(815, 816), depth: 2},
		{root: NewInt64Interval(899, 900), color: red, left: NewInt64Interval(888, 889), right: NewInt64Interval(972, 973), depth: 2},

		{root: NewInt64Interval(82, 83), color: black, left: newInt64EmptyInterval(), right: NewInt64Interval(238, 239), depth: 3},
		{root: NewInt64Interval(292, 293), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(815, 816), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(888, 889), color: black, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 3},
		{root: NewInt64Interval(972, 973), color: black, left: NewInt64Interval(953, 954), right: newInt64EmptyInterval(), depth: 3},

		{root: NewInt64Interval(238, 239), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
		{root: NewInt64Interval(953, 954), color: red, left: newInt64EmptyInterval(), right: newInt64EmptyInterval(), depth: 4},
	}
	visitsAfterDelete11 := tr.visitLevel()
	if !reflect.DeepEqual(expectedAfterDelete11, visitsAfterDelete11) {
		t.Fatalf("level order after deleting '11' expected %v, got %v", expectedAfterDelete11, visitsAfterDelete11)
	}
}

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

	tr := ivt.(*intervalTree)
	if tr.root.max.Compare(StringComparable("8")) != 0 {
		t.Fatalf("wrong root max got %v, expected 8", tr.root.max)
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
