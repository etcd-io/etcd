// Copyright 2014 Will Fitzgerald. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file tests bit sets

package bitset

import (
	"encoding"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"
)

func TestStringer(t *testing.T) {
	v := New(0)
	for i := uint(0); i < 10; i++ {
		v.Set(i)
	}
	if v.String() != "{0,1,2,3,4,5,6,7,8,9}" {
		t.Error("bad string output")
	}
}

func TestStringLong(t *testing.T) {
	v := New(0)
	for i := uint(0); i < 262145; i++ {
		v.Set(i)
	}
	str := v.String()
	if len(str) != 1723903 {
		t.Error("Unexpected string length: ", len(str))
	}
}

func TestEmptyBitSet(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("A zero-length bitset should be fine")
		}
	}()
	b := New(0)
	if b.Len() != 0 {
		t.Errorf("Empty set should have capacity 0, not %d", b.Len())
	}
}

func TestZeroValueBitSet(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("A zero-length bitset should be fine")
		}
	}()
	var b BitSet
	if b.Len() != 0 {
		t.Errorf("Empty set should have capacity 0, not %d", b.Len())
	}
}

func TestBitSetNew(t *testing.T) {
	v := New(16)
	if v.Test(0) {
		t.Errorf("Unable to make a bit set and read its 0th value.")
	}
}

func TestBitSetHuge(t *testing.T) {
	v := New(uint(math.MaxUint32))
	if v.Test(0) {
		t.Errorf("Unable to make a huge bit set and read its 0th value.")
	}
}

func TestLen(t *testing.T) {
	v := New(1000)
	if v.Len() != 1000 {
		t.Errorf("Len should be 1000, but is %d.", v.Len())
	}
}

func TestBitSetIsClear(t *testing.T) {
	v := New(1000)
	for i := uint(0); i < 1000; i++ {
		if v.Test(i) {
			t.Errorf("Bit %d is set, and it shouldn't be.", i)
		}
	}
}

func TestExendOnBoundary(t *testing.T) {
	v := New(32)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Border out of index error should not have caused a panic")
		}
	}()
	v.Set(32)
}

func TestExpand(t *testing.T) {
	v := New(0)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Expansion should not have caused a panic")
		}
	}()
	for i := uint(0); i < 1000; i++ {
		v.Set(i)
	}
}

func TestBitSetAndGet(t *testing.T) {
	v := New(1000)
	v.Set(100)
	if !v.Test(100) {
		t.Errorf("Bit %d is clear, and it shouldn't be.", 100)
	}
}

func TestNextClear(t *testing.T) {
	v := New(1000)
	v.Set(0).Set(1)
	next, found := v.NextClear(0)
	if !found || next != 2 {
		t.Errorf("Found next clear bit as %d, it should have been 2", next)
	}

	v = New(1000)
	for i := uint(0); i < 66; i++ {
		v.Set(i)
	}
	next, found = v.NextClear(0)
	if !found || next != 66 {
		t.Errorf("Found next clear bit as %d, it should have been 66", next)
	}

	v = New(1000)
	for i := uint(0); i < 64; i++ {
		v.Set(i)
	}
	v.Clear(45)
	v.Clear(52)
	next, found = v.NextClear(10)
	if !found || next != 45 {
		t.Errorf("Found next clear bit as %d, it should have been 45", next)
	}

	v = New(1000)
	for i := uint(0); i < 128; i++ {
		v.Set(i)
	}
	v.Clear(73)
	v.Clear(99)
	next, found = v.NextClear(10)
	if !found || next != 73 {
		t.Errorf("Found next clear bit as %d, it should have been 73", next)
	}

	next, found = v.NextClear(72)
	if !found || next != 73 {
		t.Errorf("Found next clear bit as %d, it should have been 73", next)
	}
	next, found = v.NextClear(73)
	if !found || next != 73 {
		t.Errorf("Found next clear bit as %d, it should have been 73", next)
	}
	next, found = v.NextClear(74)
	if !found || next != 99 {
		t.Errorf("Found next clear bit as %d, it should have been 73", next)
	}

	v = New(128)
	next, found = v.NextClear(0)
	if !found || next != 0 {
		t.Errorf("Found next clear bit as %d, it should have been 0", next)
	}

	for i := uint(0); i < 128; i++ {
		v.Set(i)
	}
	_, found = v.NextClear(0)
	if found {
		t.Errorf("There are not clear bits")
	}

	b := new(BitSet)
	c, d := b.NextClear(1)
	if c != 0 || d {
		t.Error("Unexpected values")
		return
	}

	v = New(100)
	for i := uint(0); i != 100; i++ {
		v.Set(i)
	}
	next, found = v.NextClear(0)
	if found || next != 0 {
		t.Errorf("Found next clear bit as %d, it should have return (0, false)", next)

	}
}

