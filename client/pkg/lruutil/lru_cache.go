// Copyright 2022 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	ttl               time.Duration
}

var _ heap.Interface = &lruQueue{}

// NewTimeEvictLru create a new Lru, which evicts units after a time period of non use
// @ttl is time an entry exists before it is evicted
func NewTimeEvictLru(ttl time.Duration) *LruTimeCache {
	lru := LruTimeCache{
		lruCheckpointHeap: make(lruQueue, 0),
		bufMap:            make(map[string][]byte),
		ttl:               ttl,
	}
	heap.Init(&lru.lruCheckpointHeap)
	return &lru
}

func (lru *LruTimeCache) tick() {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	for {
		if len(lru.lruCheckpointHeap) < 1 {
			return
		}
		lt := lru.lruCheckpointHeap[0]
		if time.Now().After(lt.time) /* lt.time: next checkpoint time */ {
			item := heap.Pop(&lru.lruCheckpointHeap).(*lruItem)
			delete(lru.bufMap, item.id)
		} else {
			return
		}
	}
}
func (lru *LruTimeCache) Get(key string) (v []byte, ok bool) {
	lru.tick()
	lru.mu.Lock()
	defer lru.mu.Unlock()
	element := lru.bufMap[key]
	if element == nil {
		return nil, false
	}
	return element, true
}

func (lru *LruTimeCache) Set(key string, value []byte) {
	lru.tick()
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.bufMap[key] = value
	heap.Push(&lru.lruCheckpointHeap, &lruItem{
		id:    key,
		time:  time.Now().Add(lru.ttl),
		index: 0,
	})
}

func (lru *LruTimeCache) Len() int {
	lru.tick()
	lru.mu.Lock()
	defer lru.mu.Unlock()
	return lru.lruCheckpointHeap.Len()
}
