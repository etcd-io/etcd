package ttlcache

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Heap(t *testing.T) {
	var h tsHeap[string, int]

	i1 := &item[string, int]{expireAt: 1}
	i2 := &item[string, int]{expireAt: 2}
	i3 := &item[string, int]{expireAt: 3}

	heap.Push(&h, i2)
	heap.Push(&h, i3)
	heap.Push(&h, i1)

	assert.Equal(t, 3, h.Len())
	assert.Equal(t, i1, h[0])
	assert.NotEqual(t, 0, i2.expireIndex)
	assert.NotEqual(t, 0, i3.expireIndex)

	assert.Equal(t, i1, heap.Pop(&h))
	cp := h[:3]
	assert.Nil(t, cp[2]) // removed element should be nullified

	assert.Equal(t, 2, h.Len())
	assert.Equal(t, i2, h[0])
	assert.Equal(t, 1, i3.expireIndex)
}
