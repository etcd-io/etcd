package cache

import "sync"

type threadSafeMap struct {
	lock  sync.RWMutex
	items map[string]interface{}
}

func NewStore() *threadSafeMap {
	return &threadSafeMap{}
}

func (c *threadSafeMap) Add(key string, obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items[key] = obj
}

func (c *threadSafeMap) Update(key string, obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items[key] = obj
}

func (c *threadSafeMap) Delete(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, exists := c.items[key]; exists {
		delete(c.items, key)
	}
}

func (c *threadSafeMap) Get(key string) (item interface{}, exists bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	item, exists = c.items[key]
	return item, exists
}

func (c *threadSafeMap) Replace(items map[string]interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items = items
}

func (c *threadSafeMap) InvalidateAll() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.items = map[string]interface{}{}
}