func TestIterate(t *testing.T) {
	v := New(10000)
	v.Set(0)
	v.Set(1)
	v.Set(2)
	data := make([]uint, 3)
	c := 0
	for i, e := v.NextSet(0); e; i, e = v.NextSet(i + 1) {
		data[c] = i
		c++
	}
	if data[0] != 0 {
		t.Errorf("bug 0")
	}
	if data[1] != 1 {
		t.Errorf("bug 1")
	}
	if data[2] != 2 {
		t.Errorf("bug 2")
	}
	v.Set(10)
	v.Set(2000)
	data = make([]uint, 5)
	c = 0
	for i, e := v.NextSet(0); e; i, e = v.NextSet(i + 1) {
		data[c] = i
		c++
	}
	if data[0] != 0 {
		t.Errorf("bug 0")
	}
	if data[1] != 1 {
		t.Errorf("bug 1")
	}
	if data[2] != 2 {
		t.Errorf("bug 2")
	}
	if data[3] != 10 {
		t.Errorf("bug 3")
	}
	if data[4] != 2000 {
		t.Errorf("bug 4")
	}

}

func TestSetTo(t *testing.T) {
	v := New(1000)
	v.SetTo(100, true)
	if !v.Test(100) {
		t.Errorf("Bit %d is clear, and it shouldn't be.", 100)
	}
	v.SetTo(100, false)
	if v.Test(100) {
		t.Errorf("Bit %d is set, and it shouldn't be.", 100)
	}
}

func TestChain(t *testing.T) {
	if !New(1000).Set(100).Set(99).Clear(99).Test(100) {
		t.Errorf("Bit %d is clear, and it shouldn't be.", 100)
	}
}

func TestOutOfBoundsLong(t *testing.T) {
	v := New(64)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Long distance out of index error should not have caused a panic")
		}
	}()
	v.Set(1000)
}

func TestOutOfBoundsClose(t *testing.T) {
	v := New(65)
	defer func() {
		if r := recover(); r != nil {
			t.Error("Local out of index error should not have caused a panic")
		}
	}()
	v.Set(66)
}

func TestCount(t *testing.T) {
	tot := uint(64*4 + 11) // just some multi unit64 number
	v := New(tot)
	checkLast := true
	for i := uint(0); i < tot; i++ {
		sz := uint(v.Count())
		if sz != i {
			t.Errorf("Count reported as %d, but it should be %d", sz, i)
			checkLast = false
			break
		}
		v.Set(i)
	}
	if checkLast {
		sz := uint(v.Count())
		if sz != tot {
			t.Errorf("After all bits set, size reported as %d, but it should be %d", sz, tot)
		}
	}
}

// test setting every 3rd bit, just in case something odd is happening
func TestCount2(t *testing.T) {
	tot := uint(64*4 + 11) // just some multi unit64 number
	v := New(tot)
	for i := uint(0); i < tot; i += 3 {
		sz := uint(v.Count())
		if sz != i/3 {
			t.Errorf("Count reported as %d, but it should be %d", sz, i)
			break
		}
		v.Set(i)
	}
}

// nil tests
func TestNullTest(t *testing.T) {
	var v *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Checking bit of null reference should have caused a panic")
		}
	}()
	v.Test(66)
}

func TestNullSet(t *testing.T) {
	var v *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Setting bit of null reference should have caused a panic")
		}
	}()
	v.Set(66)
}

func TestNullClear(t *testing.T) {
	var v *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Clearning bit of null reference should have caused a panic")
		}
	}()
	v.Clear(66)
}

func TestNullCount(t *testing.T) {
	var v *BitSet
	defer func() {
		if r := recover(); r != nil {
			t.Error("Counting null reference should not have caused a panic")
		}
	}()
	cnt := v.Count()
	if cnt != 0 {
		t.Errorf("Count reported as %d, but it should be 0", cnt)
	}
}

