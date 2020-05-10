// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text

import "testing"

// -=-= Extent =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

func TestExtent(t *testing.T) {
	ol := &Extent{Offset: 5, Length: 20}
	assertEquals("offset 5, length 20", ol.String(), t)
}

func TestExtentIntersect(t *testing.T) {
	ol15 := &Extent{Offset: 1, Length: 5}
	ol30 := &Extent{Offset: 3, Length: 0}
	ol33 := &Extent{Offset: 3, Length: 3}
	ol51 := &Extent{Offset: 5, Length: 1}
	ol61 := &Extent{Offset: 6, Length: 1}

	type test struct {
		ol1, ol2 *Extent
		expect   string
	}

	tests := []test{
		{ol15, ol15, "offset 1, length 5"},
		{ol15, ol30, "offset 3, length 0"},
		{ol15, ol33, "offset 3, length 3"},
		{ol15, ol51, "offset 5, length 1"},
		{ol15, ol61, ""},

		{ol30, ol15, "offset 3, length 0"},
		{ol30, ol30, ""},
		{ol30, ol33, ""},
		{ol30, ol51, ""},
		{ol30, ol61, ""},

		{ol33, ol15, "offset 3, length 3"},
		{ol33, ol30, ""},
		{ol33, ol33, "offset 3, length 3"},
		{ol33, ol51, "offset 5, length 1"},
		{ol33, ol61, ""},

		{ol51, ol15, "offset 5, length 1"},
		{ol51, ol30, ""},
		{ol51, ol33, "offset 5, length 1"},
		{ol51, ol51, "offset 5, length 1"},
		{ol51, ol61, ""},

		{ol61, ol15, ""},
		{ol61, ol30, ""},
		{ol61, ol33, ""},
		{ol61, ol51, ""},
		{ol61, ol61, "offset 6, length 1"},
	}

	for _, tst := range tests {
		overlap := tst.ol1.Intersect(tst.ol2)
		if overlap == nil && tst.expect != "" {
			t.Fatalf("%s ∩ %s produced nil, expected %s",
				tst.ol1, tst.ol2, tst.expect)
		} else if overlap != nil && tst.expect != overlap.String() {
			t.Fatalf("%s ∩ %s produced %s, expected %s",
				tst.ol1, tst.ol2, overlap, tst.expect)
		}
	}
}

// -=-= EditSet -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

func applyToString(e *EditSet, s string) string {
	result, err := ApplyToString(e, s)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	return result
}

func TestSizeChange(t *testing.T) {
	es := NewEditSet()
	es.Add(&Extent{2, 5}, "x") // replace 5 bytes with 1 (-4)
	es.Add(&Extent{7, 0}, "6") // add 1 byte
	if es.SizeChange() != -3 {
		t.Fatalf("SizeChange: expected -3, got %d", es.SizeChange())
	}

	es = NewEditSet()
	hello := "こんにちは"
	es.Add(&Extent{6, 5}, hello)
	chg := len(hello) - 5
	if es.SizeChange() != int64(chg) {
		t.Fatalf("SizeChange: chged %d, got %d", chg, es.SizeChange())
	}
}

func TestNewOldOffset(t *testing.T) {
	es := NewEditSet()
	es.Add(&Extent{2, 5}, "x") // replace 5 bytes with 1 (-4)
	es.Add(&Extent{7, 0}, "6") // add 1 byte

	// NewOffset  "--------" -> "--x6--"
	offset := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	expect := []int{0, 1, 2, 2, 2, 2, 2, 3, 5}
	for i := range offset {
		actual := es.NewOffset(offset[i])
		if actual != expect[i] {
			t.Fatalf("NewOffset(%d): expected %d, got %d",
				offset[i], expect[i], actual)
		}
	}

	// OldOffset
	offset = []int{0, 1, 2, 3, 4, 5}
	expect = []int{0, 1, 2, 7, 7, 8}
	for i := range offset {
		actual := es.OldOffset(offset[i])
		if actual != expect[i] {
			t.Fatalf("OldOffset(%d): expected %d, got %d",
				offset[i], expect[i], actual)
		}
	}

}

func TestEditString(t *testing.T) {
	es := NewEditSet()
	assertEquals("", es.String(), t)

	es.Add(&Extent{5, 6}, "x")
	es.Add(&Extent{1, 2}, "y")
	es.Add(&Extent{3, 1}, "z")
	assertEquals(`Replace offset 1, length 2 with "y"
Replace offset 3, length 1 with "z"
Replace offset 5, length 6 with "x"
`, es.String(), t)
}

func TestOverlap(t *testing.T) {
	type test struct {
		offset, length  int
		overlapExpected bool // Does this overlap Extent{3,4}?
	}

	//                                                   123456789
	// Which intervals should overlap Extent{3,4}?   |--|
	tests := []test{
		{2, 1, false}, // Regions starting to the left of offset 3
		{2, 2, true},
		{3, 0, false}, // Regions starting inside the interval
		{3, 1, true},
		{3, 4, true},
		{3, 6, true},
		{4, 1, true},
		{4, 3, true},
		{4, 9, true},
		{6, 0, true},
		{6, 1, true},
		{6, 7, true},
		{7, 0, false}, // Regions to the right of the interval
		{7, 3, false},
	}

	for _, tst := range tests {
		es := NewEditSet()
		es.Add(&Extent{3, 4}, "x")
		edit := &Extent{tst.offset, tst.length}
		err := es.Add(edit, "z")
		if tst.overlapExpected != (err != nil) {
			t.Fatalf("Overlapping edit %v undetected", edit)
		}
	}
}

func TestEditApply(t *testing.T) {
	input := "0123456789"

	es := NewEditSet()
	assertEquals(input, applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{0, 0}, "AAA")
	assertEquals("AAA0123456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{0, 2}, "AAA")
	assertEquals("AAA23456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{3, 2}, "")
	assertEquals("01256789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{8, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	err := es.Add(&Extent{-1, 3}, "")
	assertTrue(err != nil, t)
	//assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{12, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{2, 0}, "A")
	es.Add(&Extent{8, 1}, "B")
	es.Add(&Extent{4, 0}, "C")
	es.Add(&Extent{6, 2}, "D")
	assertEquals("01A23C45DB9", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(&Extent{0, 0}, "ABC")
	assertEquals("ABC", applyToString(es, ""), t)

	es = NewEditSet()
	es.Add(&Extent{0, 3}, "")
	assertEquals("", applyToString(es, "ABC"), t)

	es = NewEditSet()
	es.Add(&Extent{0, 0}, "")
	assertEquals("", applyToString(es, ""), t)
}
