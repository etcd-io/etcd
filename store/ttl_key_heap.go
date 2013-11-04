package store

import (
	"container/heap"
)

// An TTLKeyHeap is a min-heap of TTLKeys order by expiration time
type TTLKeyHeap struct {
	Array []*Node
	Map   map[*Node]int
}

func (h TTLKeyHeap) Len() int {
	return len(h.Array)
}

func (h TTLKeyHeap) Less(i, j int) bool {
	return h.Array[i].ExpireTime.Before(h.Array[j].ExpireTime)
}

func (h TTLKeyHeap) Swap(i, j int) {
	// swap node
	h.Array[i], h.Array[j] = h.Array[j], h.Array[i]

	// update map
	h.Map[h.Array[i]] = i
	h.Map[h.Array[j]] = j
}

func (h *TTLKeyHeap) Push(x interface{}) {
	n, _ := x.(*Node)
	h.Map[n] = len(h.Array)
	h.Array = append(h.Array, n)
}

func (h *TTLKeyHeap) Pop() interface{} {
	old := h.Array
	n := len(old)
	x := old[n-1]
	h.Array = old[0 : n-1]
	delete(h.Map, x)
	return x
}

func (h *TTLKeyHeap) Update(n *Node) {
	index := h.Map[n]
	heap.Remove(h, index)
	heap.Push(h, n)
}