func TestPanicDifferenceBNil(t *testing.T) {
	var b *BitSet
	var compare = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil First should should have caused a panic")
		}
	}()
	b.Difference(compare)
}

func TestPanicDifferenceCompareNil(t *testing.T) {
	var compare *BitSet
	var b = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil Second should should have caused a panic")
		}
	}()
	b.Difference(compare)
}

func TestPanicUnionBNil(t *testing.T) {
	var b *BitSet
	var compare = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil First should should have caused a panic")
		}
	}()
	b.Union(compare)
}

func TestPanicUnionCompareNil(t *testing.T) {
	var compare *BitSet
	var b = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil Second should should have caused a panic")
		}
	}()
	b.Union(compare)
}

func TestPanicIntersectionBNil(t *testing.T) {
	var b *BitSet
	var compare = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil First should should have caused a panic")
		}
	}()
	b.Intersection(compare)
}

func TestPanicIntersectionCompareNil(t *testing.T) {
	var compare *BitSet
	var b = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil Second should should have caused a panic")
		}
	}()
	b.Intersection(compare)
}

func TestPanicSymmetricDifferenceBNil(t *testing.T) {
	var b *BitSet
	var compare = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil First should should have caused a panic")
		}
	}()
	b.SymmetricDifference(compare)
}

func TestPanicSymmetricDifferenceCompareNil(t *testing.T) {
	var compare *BitSet
	var b = New(10)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil Second should should have caused a panic")
		}
	}()
	b.SymmetricDifference(compare)
}

func TestPanicComplementBNil(t *testing.T) {
	var b *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil should should have caused a panic")
		}
	}()
	b.Complement()
}

func TestPanicAnytBNil(t *testing.T) {
	var b *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil should should have caused a panic")
		}
	}()
	b.Any()
}

func TestPanicNonetBNil(t *testing.T) {
	var b *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil should should have caused a panic")
		}
	}()
	b.None()
}

func TestPanicAlltBNil(t *testing.T) {
	var b *BitSet
	defer func() {
		if r := recover(); r == nil {
			t.Error("Nil should should have caused a panic")
		}
	}()
	b.All()
}

func TestAll(t *testing.T) {
	v := New(0)
	if !v.All() {
		t.Error("Empty sets should return true on All()")
	}
	v = New(2)
	v.SetTo(0, true)
	v.SetTo(1, true)
	if !v.All() {
		t.Error("Non-empty sets with all bits set should return true on All()")
	}
	v = New(2)
	if v.All() {
		t.Error("Non-empty sets with no bits set should return false on All()")
	}
	v = New(2)
	v.SetTo(0, true)
	if v.All() {
		t.Error("Non-empty sets with some bits set should return false on All()")
	}
}

func TestShrink(t *testing.T) {
	b := New(0)

	b.Set(0)
	b.Set(1)
	b.Set(2)
	b.Set(3)
	b.Set(64)
	b.Shrink(2)
	if !b.Test(0) {
		t.Error("0 should be set")
		return
	}
	if !b.Test(1) {
		t.Error("1 should be set")
		return
	}
	if !b.Test(2) {
		t.Error("2 should be set")
		return
	}
	if b.Test(3) {
		t.Error("3 should not be set")
		return
	}
	if b.Test(64) {
		t.Error("64 should not be set")
		return
	}

	b.Set(24)
	b.Shrink(100)
	if !b.Test(24) {
		t.Error("24 should be set")
		return
	}

	b.Set(127)
	b.Set(128)
	b.Set(129)
	b.Shrink(128)
	if !b.Test(127) {
		t.Error("127 should be set")
		return
	}
	if !b.Test(128) {
		t.Error("128 should be set")
		return
	}
	if b.Test(129) {
		t.Error("129 should not be set")
		return
	}

	b.Set(129)
	b.Shrink(129)
	if !b.Test(129) {
		t.Error("129 should be set")
		return
	}

	b.Set(1000)
	b.Set(2000)
	b.Set(3000)
	b.Shrink(3000)
	if len(b.set) != 3000/64+1 {
		t.Error("Wrong length of BitSet.set")
		return
	}
	if !b.Test(3000) {
		t.Error("3000 should be set")
		return
	}

	b.Shrink(2000)
	if len(b.set) != 2000/64+1 {
		t.Error("Wrong length of BitSet.set")
		return
	}
	if b.Test(3000) {
		t.Error("3000 should not be set")
		return
	}
	if !b.Test(2000) {
		t.Error("2000 should be set")
		return
	}
	if !b.Test(1000) {
		t.Error("1000 should be set")
		return
	}
	if !b.Test(24) {
		t.Error("24 should be set")
		return
	}
}

