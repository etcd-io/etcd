package gosec

import (
	"container/list"
	"sync"
)

// GlobalCache is a shared LRU cache for expensive operations.
// Each use case should define its own named key type to avoid collisions.
//
// Key type requirements:
//   - The key type must be comparable (no slices, maps, or funcs)
//   - Use type definitions (type MyKey struct{...}), not type aliases (type MyKey = ...)
//   - Avoid anonymous structs - they collide if the structure matches
var GlobalCache = NewLRUCache[any, any](1 << 16)

// LRUCache is a simple thread-safe generic LRU cache.
type LRUCache[K comparable, V any] struct {
	capacity  int
	items     map[K]*list.Element
	evictList *list.List
	lock      sync.Mutex
}

type entry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRUCache creates a new thread-safe LRU cache with the given capacity.
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	return &LRUCache[K, V]{
		capacity:  capacity,
		items:     make(map[K]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the cache. Returns the value and true if found,
// or the zero value and false if not found. Moves the entry to the front of the LRU list.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var zero V
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		return ent.Value.(*entry[K, V]).value, true
	}
	return zero, false
}

// Add inserts or updates a key-value pair in the cache.
// If the key exists, its value is updated and moved to the front.
// If the cache is full, the least recently used entry is evicted.
func (c *LRUCache[K, V]) Add(key K, value V) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		ent.Value.(*entry[K, V]).value = value
		return
	}

	ent := &entry[K, V]{key, value}
	element := c.evictList.PushFront(ent)
	c.items[key] = element

	if c.evictList.Len() > c.capacity {
		c.removeOldest()
	}
}

func (c *LRUCache[K, V]) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.evictList.Remove(ent)
		delete(c.items, ent.Value.(*entry[K, V]).key)
	}
}
