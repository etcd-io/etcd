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
	"bytes"
	"math"
)

// Comparable is an interface for trichotomic comparisons.
type Comparable interface {
	// Compare gives the result of a 3-way comparison
	// a.Compare(b) = 1 => a > b
	// a.Compare(b) = 0 => a == b
	// a.Compare(b) = -1 => a < b
	Compare(c Comparable) int
}

type rbcolor int

const (
	black rbcolor = iota
	red
)

// Interval implements a Comparable interval [begin, end)
// TODO: support different sorts of intervals: (a,b), [a,b], (a, b]
type Interval struct {
	Begin Comparable
	End   Comparable
}

// Compare on an interval gives == if the interval overlaps.
func (ivl *Interval) Compare(c Comparable) int {
	ivl2 := c.(*Interval)
	ivbCmpBegin := ivl.Begin.Compare(ivl2.Begin)
	ivbCmpEnd := ivl.Begin.Compare(ivl2.End)
	iveCmpBegin := ivl.End.Compare(ivl2.Begin)

	// ivl is left of ivl2
	if ivbCmpBegin < 0 && iveCmpBegin <= 0 {
		return -1
	}

	// iv is right of iv2
	if ivbCmpEnd >= 0 {
		return 1
	}

	return 0
}

type intervalNode struct {
	// iv is the interval-value pair entry.
	iv IntervalValue
	// max endpoint of all descendent nodes.
	max Comparable
	// left and right are sorted by low endpoint of key interval
	left, right *intervalNode
	// parent is the direct ancestor of the node
	parent *intervalNode
	c      rbcolor
}

func (x *intervalNode) color(nilNode *intervalNode) rbcolor {
	if x == nilNode {
		return black
	}
	return x.c
}

func (x *intervalNode) height(nilNode *intervalNode) int {
	if x == nilNode {
		return 0
	}
	ld := x.left.height(nilNode)
	rd := x.right.height(nilNode)
	if ld < rd {
		return rd + 1
	}
	return ld + 1
}

func (x *intervalNode) min(nilNode *intervalNode) *intervalNode {
	for x.left != nilNode {
		x = x.left
	}
	return x
}

// successor is the next in-order node in the tree
func (x *intervalNode) successor(nilNode *intervalNode) *intervalNode {
	if x.right != nilNode {
		return x.right.min(nilNode)
	}
	y := x.parent
	for y != nilNode && x == y.right {
		x = y
		y = y.parent
	}
	return y
}

// updateMax updates the maximum values for a node and its ancestors
func (x *intervalNode) updateMax(nilNode *intervalNode) {
	for x != nilNode {
		oldmax := x.max
		max := x.iv.Ivl.End
		if x.left != nilNode && x.left.max.Compare(max) > 0 {
			max = x.left.max
		}
		if x.right != nilNode && x.right.max.Compare(max) > 0 {
			max = x.right.max
		}
		if oldmax.Compare(max) == 0 {
			break
		}
		x.max = max
		x = x.parent
	}
}

type nodeVisitor func(n *intervalNode) bool

// visit will call a node visitor on each node that overlaps the given interval
func (x *intervalNode) visit(iv *Interval, nilNode *intervalNode, nv nodeVisitor) bool {
	if x == nilNode {
		return true
	}
	v := iv.Compare(&x.iv.Ivl)
	switch {
	case v < 0:
		if !x.left.visit(iv, nilNode, nv) {
			return false
		}
	case v > 0:
		maxiv := Interval{x.iv.Ivl.Begin, x.max}
		if maxiv.Compare(iv) == 0 {
			if !x.left.visit(iv, nilNode, nv) || !x.right.visit(iv, nilNode, nv) {
				return false
			}
		}
	default:
		if !x.left.visit(iv, nilNode, nv) || !nv(x) || !x.right.visit(iv, nilNode, nv) {
			return false
		}
	}
	return true
}

type IntervalValue struct {
	Ivl Interval
	Val interface{}
}

// IntervalTree represents a (mostly) textbook implementation of the
// "Introduction to Algorithms" (Cormen et al, 2nd ed.) chapter 13 red-black tree
// and chapter 14.3 interval tree with search supporting "stabbing queries".
type IntervalTree struct {
	root  *intervalNode
	count int

	// red-black NIL node, can not use golang nil instead
	nilNode *intervalNode
}

func NewIntervalTree() *IntervalTree {
	tree := &IntervalTree{}

	tree.nilNode = &intervalNode{}

	tree.root = tree.nilNode
	//tree.nilNode.left = tree.root
	//tree.nilNode.right = tree.root
	//tree.nilNode.parent = tree.root
	tree.nilNode.c = black

	return tree
}