func TestInsertAtWithSet(t *testing.T) {
	b := New(0)
	b.Set(0)
	b.Set(1)
	b.Set(63)
	b.Set(64)
	b.Set(65)

	b.InsertAt(3)
	if !b.Test(0) {
		t.Error("0 should be set")
		return
	}
	if !b.Test(1) {
		t.Error("1 should be set")
		return
	}
	if b.Test(3) {
		t.Error("3 should not be set")
		return
	}
	if !b.Test(64) {
		t.Error("64 should be set")
		return
	}
	if !b.Test(65) {
		t.Error("65 should be set")
		return
	}
	if !b.Test(66) {
		t.Error("66 should be set")
		return
	}

}

func TestInsertAt(t *testing.T) {
	type testCase struct {
		input     []string
		insertIdx uint
		expected  []string
	}

	testCases := []testCase{
		{
			input: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
			},
			insertIdx: uint(62),
			expected: []string{
				"1011111111111111111111111111111111111111111111111111111111111111",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		{
			input: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
			},
			insertIdx: uint(63),
			expected: []string{
				"0111111111111111111111111111111111111111111111111111111111111111",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		{
			input: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
			},
			insertIdx: uint(0),
			expected: []string{
				"1111111111111111111111111111111111111111111111111111111111111110",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		{
			input: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
				"1111111111111111111111111111111111111111111111111111111111111111",
				"1111111111111111111111111111111111111111111111111111111111111111",
			},
			insertIdx: uint(70),
			expected: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
				"1111111111111111111111111111111111111111111111111111111110111111",
				"1111111111111111111111111111111111111111111111111111111111111111",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		{
			input: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
				"1111111111111111111111111111111111111111111111111111111111111111",
				"1111111111111111111111111111111111111111111111111111111111110000",
			},
			insertIdx: uint(70),
			expected: []string{
				"1111111111111111111111111111111111111111111111111111111111111111",
				"1111111111111111111111111111111111111111111111111111111110111111",
				"1111111111111111111111111111111111111111111111111111111111100001",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		{
			input: []string{
				"1111111111111111111111111111111111111111111111111111111111110000",
			},
			insertIdx: uint(10),
			expected: []string{
				"1111111111111111111111111111111111111111111111111111101111110000",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
	}

	for _, tc := range testCases {
		var input []uint64
		for _, inputElement := range tc.input {
			parsed, _ := strconv.ParseUint(inputElement, 2, 64)
			input = append(input, parsed)
		}

		var expected []uint64
		for _, expectedElement := range tc.expected {
			parsed, _ := strconv.ParseUint(expectedElement, 2, 64)
			expected = append(expected, parsed)
		}

		b := From(input)
		b.InsertAt(tc.insertIdx)
		if len(b.set) != len(expected) {
			t.Error("Length of sets should be equal")
			return
		}
		for i := range b.set {
			if b.set[i] != expected[i] {
				t.Error("Unexpected results found in set")
				return
			}
		}
	}
}

func TestNone(t *testing.T) {
	v := New(0)
	if !v.None() {
		t.Error("Empty sets should return true on None()")
	}
	v = New(2)
	v.SetTo(0, true)
	v.SetTo(1, true)
	if v.None() {
		t.Error("Non-empty sets with all bits set should return false on None()")
	}
	v = New(2)
	if !v.None() {
		t.Error("Non-empty sets with no bits set should return true on None()")
	}
	v = New(2)
	v.SetTo(0, true)
	if v.None() {
		t.Error("Non-empty sets with some bits set should return false on None()")
	}
	v = new(BitSet)
	if !v.None() {
		t.Error("Empty sets should return true on None()")
	}
}

func TestEqual(t *testing.T) {
	a := New(100)
	b := New(99)
	c := New(100)
	if a.Equal(b) {
		t.Error("Sets of different sizes should be not be equal")
	}
	if !a.Equal(c) {
		t.Error("Two empty sets of the same size should be equal")
	}
	a.Set(99)
	c.Set(0)
	if a.Equal(c) {
		t.Error("Two sets with differences should not be equal")
	}
	c.Set(99)
	a.Set(0)
	if !a.Equal(c) {
		t.Error("Two sets with the same bits set should be equal")
	}
	if a.Equal(nil) {
		t.Error("The sets should be different")
	}
	a = New(0)
	b = New(0)
	if !a.Equal(b) {
		t.Error("Two empty set should be equal")
	}
}

func TestUnion(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)
		b.Set(i - 1)
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	if a.UnionCardinality(b) != 200 {
		t.Errorf("Union should have 200 bits set, but had %d", a.UnionCardinality(b))
	}
	if a.UnionCardinality(b) != b.UnionCardinality(a) {
		t.Errorf("Union should be symmetric")
	}

	c := a.Union(b)
	d := b.Union(a)
	if c.Count() != 200 {
		t.Errorf("Union should have 200 bits set, but had %d", c.Count())
	}
	if !c.Equal(d) {
		t.Errorf("Union should be symmetric")
	}
}

func TestInPlaceUnion(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)
		b.Set(i - 1)
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	c := a.Clone()
	c.InPlaceUnion(b)
	d := b.Clone()
	d.InPlaceUnion(a)
	if c.Count() != 200 {
		t.Errorf("Union should have 200 bits set, but had %d", c.Count())
	}
	if d.Count() != 200 {
		t.Errorf("Union should have 200 bits set, but had %d", d.Count())
	}
	if !c.Equal(d) {
		t.Errorf("Union should be symmetric")
	}
}

func TestIntersection(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)
		b.Set(i - 1).Set(i)
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	if a.IntersectionCardinality(b) != 50 {
		t.Errorf("Intersection should have 50 bits set, but had %d", a.IntersectionCardinality(b))
	}
	if a.IntersectionCardinality(b) != b.IntersectionCardinality(a) {
		t.Errorf("Intersection should be symmetric")
	}
	c := a.Intersection(b)
	d := b.Intersection(a)
	if c.Count() != 50 {
		t.Errorf("Intersection should have 50 bits set, but had %d", c.Count())
	}
	if !c.Equal(d) {
		t.Errorf("Intersection should be symmetric")
	}
}

func TestInplaceIntersection(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)
		b.Set(i - 1).Set(i)
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	c := a.Clone()
	c.InPlaceIntersection(b)
	d := b.Clone()
	d.InPlaceIntersection(a)
	if c.Count() != 50 {
		t.Errorf("Intersection should have 50 bits set, but had %d", c.Count())
	}
	if d.Count() != 50 {
		t.Errorf("Intersection should have 50 bits set, but had %d", d.Count())
	}
	if !c.Equal(d) {
		t.Errorf("Intersection should be symmetric")
	}
}

func TestDifference(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)
		b.Set(i - 1)
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	if a.DifferenceCardinality(b) != 50 {
		t.Errorf("a-b Difference should have 50 bits set, but had %d", a.DifferenceCardinality(b))
	}
	if b.DifferenceCardinality(a) != 150 {
		t.Errorf("b-a Difference should have 150 bits set, but had %d", b.DifferenceCardinality(a))
	}

	c := a.Difference(b)
	d := b.Difference(a)
	if c.Count() != 50 {
		t.Errorf("a-b Difference should have 50 bits set, but had %d", c.Count())
	}
	if d.Count() != 150 {
		t.Errorf("b-a Difference should have 150 bits set, but had %d", d.Count())
	}
	if c.Equal(d) {
		t.Errorf("Difference, here, should not be symmetric")
	}
}

func TestInPlaceDifference(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)
		b.Set(i - 1)
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	c := a.Clone()
	c.InPlaceDifference(b)
	d := b.Clone()
	d.InPlaceDifference(a)
	if c.Count() != 50 {
		t.Errorf("a-b Difference should have 50 bits set, but had %d", c.Count())
	}
	if d.Count() != 150 {
		t.Errorf("b-a Difference should have 150 bits set, but had %d", d.Count())
	}
	if c.Equal(d) {
		t.Errorf("Difference, here, should not be symmetric")
	}
}

