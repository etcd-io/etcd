package bbolt

import (
	"math/rand"
	"os"
	"reflect"
	"sort"
	"testing"
	"unsafe"
)

// TestFreelistType is used as a env variable for test to indicate the backend type
const TestFreelistType = "TEST_FREELIST_TYPE"

// Ensure that a page is added to a transaction's freelist.
func TestFreelist_free(t *testing.T) {
	f := newTestFreelist()
	f.free(100, &page{id: 12})
	if !reflect.DeepEqual([]pgid{12}, f.pending[100].ids) {
		t.Fatalf("exp=%v; got=%v", []pgid{12}, f.pending[100])
	}
}

// Ensure that a page and its overflow is added to a transaction's freelist.
func TestFreelist_free_overflow(t *testing.T) {
	f := newTestFreelist()
	f.free(100, &page{id: 12, overflow: 3})
	if exp := []pgid{12, 13, 14, 15}; !reflect.DeepEqual(exp, f.pending[100].ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.pending[100])
	}
}

// Ensure that a transaction's free pages can be released.
func TestFreelist_release(t *testing.T) {
	f := newTestFreelist()
	f.free(100, &page{id: 12, overflow: 1})
	f.free(100, &page{id: 9})
	f.free(102, &page{id: 39})
	f.release(100)
	f.release(101)
	if exp := []pgid{9, 12, 13}; !reflect.DeepEqual(exp, f.getFreePageIDs()) {
		t.Fatalf("exp=%v; got=%v", exp, f.getFreePageIDs())
	}

	f.release(102)
	if exp := []pgid{9, 12, 13, 39}; !reflect.DeepEqual(exp, f.getFreePageIDs()) {
		t.Fatalf("exp=%v; got=%v", exp, f.getFreePageIDs())
	}
}