// Delete removes the node with the given interval from the tree, returning
// true if a node is in fact removed.
func (ivt *IntervalTree) Delete(ivl Interval) bool {
	z := ivt.find(ivl)
	if z == ivt.nilNode {
		return false
	}

	y := z
	if z.left != ivt.nilNode && z.right != ivt.nilNode {
		y = z.successor(ivt.nilNode)
	}

	// x := y.left
	// if x == ivt.nilNode {
	// 	x = y.right
	// }
	x := ivt.nilNode
	if y.left != ivt.nilNode {
		x = y.left
	} else if y.right != ivt.nilNode {
		x = y.right
	}

	x.parent = y.parent

	if y.parent == ivt.nilNode {
		ivt.root = x
		//ivt.nilNode.left = ivt.root
		//ivt.nilNode.right = ivt.root
		//ivt.nilNode.parent = ivt.root
	} else {
		if y == y.parent.left {
			y.parent.left = x
		} else {
			y.parent.right = x
		}
		y.parent.updateMax(ivt.nilNode)
	}
	if y != z {
		z.iv = y.iv
		z.updateMax(ivt.nilNode)
	}

	//if y.color() == black && x != nil {
	if y.color(ivt.nilNode) == black { //&& !(x == ivt.nilNode && x.parent == ivt.nilNode) {
		ivt.deleteFixup(x)
	}

	ivt.count--
	return true
}

func (ivt *IntervalTree) deleteFixup(x *intervalNode) {
	for x != ivt.root && x.color(ivt.nilNode) == black {
		if x == x.parent.left {
			w := x.parent.right
			if w.color(ivt.nilNode) == red {
				w.c = black
				x.parent.c = red
				ivt.rotateLeft(x.parent)
				w = x.parent.right
			}
			// if w == ivt.nilNode {
			// 	break
			// }
			if w.left.color(ivt.nilNode) == black && w.right.color(ivt.nilNode) == black {
				w.c = red
				x = x.parent
			} else {
				if w.right.color(ivt.nilNode) == black {
					w.left.c = black
					w.c = red
					ivt.rotateRight(w)
					w = x.parent.right
				}
				w.c = x.parent.color(ivt.nilNode)
				x.parent.c = black
				w.right.c = black
				ivt.rotateLeft(x.parent)
				x = ivt.root
			}
		} else {
			// same as above but with left and right exchanged
			w := x.parent.left
			if w.color(ivt.nilNode) == red {
				w.c = black
				x.parent.c = red
				ivt.rotateRight(x.parent)
				w = x.parent.left
			}
			// if w == ivt.nilNode {
			// 	break
			// }
			if w.left.color(ivt.nilNode) == black && w.right.color(ivt.nilNode) == black {
				w.c = red
				x = x.parent
			} else {
				if w.left.color(ivt.nilNode) == black {
					w.right.c = black
					w.c = red
					ivt.rotateLeft(w)
					w = x.parent.left
				}
				w.c = x.parent.color(ivt.nilNode)
				x.parent.c = black
				w.left.c = black
				ivt.rotateRight(x.parent)
				x = ivt.root
			}
		}
	}
	//if x != nil {
	//ivt.nilNode.parent = ivt.root
	x.c = black
	//}
}

// Insert adds a node with the given interval into the tree.
func (ivt *IntervalTree) Insert(ivl Interval, val interface{}) {
	y := ivt.nilNode
	z := &intervalNode{
		iv:     IntervalValue{ivl, val},
		max:    ivl.End,
		c:      red,
		left:   ivt.nilNode,
		right:  ivt.nilNode,
		parent: ivt.nilNode,
	}
	x := ivt.root
	for x != ivt.nilNode {
		y = x
		if z.iv.Ivl.Begin.Compare(x.iv.Ivl.Begin) < 0 {
			x = x.left
		} else {
			x = x.right
		}
	}

	z.parent = y
	if y == ivt.nilNode {
		ivt.root = z
		//ivt.root.parent = ivt.nilNode
		//ivt.nilNode.parent = ivt.root
		//ivt.nilNode.left = ivt.root
		//ivt.nilNode.right = ivt.root
	} else {
		if z.iv.Ivl.Begin.Compare(y.iv.Ivl.Begin) < 0 {
			y.left = z
		} else {
			y.right = z
		}
		y.updateMax(ivt.nilNode)
	}
	z.c = red
	ivt.insertFixup(z)
	ivt.count++
}