func TestSymmetricDifference(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)            // 01010101010 ... 0000000
		b.Set(i - 1).Set(i) // 11111111111111111000000
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	if a.SymmetricDifferenceCardinality(b) != 150 {
		t.Errorf("a^b Difference should have 150 bits set, but had %d", a.SymmetricDifferenceCardinality(b))
	}
	if b.SymmetricDifferenceCardinality(a) != 150 {
		t.Errorf("b^a Difference should have 150 bits set, but had %d", b.SymmetricDifferenceCardinality(a))
	}

	c := a.SymmetricDifference(b)
	d := b.SymmetricDifference(a)
	if c.Count() != 150 {
		t.Errorf("a^b Difference should have 150 bits set, but had %d", c.Count())
	}
	if d.Count() != 150 {
		t.Errorf("b^a Difference should have 150 bits set, but had %d", d.Count())
	}
	if !c.Equal(d) {
		t.Errorf("SymmetricDifference should be symmetric")
	}
}

func TestInPlaceSymmetricDifference(t *testing.T) {
	a := New(100)
	b := New(200)
	for i := uint(1); i < 100; i += 2 {
		a.Set(i)            // 01010101010 ... 0000000
		b.Set(i - 1).Set(i) // 11111111111111111000000
	}
	for i := uint(100); i < 200; i++ {
		b.Set(i)
	}
	c := a.Clone()
	c.InPlaceSymmetricDifference(b)
	d := b.Clone()
	d.InPlaceSymmetricDifference(a)
	if c.Count() != 150 {
		t.Errorf("a^b Difference should have 150 bits set, but had %d", c.Count())
	}
	if d.Count() != 150 {
		t.Errorf("b^a Difference should have 150 bits set, but had %d", d.Count())
	}
	if !c.Equal(d) {
		t.Errorf("SymmetricDifference should be symmetric")
	}
}