// Ensure that releaseRange handles boundary conditions correctly
func TestFreelist_releaseRange(t *testing.T) {
	type testRange struct {
		begin, end txid
	}

	type testPage struct {
		id       pgid
		n        int
		allocTxn txid
		freeTxn  txid
	}

	var releaseRangeTests = []struct {
		title         string
		pagesIn       []testPage
		releaseRanges []testRange
		wantFree      []pgid
	}{
		{
			title:         "Single pending in range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{1, 300}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending with minimum end range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{1, 200}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending outsize minimum end range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{1, 199}},
			wantFree:      nil,
		},
		{
			title:         "Single pending with minimum begin range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{100, 300}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending outside minimum begin range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{101, 300}},
			wantFree:      nil,
		},
		{
			title:         "Single pending in minimum range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 199, freeTxn: 200}},
			releaseRanges: []testRange{{199, 200}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending and read transaction at 199",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 199, freeTxn: 200}},
			releaseRanges: []testRange{{100, 198}, {200, 300}},
			wantFree:      nil,
		},
		{
			title: "Adjacent pending and read transactions at 199, 200",
			pagesIn: []testPage{
				{id: 3, n: 1, allocTxn: 199, freeTxn: 200},
				{id: 4, n: 1, allocTxn: 200, freeTxn: 201},
			},
			releaseRanges: []testRange{
				{100, 198},
				{200, 199}, // Simulate the ranges db.freePages might produce.
				{201, 300},
			},
			wantFree: nil,
		},
		{
			title: "Out of order ranges",
			pagesIn: []testPage{
				{id: 3, n: 1, allocTxn: 199, freeTxn: 200},
				{id: 4, n: 1, allocTxn: 200, freeTxn: 201},
			},
			releaseRanges: []testRange{
				{201, 199},
				{201, 200},
				{200, 200},
			},
			wantFree: nil,
		},
		{
			title: "Multiple pending, read transaction at 150",
			pagesIn: []testPage{
				{id: 3, n: 1, allocTxn: 100, freeTxn: 200},
				{id: 4, n: 1, allocTxn: 100, freeTxn: 125},
				{id: 5, n: 1, allocTxn: 125, freeTxn: 150},
				{id: 6, n: 1, allocTxn: 125, freeTxn: 175},
				{id: 7, n: 2, allocTxn: 150, freeTxn: 175},
				{id: 9, n: 2, allocTxn: 175, freeTxn: 200},
			},
			releaseRanges: []testRange{{50, 149}, {151, 300}},
			wantFree:      []pgid{4, 9, 10},
		},
	}

	for _, c := range releaseRangeTests {
		f := newTestFreelist()
		var ids []pgid
		for _, p := range c.pagesIn {
			for i := uint64(0); i < uint64(p.n); i++ {
				ids = append(ids, pgid(uint64(p.id)+i))
			}
		}
		f.readIDs(ids)
		for _, p := range c.pagesIn {
			f.allocate(p.allocTxn, p.n)
		}

		for _, p := range c.pagesIn {
			f.free(p.freeTxn, &page{id: p.id, overflow: uint32(p.n - 1)})
		}

		for _, r := range c.releaseRanges {
			f.releaseRange(r.begin, r.end)
		}

		if exp := c.wantFree; !reflect.DeepEqual(exp, f.getFreePageIDs()) {
			t.Errorf("exp=%v; got=%v for %s", exp, f.getFreePageIDs(), c.title)
		}
	}
}

func TestFreelistHashmap_allocate(t *testing.T) {
	f := newTestFreelist()
	if f.freelistType != FreelistMapType {
		t.Skip()
	}

	ids := []pgid{3, 4, 5, 6, 7, 9, 12, 13, 18}
	f.readIDs(ids)

	f.allocate(1, 3)
	if x := f.free_count(); x != 6 {
		t.Fatalf("exp=5; got=%v", x)
	}

	f.allocate(1, 2)
	if x := f.free_count(); x != 4 {
		t.Fatalf("exp=3; got=%v", x)
	}
	f.allocate(1, 1)
	if x := f.free_count(); x != 3 {
		t.Fatalf("exp=2; got=%v", x)
	}

	f.allocate(1, 0)
	if x := f.free_count(); x != 3 {
		t.Fatalf("exp=2; got=%v", x)
	}
}

// Ensure that a freelist can find contiguous blocks of pages.
func TestFreelistArray_allocate(t *testing.T) {
	f := newTestFreelist()
	if f.freelistType != FreelistArrayType {
		t.Skip()
	}
	ids := []pgid{3, 4, 5, 6, 7, 9, 12, 13, 18}
	f.readIDs(ids)
	if id := int(f.allocate(1, 3)); id != 3 {
		t.Fatalf("exp=3; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 6 {
		t.Fatalf("exp=6; got=%v", id)
	}
	if id := int(f.allocate(1, 3)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if id := int(f.allocate(1, 2)); id != 12 {
		t.Fatalf("exp=12; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 7 {
		t.Fatalf("exp=7; got=%v", id)
	}
	if id := int(f.allocate(1, 0)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if id := int(f.allocate(1, 0)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if exp := []pgid{9, 18}; !reflect.DeepEqual(exp, f.getFreePageIDs()) {
		t.Fatalf("exp=%v; got=%v", exp, f.getFreePageIDs())
	}

	if id := int(f.allocate(1, 1)); id != 9 {
		t.Fatalf("exp=9; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 18 {
		t.Fatalf("exp=18; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if exp := []pgid{}; !reflect.DeepEqual(exp, f.getFreePageIDs()) {
		t.Fatalf("exp=%v; got=%v", exp, f.getFreePageIDs())
	}
}

// Ensure that a freelist can deserialize from a freelist page.
func TestFreelist_read(t *testing.T) {
	// Create a page.
	var buf [4096]byte
	page := (*page)(unsafe.Pointer(&buf[0]))
	page.flags = freelistPageFlag
	page.count = 2

	// Insert 2 page ids.
	ids := (*[3]pgid)(unsafe.Pointer(&page.ptr))
	ids[0] = 23
	ids[1] = 50

	// Deserialize page into a freelist.
	f := newTestFreelist()
	f.read(page)

	// Ensure that there are two page ids in the freelist.
	if exp := []pgid{23, 50}; !reflect.DeepEqual(exp, f.getFreePageIDs()) {
		t.Fatalf("exp=%v; got=%v", exp, f.getFreePageIDs())
	}
}

// Ensure that a freelist can serialize into a freelist page.
func TestFreelist_write(t *testing.T) {
	// Create a freelist and write it to a page.
	var buf [4096]byte
	f := newTestFreelist()

	f.readIDs([]pgid{12, 39})
	f.pending[100] = &txPending{ids: []pgid{28, 11}}
	f.pending[101] = &txPending{ids: []pgid{3}}
	p := (*page)(unsafe.Pointer(&buf[0]))
	if err := f.write(p); err != nil {
		t.Fatal(err)
	}

	// Read the page back out.
	f2 := newTestFreelist()
	f2.read(p)

	// Ensure that the freelist is correct.
	// All pages should be present and in reverse order.
	if exp := []pgid{3, 11, 12, 28, 39}; !reflect.DeepEqual(exp, f2.getFreePageIDs()) {
		t.Fatalf("exp=%v; got=%v", exp, f2.getFreePageIDs())
	}
}

func Benchmark_FreelistRelease10K(b *testing.B)    { benchmark_FreelistRelease(b, 10000) }
func Benchmark_FreelistRelease100K(b *testing.B)   { benchmark_FreelistRelease(b, 100000) }
func Benchmark_FreelistRelease1000K(b *testing.B)  { benchmark_FreelistRelease(b, 1000000) }
func Benchmark_FreelistRelease10000K(b *testing.B) { benchmark_FreelistRelease(b, 10000000) }

func benchmark_FreelistRelease(b *testing.B, size int) {
	ids := randomPgids(size)
	pending := randomPgids(len(ids) / 400)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txp := &txPending{ids: pending}
		f := newTestFreelist()
		f.pending = map[txid]*txPending{1: txp}
		f.readIDs(ids)
		f.release(1)
	}
}

func randomPgids(n int) []pgid {
	rand.Seed(42)
	pgids := make(pgids, n)
	for i := range pgids {
		pgids[i] = pgid(rand.Int63())
	}
	sort.Sort(pgids)
	return pgids
}

func Test_freelist_ReadIDs_and_getFreePageIDs(t *testing.T) {
	f := newTestFreelist()
	exp := []pgid{3, 4, 5, 6, 7, 9, 12, 13, 18}

	f.readIDs(exp)

	if got := f.getFreePageIDs(); !reflect.DeepEqual(exp, got) {
		t.Fatalf("exp=%v; got=%v", exp, got)
	}

	f2 := newTestFreelist()
	var exp2 []pgid
	f2.readIDs(exp2)

	if got2 := f2.getFreePageIDs(); !reflect.DeepEqual(got2, exp2) {
		t.Fatalf("exp2=%#v; got2=%#v", exp2, got2)
	}

}

func Test_freelist_mergeWithExist(t *testing.T) {
	bm1 := pidSet{1: struct{}{}}

	bm2 := pidSet{5: struct{}{}}
	tests := []struct {
		name            string
		ids             []pgid
		pgid            pgid
		want            []pgid
		wantForwardmap  map[pgid]uint64
		wantBackwardmap map[pgid]uint64
		wantfreemap     map[uint64]pidSet
	}{
		{
			name:            "test1",
			ids:             []pgid{1, 2, 4, 5, 6},
			pgid:            3,
			want:            []pgid{1, 2, 3, 4, 5, 6},
			wantForwardmap:  map[pgid]uint64{1: 6},
			wantBackwardmap: map[pgid]uint64{6: 6},
			wantfreemap:     map[uint64]pidSet{6: bm1},
		},
		{
			name:            "test2",
			ids:             []pgid{1, 2, 5, 6},
			pgid:            3,
			want:            []pgid{1, 2, 3, 5, 6},
			wantForwardmap:  map[pgid]uint64{1: 3, 5: 2},
			wantBackwardmap: map[pgid]uint64{6: 2, 3: 3},
			wantfreemap:     map[uint64]pidSet{3: bm1, 2: bm2},
		},
		{
			name:            "test3",
			ids:             []pgid{1, 2},
			pgid:            3,
			want:            []pgid{1, 2, 3},
			wantForwardmap:  map[pgid]uint64{1: 3},
			wantBackwardmap: map[pgid]uint64{3: 3},
			wantfreemap:     map[uint64]pidSet{3: bm1},
		},
		{
			name:            "test4",
			ids:             []pgid{2, 3},
			pgid:            1,
			want:            []pgid{1, 2, 3},
			wantForwardmap:  map[pgid]uint64{1: 3},
			wantBackwardmap: map[pgid]uint64{3: 3},
			wantfreemap:     map[uint64]pidSet{3: bm1},
		},
	}
	for _, tt := range tests {
		f := newTestFreelist()
		if f.freelistType == FreelistArrayType {
			t.Skip()
		}
		f.readIDs(tt.ids)

		f.mergeWithExistingSpan(tt.pgid)

		if got := f.getFreePageIDs(); !reflect.DeepEqual(tt.want, got) {
			t.Fatalf("name %s; exp=%v; got=%v", tt.name, tt.want, got)
		}
		if got := f.forwardMap; !reflect.DeepEqual(tt.wantForwardmap, got) {
			t.Fatalf("name %s; exp=%v; got=%v", tt.name, tt.wantForwardmap, got)
		}
		if got := f.backwardMap; !reflect.DeepEqual(tt.wantBackwardmap, got) {
			t.Fatalf("name %s; exp=%v; got=%v", tt.name, tt.wantBackwardmap, got)
		}
		if got := f.freemaps; !reflect.DeepEqual(tt.wantfreemap, got) {
			t.Fatalf("name %s; exp=%v; got=%v", tt.name, tt.wantfreemap, got)
		}
	}
}

// newTestFreelist get the freelist type from env and initial the freelist
func newTestFreelist() *freelist {
	freelistType := FreelistArrayType
	if env := os.Getenv(TestFreelistType); env == string(FreelistMapType) {
		freelistType = FreelistMapType
	}

	return newFreelist(freelistType)
}
