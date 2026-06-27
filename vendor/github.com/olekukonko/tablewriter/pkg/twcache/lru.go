package twcache

import (
	"sync"
	"sync/atomic"
)

// EvictCallback is a function called when an entry is evicted.
// This includes evictions during Purge or Resize operations.
type EvictCallback[K comparable, V any] func(key K, value V)

// LRU is a thread-safe, generic LRU cache with a fixed size.
// It has zero dependencies, high performance, and full features.
type LRU[K comparable, V any] struct {
	size    int
	items   map[K]*entry[K, V]
	head    *entry[K, V] // Most Recently Used
	tail    *entry[K, V] // Least Recently Used
	onEvict EvictCallback[K, V]

	mu     sync.Mutex
	hits   atomic.Int64
	misses atomic.Int64
}

// entry represents a single item in the LRU linked list.
// It holds the key, value, and pointers to prev/next entries.
type entry[K comparable, V any] struct {
	key   K
	value V
	prev  *entry[K, V]
	next  *entry[K, V]
}

// NewLRU creates a new LRU cache with the given size.
// Returns nil if size <= 0, acting as a disabled cache.
// Caps size at 100,000 for reasonableness.
func NewLRU[K comparable, V any](size int) *LRU[K, V] {
	return NewLRUEvict[K, V](size, nil)
}

// NewLRUEvict creates a new LRU cache with an eviction callback.
// The callback is optional and called on evictions.
// Returns nil if size <= 0.
func NewLRUEvict[K comparable, V any](size int, onEvict EvictCallback[K, V]) *LRU[K, V] {
	if size <= 0 {
		return nil // nil = disabled cache (fast path in hot code)
	}
	if size > 100_000 {
		size = 100_000 // reasonable upper bound
	}
	return &LRU[K, V]{
		size:    size,
		items:   make(map[K]*entry[K, V], size),
		onEvict: onEvict,
	}
}

// GetOrCompute retrieves a value or computes it if missing.
// Ensures no double computation under concurrency.
// Ideal for expensive computations like twwidth.
func (c *LRU[K, V]) GetOrCompute(key K, compute func() V) V {
	if c == nil || c.size <= 0 {
		return compute()
	}

	c.mu.Lock()
	if e, ok := c.items[key]; ok {
		c.moveToFront(e)
		c.hits.Add(1)
		c.mu.Unlock()
		return e.value
	}

	c.misses.Add(1)
	value := compute() // expensive work only on real miss

	// Double-check: someone might have added it while computing
	if e, ok := c.items[key]; ok {
		e.value = value
		c.moveToFront(e)
		c.mu.Unlock()
		return value
	}

	// Evict if needed
	if len(c.items) >= c.size {
		c.removeOldest()
	}

	e := &entry[K, V]{key: key, value: value}
	c.addToFront(e)
	c.items[key] = e
	c.mu.Unlock()
	return value
}

// Get retrieves a value by key if it exists.
// Returns the value and true if found, else zero and false.
// Updates the entry to most recently used.
func (c *LRU[K, V]) Get(key K) (V, bool) {
	if c == nil || c.size <= 0 {
		var zero V
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		var zero V
		return zero, false
	}
	c.hits.Add(1)
	c.moveToFront(e)
	return e.value, true
}

// Add inserts or updates a key-value pair.
// Evicts the oldest if cache is full.
// Returns true if an eviction occurred.
func (c *LRU[K, V]) Add(key K, value V) (evicted bool) {
	if c == nil || c.size <= 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.items[key]; ok {
		e.value = value
		c.moveToFront(e)
		return false
	}

	if len(c.items) >= c.size {
		c.removeOldest()
		evicted = true
	}

	e := &entry[K, V]{key: key, value: value}
	c.addToFront(e)
	c.items[key] = e
	return evicted
}

// Remove deletes a key from the cache.
// Returns true if the key was found and removed.
func (c *LRU[K, V]) Remove(key K) bool {
	if c == nil || c.size <= 0 {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeNode(e)
	delete(c.items, key)
	return true
}

// Purge clears all entries from the cache.
// Calls onEvict for each entry if set.
// Resets hit/miss counters.
func (c *LRU[K, V]) Purge() {
	if c == nil || c.size <= 0 {
		return
	}
	c.mu.Lock()
	if c.onEvict != nil {
		for key, e := range c.items {
			c.onEvict(key, e.value)
		}
	}
	c.items = make(map[K]*entry[K, V], c.size)
	c.head = nil
	c.tail = nil
	c.hits.Store(0)
	c.misses.Store(0)
	c.mu.Unlock()
}

// Len returns the current number of items in the cache.
func (c *LRU[K, V]) Len() int {
	if c == nil || c.size <= 0 {
		return 0
	}
	c.mu.Lock()
	n := len(c.items)
	c.mu.Unlock()
	return n
}

// Cap returns the maximum capacity of the cache.
func (c *LRU[K, V]) Cap() int {
	if c == nil {
		return 0
	}
	return c.size
}

// HitRate returns the cache hit ratio (0.0 to 1.0).
// Based on hits / (hits + misses).
func (c *LRU[K, V]) HitRate() float64 {
	h := c.hits.Load()
	m := c.misses.Load()
	total := h + m
	if total == 0 {
		return 0.0
	}
	return float64(h) / float64(total)
}

// RemoveOldest removes and returns the least recently used item.
// Returns key, value, and true if an item was removed.
// Calls onEvict if set.
func (c *LRU[K, V]) RemoveOldest() (key K, value V, ok bool) {
	if c == nil || c.size <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tail == nil {
		return
	}

	key = c.tail.key
	value = c.tail.value

	c.removeOldest()
	return key, value, true
}

// moveToFront moves an entry to the front (MRU position).
func (c *LRU[K, V]) moveToFront(e *entry[K, V]) {
	if c.head == e {
		return
	}
	c.removeNode(e)
	c.addToFront(e)
}

// addToFront adds an entry to the front of the list.
func (c *LRU[K, V]) addToFront(e *entry[K, V]) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}

}

// removeNode removes an entry from the linked list.
func (c *LRU[K, V]) removeNode(e *entry[K, V]) {
	if e.prev != nil {
		e.prev.next = e.next
	} else {
		c.head = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	} else {
		c.tail = e.prev
	}
	e.prev = nil
	e.next = nil
}

// removeOldest removes the tail entry (LRU).
// Calls onEvict if set and deletes from map.
func (c *LRU[K, V]) removeOldest() {
	if c.tail == nil {
		return
	}
	e := c.tail
	if c.onEvict != nil {
		c.onEvict(e.key, e.value)
	}
	c.removeNode(e)
	delete(c.items, e.key)
}