func TestComplement(t *testing.T) {
	a := New(50)
	b := a.Complement()
	if b.Count() != 50 {
		t.Errorf("Complement failed, size should be 50, but was %d", b.Count())
	}
	a = New(50)
	a.Set(10).Set(20).Set(42)
	b = a.Complement()
	if b.Count() != 47 {
		t.Errorf("Complement failed, size should be 47, but was %d", b.Count())
	}
}

func TestIsSuperSet(t *testing.T) {
	a := New(500)
	b := New(300)
	c := New(200)

	// Setup bitsets
	// a and b overlap
	// only c is (strict) super set
	for i := uint(0); i < 100; i++ {
		a.Set(i)
	}
	for i := uint(50); i < 150; i++ {
		b.Set(i)
	}
	for i := uint(0); i < 200; i++ {
		c.Set(i)
	}

	if a.IsSuperSet(b) {
		t.Errorf("IsSuperSet fails")
	}
	if a.IsSuperSet(c) {
		t.Errorf("IsSuperSet fails")
	}
	if b.IsSuperSet(a) {
		t.Errorf("IsSuperSet fails")
	}
	if b.IsSuperSet(c) {
		t.Errorf("IsSuperSet fails")
	}
	if !c.IsSuperSet(a) {
		t.Errorf("IsSuperSet fails")
	}
	if !c.IsSuperSet(b) {
		t.Errorf("IsSuperSet fails")
	}

	if a.IsStrictSuperSet(b) {
		t.Errorf("IsStrictSuperSet fails")
	}
	if a.IsStrictSuperSet(c) {
		t.Errorf("IsStrictSuperSet fails")
	}
	if b.IsStrictSuperSet(a) {
		t.Errorf("IsStrictSuperSet fails")
	}
	if b.IsStrictSuperSet(c) {
		t.Errorf("IsStrictSuperSet fails")
	}
	if !c.IsStrictSuperSet(a) {
		t.Errorf("IsStrictSuperSet fails")
	}
	if !c.IsStrictSuperSet(b) {
		t.Errorf("IsStrictSuperSet fails")
	}
}

func TestDumpAsBits(t *testing.T) {
	a := New(10).Set(10)
	astr := "0000000000000000000000000000000000000000000000000000010000000000."
	if a.DumpAsBits() != astr {
		t.Errorf("DumpAsBits failed, output should be \"%s\" but was \"%s\"", astr, a.DumpAsBits())
	}
	var b BitSet // zero value (b.set == nil)
	bstr := "."
	if b.DumpAsBits() != bstr {
		t.Errorf("DumpAsBits failed, output should be \"%s\" but was \"%s\"", bstr, b.DumpAsBits())
	}
}

func TestMarshalUnmarshalBinary(t *testing.T) {
	a := New(1010).Set(10).Set(1001)
	b := new(BitSet)

	copyBinary(t, a, b)

	// BitSets must be equal after marshalling and unmarshalling
	if !a.Equal(b) {
		t.Error("Bitsets are not equal:\n\t", a.DumpAsBits(), "\n\t", b.DumpAsBits())
		return
	}
}

