package store

import (
	"container/heap"
	"fmt"
	"testing"
	"time"
)

func TestHeapPushPop(t *testing.T) {
	h := newTTLKeyHeap()

	// add from older expire time to earlier expire time
	// the path is equal to ttl from now
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf("%v", 10-i)
		m := time.Duration(10 - i)
		n := newKV(nil, path, path, 0, 0, nil, "", time.Now().Add(time.Second*m))
		heap.Push(h, n)
	}

	min := time.Now()

	for i := 0; i < 10; i++ {
		iNode := heap.Pop(h)
		node, _ := iNode.(*Node)
		if node.ExpireTime.Before(min) {
			t.Fatal("heap sort wrong!")
		}
		min = node.ExpireTime
	}

}

func TestHeapUpdate(t *testing.T) {
	h := &TTLKeyHeap{Map: make(map[*Node]int)}
	heap.Init(h)

	kvs := make([]*Node, 10)

	// add from older expire time to earlier expire time
	// the path is equal to ttl from now
	for i, n := range kvs {
		path := fmt.Sprintf("%v", 10-i)
		m := time.Duration(10 - i)
		n = newKV(nil, path, path, 0, 0, nil, "", time.Now().Add(time.Second*m))
		kvs[i] = n
		heap.Push(h, n)
	}

	// Path 7
	kvs[3].ExpireTime = time.Now().Add(time.Second * 11)

	// Path 5
	kvs[5].ExpireTime = time.Now().Add(time.Second * 12)

	h.update(kvs[3])
	h.update(kvs[5])

	min := time.Now()

	for i := 0; i < 10; i++ {
		iNode := heap.Pop(h)
		node, _ := iNode.(*Node)
		if node.ExpireTime.Before(min) {
			t.Fatal("heap sort wrong!")
		}
		min = node.ExpireTime

		if i == 8 {
			if node.Path != "7" {
				t.Fatal("heap sort wrong!", node.Path)
			}
		}

		if i == 9 {
			if node.Path != "5" {
				t.Fatal("heap sort wrong!")
			}
		}

	}

}
