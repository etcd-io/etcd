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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type CacheObj struct {
	key string
	val int
}

func Test_SetGet(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}

	c := NewCache[string, *CacheObj](2, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now)
	c.Set(obj2.key, obj2, now)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
}

func Test_Capacity(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}
	obj3 := &CacheObj{"k3", 3}

	c := NewCache[string, *CacheObj](2, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now)
	c.Set(obj2.key, obj2, now)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
	assertNotInCache(t, c, obj3, now)

	c.Set(obj3.key, obj3, now)

	assert.Equal(t, 2, c.Size())

	assertNotInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)
}

func Test_LRU(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}
	obj3 := &CacheObj{"k3", 3}

	c := NewCache[string, *CacheObj](2, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now)
	c.Set(obj2.key, obj2, now)

	assertInCache(t, c, obj2, now)
	assertInCache(t, c, obj1, now) // obj1 was touched after obj2
	assertNotInCache(t, c, obj3, now)

	c.Set(obj3.key, obj3, now)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, now)
	assertNotInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)
}

func Test_TTL_Read(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}
	obj3 := &CacheObj{"k3", 3}

	c := NewCache[string, *CacheObj](3, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now)
	c.Set(obj2.key, obj2, now)
	c.Set(obj3.key, obj3, now)

	assert.Equal(t, 3, c.Size())

	now = now.Add(time.Hour)

	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)

	expired := now.Add(time.Second)

	// Element is not returned, but preserved in cache
	assertNotInCache(t, c, obj1, expired)
	assertNotInCache(t, c, obj2, expired)
	assertNotInCache(t, c, obj3, expired)

	// Call from future to the past - element is still there
	assert.Equal(t, 3, c.Size())

	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)
}

func Test_TTL_LRU(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}
	obj3 := &CacheObj{"k3", 3}
	obj4 := &CacheObj{"k4", 3}
	obj5 := &CacheObj{"k5", 3}

	c := NewCache[string, *CacheObj](3, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now.Add(time.Hour))
	c.Set(obj2.key, obj2, now)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)

	expired := now.Add(time.Hour).Add(time.Second)

	// we still have capacity, but obj2 will be evicted by ttl
	c.Set(obj3.key, obj3, expired)

	assert.Equal(t, 2, c.Size())
	assertInCache(t, c, obj1, now)
	assertNotInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)

	// touch obj1, change lru from obj3 -> obj1 ot obj1 -> obj3
	assertInCache(t, c, obj1, now)

	// discard obj3 by capacity limit
	c.Set(obj4.key, obj4, now)
	c.Set(obj5.key, obj5, now)

	assert.Equal(t, 3, c.Size())

	assertInCache(t, c, obj1, now)
	assertNotInCache(t, c, obj2, now)
	assertNotInCache(t, c, obj3, now)
	assertInCache(t, c, obj4, now)
	assertInCache(t, c, obj5, now)
}

func Test_Update(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}
	obj3 := &CacheObj{"k3", 3}

	c := NewCache[string, *CacheObj](2, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now)
	c.Set(obj2.key, obj2, now)

	c.Set(obj1.key, obj1, now.Add(time.Hour)) // obj1 touched after obj1 by updating the value
	c.Set(obj3.key, obj3, now)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, now)
	assertNotInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)

	expired1 := now.Add(time.Hour).Add(time.Second)
	expired2 := now.Add(2 * time.Hour).Add(time.Second)

	assert.Equal(t, 2, c.Size())

	// Expiration ts should be updated too
	assertInCache(t, c, obj1, expired1)
	assertNotInCache(t, c, obj3, expired1)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, expired1)
	assertNotInCache(t, c, obj1, expired2)

	assert.Equal(t, 2, c.Size())
}

func Test_Remove(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	obj1 := &CacheObj{"k1", 1}
	obj2 := &CacheObj{"k2", 2}
	obj3 := &CacheObj{"k3", 3}

	c := NewCache[string, *CacheObj](3, time.Hour)

	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)
	c.Set(obj1.key, obj1, now)
	c.Set(obj2.key, obj2, now)
	c.Set(obj3.key, obj3, now)

	assert.Equal(t, 3, c.Size())
	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)

	c.Remove("not-exists")

	assert.Equal(t, 3, c.Size())
	assertInCache(t, c, obj1, now)
	assertInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)

	c.Remove(obj2.key)

	assert.Equal(t, 2, c.Size())

	assertInCache(t, c, obj1, now)
	assertNotInCache(t, c, obj2, now)
	assertInCache(t, c, obj3, now)
}

func assertInCache(t assert.TestingT, c *Cache[string, *CacheObj], obj *CacheObj, now time.Time) {
	v, ok := c.Get(obj.key, now)
	assert.True(t, ok)
	assert.Equal(t, obj, v)
}

func assertNotInCache(t assert.TestingT, c *Cache[string, *CacheObj], obj *CacheObj, now time.Time) {
	v, ok := c.Get(obj.key, now)
	assert.False(t, ok)
	assert.Nil(t, v)
}
