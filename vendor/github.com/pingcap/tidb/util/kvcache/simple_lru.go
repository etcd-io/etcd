// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package kvcache

import (
	"container/list"
)

// Key is the interface that every key in LRU Cache should implement.
type Key interface {
	Hash() []byte
}

// Value is the interface that every value in LRU Cache should implement.
type Value interface {
}

// cacheEntry wraps Key and Value. It's the value of list.Element.
type cacheEntry struct {
	key   Key
	value Value
}

// SimpleLRUCache is a simple least recently used cache, not thread-safe, use it carefully.
type SimpleLRUCache struct {
	capacity uint
	size     uint
	elements map[string]*list.Element
	cache    *list.List
}

// NewSimpleLRUCache creates a SimpleLRUCache object, whose capacity is "capacity".
// NOTE: "capacity" should be a positive value.
func NewSimpleLRUCache(capacity uint) *SimpleLRUCache {
	if capacity <= 0 {
		panic("capacity of LRU Cache should be positive.")
	}
	return &SimpleLRUCache{
		capacity: capacity,
		size:     0,
		elements: make(map[string]*list.Element),
		cache:    list.New(),
	}
}

// Get tries to find the corresponding value according to the given key.
func (l *SimpleLRUCache) Get(key Key) (value Value, ok bool) {
	element, exists := l.elements[string(key.Hash())]
	if !exists {
		return nil, false
	}
	l.cache.MoveToFront(element)
	return element.Value.(*cacheEntry).value, true
}

// Put puts the (key, value) pair into the LRU Cache.
func (l *SimpleLRUCache) Put(key Key, value Value) {
	hash := string(key.Hash())
	element, exists := l.elements[hash]
	if exists {
		l.cache.MoveToFront(element)
		return
	}

	newCacheEntry := &cacheEntry{
		key:   key,
		value: value,
	}

	element = l.cache.PushFront(newCacheEntry)
	l.elements[hash] = element
	l.size++

	for l.size > l.capacity {
		lru := l.cache.Back()
		l.cache.Remove(lru)
		delete(l.elements, string(lru.Value.(*cacheEntry).key.Hash()))
		l.size--
	}
}