func (ivt *IntervalTree) insertFixup(z *intervalNode) {
	for z.parent.color(ivt.nilNode) == red {
		if z.parent == z.parent.parent.left {
			y := z.parent.parent.right
			if y.color(ivt.nilNode) == red {
				y.c = black
				z.parent.c = black
				z.parent.parent.c = red
				z = z.parent.parent
			} else {
				if z == z.parent.right {
					z = z.parent
					ivt.rotateLeft(z)
				}
				z.parent.c = black
				z.parent.parent.c = red
				ivt.rotateRight(z.parent.parent)
			}
		} else {
			// same as then with left/right exchanged
			y := z.parent.parent.left
			if y.color(ivt.nilNode) == red {
				y.c = black
				z.parent.c = black
				z.parent.parent.c = red
				z = z.parent.parent
			} else {
				if z == z.parent.left {
					z = z.parent
					ivt.rotateRight(z)
				}
				z.parent.c = black
				z.parent.parent.c = red
				ivt.rotateLeft(z.parent.parent)
			}
		}
	}
	ivt.root.c = black
}

// rotateLeft moves x so it is left of its right child
func (ivt *IntervalTree) rotateLeft(x *intervalNode) {

	// rotateLeft x must have right child
	if x.right == ivt.nilNode {
		return
	}

	y := x.right
	x.right = y.left
	if y.left != ivt.nilNode {
		y.left.parent = x
	}
	x.updateMax(ivt.nilNode)
	ivt.replaceParent(x, y)
	y.left = x
	y.updateMax(ivt.nilNode)
}

// rotateRight moves x so it is right of its left child
func (ivt *IntervalTree) rotateRight(x *intervalNode) {

	// rotateRight x must have left child
	if x.left == ivt.nilNode {
		return
	}

	y := x.left
	x.left = y.right
	if y.right != ivt.nilNode {
		y.right.parent = x
	}
	x.updateMax(ivt.nilNode)
	ivt.replaceParent(x, y)
	y.right = x
	y.updateMax(ivt.nilNode)
}

// replaceParent replaces x's parent with y
func (ivt *IntervalTree) replaceParent(x *intervalNode, y *intervalNode) {
	y.parent = x.parent
	if x.parent == ivt.nilNode {
		ivt.root = y
		//ivt.nilNode.left = ivt.root
		//ivt.nilNode.right = ivt.root
		//ivt.nilNode.parent = ivt.root
	} else {
		if x == x.parent.left {
			x.parent.left = y
		} else {
			x.parent.right = y
		}
		x.parent.updateMax(ivt.nilNode)
	}
	x.parent = y
}

// Len gives the number of elements in the tree
func (ivt *IntervalTree) Len() int { return ivt.count }

// Height is the number of levels in the tree; one node has height 1.
func (ivt *IntervalTree) Height() int { return ivt.root.height(ivt.nilNode) }

// MaxHeight is the expected maximum tree height given the number of nodes
func (ivt *IntervalTree) MaxHeight() int {
	return int((2 * math.Log2(float64(ivt.Len()+1))) + 0.5)
}

// IntervalVisitor is used on tree searches; return false to stop searching.
type IntervalVisitor func(n *IntervalValue) bool

// Visit calls a visitor function on every tree node intersecting the given interval.
// It will visit each interval [x, y) in ascending order sorted on x.
func (ivt *IntervalTree) Visit(ivl Interval, ivv IntervalVisitor) {
	ivt.root.visit(&ivl, ivt.nilNode, func(n *intervalNode) bool { return ivv(&n.iv) })
}

// find the exact node for a given interval
func (ivt *IntervalTree) find(ivl Interval) (ret *intervalNode) {
	// f := func(n *intervalNode) bool {
	// 	if n.iv.Ivl != ivl {
	// 		return true
	// 	}
	// 	ret = n
	// 	return false
	// }
	// ivt.root.visit(&ivl, ivt.nilNode, f)

	x := ivt.root

	for x != ivt.nilNode {
		if ivl.Begin.Compare(x.iv.Ivl.Begin) == 0 && ivl.End.Compare(x.iv.Ivl.End) == 0 {
			break
		}

		if ivl.End.Compare(x.max) > 0 {
			x = ivt.nilNode
		} else if ivl.Begin.Compare(x.iv.Ivl.Begin) < 0 {
			x = x.left
		} else {
			x = x.right
		}
	}

	return x
}

// Find gets the IntervalValue for the node matching the given interval
func (ivt *IntervalTree) Find(ivl Interval) (ret *IntervalValue) {
	n := ivt.find(ivl)
	if n == ivt.nilNode {
		return nil
	}
	return &n.iv
}

// Intersects returns true if there is some tree node intersecting the given interval.
func (ivt *IntervalTree) Intersects(iv Interval) bool {
	x := ivt.root
	for x != ivt.nilNode && iv.Compare(&x.iv.Ivl) != 0 {
		if x.left != ivt.nilNode && x.left.max.Compare(iv.Begin) > 0 {
			x = x.left
		} else {
			x = x.right
		}
	}
	return x != ivt.nilNode
}

