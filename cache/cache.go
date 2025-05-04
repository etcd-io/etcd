package cache

import (
	"sync"
)

// Object represents a cacheable etcd object.
type Object struct {
	Key   string
	Value []byte
}

// Cache is a simple thread-safe in-memory cache.
type Cache struct {
	mu    sync.RWMutex
	store map[string]Object
}

func NewCache() *Cache {
	return &Cache{
		store: make(map[string]Object),
	}
}

func (c *Cache) Set(obj Object) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[obj.Key] = obj
}

func (c *Cache) Get(key string) (Object, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	obj, ok := c.store[key]
	return obj, ok
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}
