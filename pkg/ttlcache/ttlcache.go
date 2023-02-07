// Copyright 2023 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ttlcache

import (
	"container/heap"
	"container/list"
	"time"
)

// Cache supports LRU + TTL items eviction. Structure is concurrently unsafe, Get and Size calls should be protected
// at least by shared (read-write) lock, other calls should be protected by exclusive (write) lock.
type Cache[K comparable, V any] struct {
	capacity int
	ttl      time.Duration

	kv map[K]*item[K, V]

	lru      *list.List
	keyToLRU map[K]*list.Element
	expQueue tsHeap[K, V] // expiration queue, any interaction should be done via heap.*
}

type item[K comparable, V any] struct {
	expireAt    int64
	expireIndex int // index of the element in Cache.expQueue

	key   K
	value V
}

func NewCache[K comparable, V any](capacity int, ttl time.Duration) *Cache[K, V] {
	return &Cache[K, V]{
		capacity: capacity,
		ttl:      ttl,

		kv: make(map[K]*item[K, V], capacity),

		lru:      list.New(),
		keyToLRU: make(map[K]*list.Element, capacity),
		expQueue: make(tsHeap[K, V], 0, capacity),
	}
}

func (c *Cache[K, V]) Set(k K, v V, now time.Time) {
	c.removeExpired(now)

	if _, ok := c.kv[k]; !ok && len(c.kv) == c.capacity {
		c.Remove(c.lru.Back().Value.(K))
	}

	expireAt := now.Add(c.ttl).Unix()

	it, ok := c.kv[k]
	if !ok {
		it = &item[K, V]{
			value: v,
			key:   k,

			expireAt: expireAt,
		}

		c.kv[k] = it
		heap.Push(&c.expQueue, it)
		c.keyToLRU[k] = c.lru.PushFront(k)

		return
	}

	it.value = v
	it.expireAt = now.Add(c.ttl).Unix()
	c.lru.MoveToFront(c.keyToLRU[k])
	heap.Fix(&c.expQueue, it.expireIndex)
}

func (c *Cache[K, V]) Get(k K, now time.Time) (V, bool) {
	// It's impossible to mark V as "nullable", and therefore impossible to `return nil, false`.
	// At the same time, if V is nullable type, `empty` will be nil.
	var empty V

	v, ok := c.kv[k]
	if !ok {
		return empty, false
	}

	if v.expireAt < now.Unix() {
		return empty, false
	}

	c.lru.MoveToFront(c.keyToLRU[k])

	return v.value, true
}

func (c *Cache[K, V]) Size() int {
	return len(c.kv)
}

func (c *Cache[K, V]) Remove(k K) {
	it, ok := c.kv[k]
	if !ok {
		return
	}

	c.lru.Remove(c.keyToLRU[k])
	heap.Remove(&c.expQueue, it.expireIndex)
	delete(c.kv, k)
	delete(c.keyToLRU, k)
}

func (c *Cache[K, V]) removeExpired(now time.Time) {
	ut := now.Unix()

	for c.expQueue.Len() > 0 && c.expQueue[0].expireAt < ut {
		c.Remove(c.expQueue[0].key)
	}
}