// Contains returns true if the interval tree's keys cover the entire given interval.
func (ivt *IntervalTree) Contains(ivl Interval) bool {
	var maxEnd, minBegin Comparable

	isContiguous := true
	ivt.Visit(ivl, func(n *IntervalValue) bool {
		if minBegin == nil {
			minBegin = n.Ivl.Begin
			maxEnd = n.Ivl.End
			return true
		}
		if maxEnd.Compare(n.Ivl.Begin) < 0 {
			isContiguous = false
			return false
		}
		if n.Ivl.End.Compare(maxEnd) > 0 {
			maxEnd = n.Ivl.End
		}
		return true
	})

	return isContiguous && minBegin != nil && maxEnd.Compare(ivl.End) >= 0 && minBegin.Compare(ivl.Begin) <= 0
}

// Stab returns a slice with all elements in the tree intersecting the interval.
func (ivt *IntervalTree) Stab(iv Interval) (ivs []*IntervalValue) {
	if ivt.count == 0 {
		return nil
	}
	f := func(n *IntervalValue) bool { ivs = append(ivs, n); return true }
	ivt.Visit(iv, f)
	return ivs
}

// Union merges a given interval tree into the receiver.
func (ivt *IntervalTree) Union(inIvt *IntervalTree, ivl Interval) {
	f := func(n *IntervalValue) bool {
		ivt.Insert(n.Ivl, n.Val)
		return true
	}
	inIvt.Visit(ivl, f)
}

func (ivt *IntervalTree) LevelOrder() [][]*intervalNode {

	levels := make([][]*intervalNode, ivt.Height())

	queue := []*intervalNode{ivt.root}
	cur, last := 0, 1
	level := 0
	for cur < len(queue) {
		last = len(queue)
		levels[level] = []*intervalNode{}
		for cur < last {
			levels[level] = append(levels[level], queue[cur])
			if queue[cur].left != ivt.nilNode {
				queue = append(queue, queue[cur].left)
			}
			if queue[cur].right != ivt.nilNode {
				queue = append(queue, queue[cur].right)
			}
			cur++
		}

		level++
	}

	return levels
}

type StringComparable string

func (s StringComparable) Compare(c Comparable) int {
	sc := c.(StringComparable)
	if s < sc {
		return -1
	}
	if s > sc {
		return 1
	}
	return 0
}

func NewStringInterval(begin, end string) Interval {
	return Interval{StringComparable(begin), StringComparable(end)}
}

func NewStringPoint(s string) Interval {
	return Interval{StringComparable(s), StringComparable(s + "\x00")}
}

// StringAffineComparable treats "" as > all other strings
type StringAffineComparable string

func (s StringAffineComparable) Compare(c Comparable) int {
	sc := c.(StringAffineComparable)

	if len(s) == 0 {
		if len(sc) == 0 {
			return 0
		}
		return 1
	}
	if len(sc) == 0 {
		return -1
	}

	if s < sc {
		return -1
	}
	if s > sc {
		return 1
	}
	return 0
}

func NewStringAffineInterval(begin, end string) Interval {
	return Interval{StringAffineComparable(begin), StringAffineComparable(end)}
}
func NewStringAffinePoint(s string) Interval {
	return NewStringAffineInterval(s, s+"\x00")
}

func NewInt64Interval(a int64, b int64) Interval {
	return Interval{Int64Comparable(a), Int64Comparable(b)}
}

func NewInt64Point(a int64) Interval {
	return Interval{Int64Comparable(a), Int64Comparable(a + 1)}
}

type Int64Comparable int64

func (v Int64Comparable) Compare(c Comparable) int {
	vc := c.(Int64Comparable)
	cmp := v - vc
	if cmp < 0 {
		return -1
	}
	if cmp > 0 {
		return 1
	}
	return 0
}

// BytesAffineComparable treats empty byte arrays as > all other byte arrays
type BytesAffineComparable []byte

func (b BytesAffineComparable) Compare(c Comparable) int {
	bc := c.(BytesAffineComparable)

	if len(b) == 0 {
		if len(bc) == 0 {
			return 0
		}
		return 1
	}
	if len(bc) == 0 {
		return -1
	}

	return bytes.Compare(b, bc)
}

func NewBytesAffineInterval(begin, end []byte) Interval {
	return Interval{BytesAffineComparable(begin), BytesAffineComparable(end)}
}
func NewBytesAffinePoint(b []byte) Interval {
	be := make([]byte, len(b)+1)
	copy(be, b)
	be[len(b)] = 0
	return NewBytesAffineInterval(b, be)
}
