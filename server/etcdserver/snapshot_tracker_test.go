package etcdserver

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapTracker_MinSnapi(t *testing.T) {
	st := SnapshotTracker{}

	_, err := st.MinSnapi()
	assert.NotNil(t, err, "SnapshotTracker should be empty initially")

	st.Track(10)
	minSnapi, err := st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(10), minSnapi, "MinSnapi should return the only tracked snapshot index")

	st.Track(5)
	minSnapi, err = st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(5), minSnapi, "MinSnapi should return the minimum tracked snapshot index")

	st.UnTrack(5)
	minSnapi, err = st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(10), minSnapi, "MinSnapi should return the remaining tracked snapshot index")
}

func TestSnapTracker_Track(t *testing.T) {
	st := SnapshotTracker{}
	st.Track(20)
	st.Track(10)
	st.Track(15)

	assert.Equal(t, 3, st.h.Len(), "SnapshotTracker should have 3 snapshots tracked")

	minSnapi, err := st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(10), minSnapi, "MinSnapi should return the minimum tracked snapshot index")
}

func TestSnapTracker_UnTrack(t *testing.T) {
	st := SnapshotTracker{}
	st.Track(20)
	st.Track(30)
	st.Track(40)
	// track another snapshot with the same index
	st.Track(20)

	st.UnTrack(30)
	assert.Equal(t, 3, st.h.Len())

	minSnapi, err := st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(20), minSnapi)

	st.UnTrack(20)
	assert.Equal(t, 2, st.h.Len())

	minSnapi, err = st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(20), minSnapi)

	st.UnTrack(20)
	minSnapi, err = st.MinSnapi()
	assert.Nil(t, err)
	assert.Equal(t, uint64(40), minSnapi)

	st.UnTrack(40)
	_, err = st.MinSnapi()
	assert.NotNil(t, err)
}

func newMinHeap(elements ...uint64) minHeap[uint64] {
	h := minHeap[uint64](elements)
	heap.Init(&h)
	return h
}

func TestMinHeapLen(t *testing.T) {
	h := newMinHeap(3, 2, 1)
	assert.Equal(t, 3, h.Len())
}

func TestMinHeapLess(t *testing.T) {
	h := newMinHeap(3, 2, 1)
	assert.True(t, h.Less(0, 1))
}

func TestMinHeapSwap(t *testing.T) {
	h := newMinHeap(3, 2, 1)
	h.Swap(0, 1)
	assert.Equal(t, uint64(2), h[0])
	assert.Equal(t, uint64(1), h[1])
	assert.Equal(t, uint64(3), h[2])
}

func TestMinHeapPushPop(t *testing.T) {
	h := newMinHeap(3, 2)
	heap.Push(&h, uint64(1))
	assert.Equal(t, 3, h.Len())

	got := heap.Pop(&h).(uint64)
	assert.Equal(t, uint64(1), got)
}

func TestMinHeapEmpty(t *testing.T) {
	h := minHeap[uint64]{}
	assert.Equal(t, 0, h.Len())
}

func TestMinHeapSingleElement(t *testing.T) {
	h := newMinHeap(uint64(1))
	assert.Equal(t, 1, h.Len())

	heap.Push(&h, uint64(2))
	assert.Equal(t, 2, h.Len())

	got := heap.Pop(&h)
	assert.Equal(t, uint64(1), got)
}
