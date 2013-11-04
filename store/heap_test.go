package store

import (
	"container/heap"
	"fmt"
	"testing"
	"time"
)

func TestHeapPushPop(t *testing.T) {
	h := &TTLKeyHeap{Map: make(map[*Node]int)}
	heap.Init(h)

	kvs := make([]*Node, 10)

	// add from older expire time to earlier expire time
	// the path is equal to ttl from now
	for i, n := range kvs {
		path := fmt.Sprintf("%v", 10-i)
		m := time.Duration(10 - i)
		n = newKV(nil, path, path, 0, 0, nil, "", time.Now().Add(time.Second*m))
		heap.Push(h, n)
	}

	min := time.Now()

	for i := 0; i < 9; i++ {
		iNode := heap.Pop(h)
		node, _ := iNode.(*Node)
		if node.ExpireTime.Before(min) {
			t.Fatal("heap sort wrong!")
		}
		min = node.ExpireTime
	}

}
