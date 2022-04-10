package lruutil

import (
	"container/heap"
	"sync"
	"time"
)

type lruItem struct {
	id    string
	time  time.Time
	index int
}
type lruQueue []*lruItem

func (l lruQueue) Len() int {
	return len(l)
}

func (l lruQueue) Less(i, j int) bool {
	return l[i].time.Before(l[j].time)
}

func (l lruQueue) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
	l[i].index = i
	l[j].index = j
}

func (l *lruQueue) Push(x interface{}) {
	n := len(*l)
	item := x.(*lruItem)
	item.index = n
	*l = append(*l, item)
}

func (l *lruQueue) Pop() interface{} {
	old := *l
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*l = old[0 : n-1]
	return item
}

type LruTimeCache struct {
	mu                sync.Mutex
	lruCheckpointHeap lruQueue
	bufMap            map[string][]byte
	done              chan int
}

var _ heap.Interface = &lruQueue{}

// NewTimeEvictLru create a new Lru, which evicts units after a time period of non use
// @ttl is time an entry exists before it is evicted
func NewTimeEvictLru(ttl time.Duration) *LruTimeCache {
	lru := LruTimeCache{
		lruCheckpointHeap: make(lruQueue, 0),
		bufMap:            make(map[string][]byte),
		done:              make(chan int),
	}
	heap.Init(&lru.lruCheckpointHeap)

	timer := time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-timer.C:
				lru.tick()
			case <-lru.done:
				return

			}
		}
	}()
	return &lru
}

func (lru *LruTimeCache) tick() {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	if len(lru.lruCheckpointHeap) < 1 {
		return
	}
	lt := lru.lruCheckpointHeap[0]
	if lt.time.After(time.Now()) /* lt.time: next checkpoint time */ {
		return
	}
	item := heap.Pop(&lru.lruCheckpointHeap).(*lruItem)
	delete(lru.bufMap, item.id)

}
func (lru *LruTimeCache) Get(key string) (v []byte, ok bool) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	element := lru.bufMap[key]
	if element == nil {
		return nil, false
	}
	return element, true
}

func (lru *LruTimeCache) Set(key string, value []byte) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.bufMap[key] = value
	heap.Push(&lru.lruCheckpointHeap, &lruItem{
		id:    key,
		time:  time.Now(),
		index: 0,
	})

}

func (lru *LruTimeCache) Len() int {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	return lru.lruCheckpointHeap.Len()
}
func (lru *LruTimeCache) Close() {
	close(lru.done)
}