func TestMarshalUnmarshalBinaryByLittleEndian(t *testing.T) {
	LittleEndian()
	a := New(1010).Set(10).Set(1001)
	b := new(BitSet)

	copyBinary(t, a, b)

	// BitSets must be equal after marshalling and unmarshalling
	if !a.Equal(b) {
		t.Error("Bitsets are not equal:\n\t", a.DumpAsBits(), "\n\t", b.DumpAsBits())
		return
	}
}

func copyBinary(t *testing.T, from encoding.BinaryMarshaler, to encoding.BinaryUnmarshaler) {
	data, err := from.MarshalBinary()
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	err = to.UnmarshalBinary(data)
	if err != nil {
		t.Errorf(err.Error())
		return
	}
}

func TestMarshalUnmarshalJSON(t *testing.T) {
	a := New(1010).Set(10).Set(1001)
	data, err := json.Marshal(a)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	b := new(BitSet)
	err = json.Unmarshal(data, b)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	// Bitsets must be equal after marshalling and unmarshalling
	if !a.Equal(b) {
		t.Error("Bitsets are not equal:\n\t", a.DumpAsBits(), "\n\t", b.DumpAsBits())
		return
	}
}

func TestMarshalUnmarshalJSONByStdEncoding(t *testing.T) {
	Base64StdEncoding()
	a := New(1010).Set(10).Set(1001)
	data, err := json.Marshal(a)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	b := new(BitSet)
	err = json.Unmarshal(data, b)
	if err != nil {
		t.Errorf(err.Error())
		return
	}

	// Bitsets must be equal after marshalling and unmarshalling
	if !a.Equal(b) {
		t.Error("Bitsets are not equal:\n\t", a.DumpAsBits(), "\n\t", b.DumpAsBits())
		return
	}
}

func TestSafeSet(t *testing.T) {
	b := new(BitSet)
	c := b.safeSet()
	outType := fmt.Sprintf("%T", c)
	expType := "[]uint64"
	if outType != expType {
		t.Error("Expecting type: ", expType, ", gotf:", outType)
		return
	}
	if len(c) != 0 {
		t.Error("The slice should be empty")
		return
	}
}

func TestFrom(t *testing.T) {
	u := []uint64{2, 3, 5, 7, 11}
	b := From(u)
	outType := fmt.Sprintf("%T", b)
	expType := "*bitset.BitSet"
	if outType != expType {
		t.Error("Expecting type: ", expType, ", gotf:", outType)
		return
	}
}

func TestBytes(t *testing.T) {
	b := new(BitSet)
	c := b.Bytes()
	outType := fmt.Sprintf("%T", c)
	expType := "[]uint64"
	if outType != expType {
		t.Error("Expecting type: ", expType, ", gotf:", outType)
		return
	}
	if len(c) != 0 {
		t.Error("The slice should be empty")
		return
	}
}

func TestCap(t *testing.T) {
	c := Cap()
	if c <= 0 {
		t.Error("The uint capacity should be >= 0")
		return
	}
}

func TestWordsNeededLong(t *testing.T) {
	i := Cap()
	out := wordsNeeded(i)
	if out <= 0 {
		t.Error("Unexpected value: ", out)
		return
	}
}

func TestTestTooLong(t *testing.T) {
	b := new(BitSet)
	if b.Test(1) {
		t.Error("Unexpected value: true")
		return
	}
}

func TestClearTooLong(t *testing.T) {
	b := new(BitSet)
	c := b.Clear(1)
	if b != c {
		t.Error("Unexpected value")
		return
	}
}

func TestClearAll(t *testing.T) {
	u := []uint64{2, 3, 5, 7, 11}
	b := From(u)
	c := b.ClearAll()
	if c.length != 320 {
		t.Error("Unexpected length: ", b.length)
		return
	}
	if c.Test(0) || c.Test(1) || c.Test(2) || c.Test(3) || c.Test(4) || c.Test(5) {
		t.Error("All bits should be unset")
		return
	}
}

func TestFlip(t *testing.T) {
	b := new(BitSet)
	c := b.Flip(11)
	if c.length != 12 {
		t.Error("Unexpected value: ", c.length)
		return
	}
	d := c.Flip(7)
	if d.length != 12 {
		t.Error("Unexpected value: ", d.length)
		return
	}
}

func TestCopy(t *testing.T) {
	a := New(10)
	if a.Copy(nil) != 0 {
		t.Error("No values should be copied")
		return
	}
	a = New(10)
	b := New(20)
	if a.Copy(b) != 10 {
		t.Error("Unexpected value")
		return
	}
}

func TestNextSetError(t *testing.T) {
	b := new(BitSet)
	c, d := b.NextSet(1)
	if c != 0 || d {
		t.Error("Unexpected values")
		return
	}
}

func TestDeleteWithBitStrings(t *testing.T) {
	type testCase struct {
		input     []string
		deleteIdx uint
		expected  []string
	}

	testCases := []testCase{
		{
			input: []string{
				"1110000000000000000000000000000000000000000000000000000000000001",
			},
			deleteIdx: uint(63),
			expected: []string{
				"0110000000000000000000000000000000000000000000000000000000000001",
			},
		},
		{
			input: []string{
				"1000000000000000000000000000000000000000000000000000000000010101",
			},
			deleteIdx: uint(0),
			expected: []string{
				"0100000000000000000000000000000000000000000000000000000000001010",
			},
		},
		{
			input: []string{
				"0000000000000000000000000000000000000000000000000000000000111000",
			},
			deleteIdx: uint(4),
			expected: []string{
				"0000000000000000000000000000000000000000000000000000000000011000",
			},
		},
		{
			input: []string{
				"1000000000000000000000000000000000000000000000000000000000000001",
				"1010000000000000000000000000000000000000000000000000000000000001",
			},
			deleteIdx: uint(63),
			expected: []string{
				"1000000000000000000000000000000000000000000000000000000000000001",
				"0101000000000000000000000000000000000000000000000000000000000000",
			},
		},
		{
			input: []string{
				"1000000000000000000000000000000000000000000000000000000000000000",
				"1000000000000000000000000000000000000000000000000000000000000001",
				"1000000000000000000000000000000000000000000000000000000000000001",
			},
			deleteIdx: uint(64),
			expected: []string{
				"1000000000000000000000000000000000000000000000000000000000000000",
				"1100000000000000000000000000000000000000000000000000000000000000",
				"0100000000000000000000000000000000000000000000000000000000000000",
			},
		},
		{
			input: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
			},
			deleteIdx: uint(256),
			expected: []string{
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000001",
				"0000000000000000000000000000000000000000000000000000000000000000",
			},
		},
	}

	for _, tc := range testCases {
		var input []uint64
		for _, inputElement := range tc.input {
			parsed, _ := strconv.ParseUint(inputElement, 2, 64)
			input = append(input, parsed)
		}

		var expected []uint64
		for _, expectedElement := range tc.expected {
			parsed, _ := strconv.ParseUint(expectedElement, 2, 64)
			expected = append(expected, parsed)
		}

		b := From(input)
		b.DeleteAt(tc.deleteIdx)
		if len(b.set) != len(expected) {
			t.Errorf("Length of sets expected to be %d, but was %d", len(expected), len(b.set))
			return
		}
		for i := range b.set {
			if b.set[i] != expected[i] {
				t.Errorf("Unexpected output\nExpected: %b\nGot:      %b", expected[i], b.set[i])
				return
			}
		}
	}
}

func TestDeleteWithBitSetInstance(t *testing.T) {
	length := uint(256)
	bitSet := New(length)

	// the indexes that get set in the bit set
	indexesToSet := []uint{0, 1, 126, 127, 128, 129, 170, 171, 200, 201, 202, 203, 255}

	// the position we delete from the bitset
	deleteAt := uint(127)

	// the indexes that we expect to be set after the delete
	expectedToBeSet := []uint{0, 1, 126, 127, 128, 169, 170, 199, 200, 201, 202, 254}

	expected := make(map[uint]struct{})
	for _, index := range expectedToBeSet {
		expected[index] = struct{}{}
	}

	for _, index := range indexesToSet {
		bitSet.Set(index)
	}

	bitSet.DeleteAt(deleteAt)

	for i := uint(0); i < length; i++ {
		if _, ok := expected[i]; ok {
			if !bitSet.Test(i) {
				t.Errorf("Expected index %d to be set, but wasn't", i)
			}
		} else {
			if bitSet.Test(i) {
				t.Errorf("Expected index %d to not be set, but was", i)
			}
		}

	}
}
