// Copyright 2016 The etcd Authors
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

// Package cache exports functionality for efficiently caching and mapping
// `RangeRequest`s to corresponding `RangeResponse`s.
package cache

import (
	"errors"
	"sync"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/pkg/v3/adt"
	"go.etcd.io/etcd/pkg/v3/ttlcache"
)

var (
	DefaultMaxEntries = 2048
	ErrCompacted      = rpctypes.ErrGRPCCompacted
	ErrNotExists      = errors.New("not exist")
)

type Cache interface {
	Add(req *pb.RangeRequest, resp *pb.RangeResponse)
	Get(req *pb.RangeRequest) (*pb.RangeResponse, error)
	Compact(revision int64)
	Invalidate(key []byte, endkey []byte)
	Size() int
	Close()
}

// keyFunc returns the key of a request, which is used to look up its caching response in the cache.
func keyFunc(req *pb.RangeRequest) string {
	// TODO: use marshalTo to reduce allocation
	b, err := req.Marshal()
	if err != nil {
		panic(err)
	}
	return string(b)
}

func NewCache(maxCacheEntries int, ttl time.Duration) Cache {
	return &cache{
		cache:        ttlcache.NewCache[string, *pb.RangeResponse](maxCacheEntries, ttl),
		cachedRanges: adt.NewIntervalTree(),
		compactedRev: -1,
	}
}

func NewNullCache() Cache {
	return &nullCache{}
}

func (c *cache) Close() {}

// cache implements Cache
type cache struct {
	mu    sync.RWMutex
	cache *ttlcache.Cache[string, *pb.RangeResponse]

	// a reverse index for cache invalidation
	cachedRanges adt.IntervalTree

	compactedRev int64
}

// Add adds the response of a request to the cache if its revision is larger than the compacted revision of the cache.
func (c *cache) Add(req *pb.RangeRequest, resp *pb.RangeResponse) {
	key := keyFunc(req)

	c.mu.Lock()
	defer c.mu.Unlock()

	if req.Revision > c.compactedRev {
		c.cache.Set(key, resp, time.Now())
	}
	// we do not need to invalidate a request with a revision specified.
	// so we do not need to add it into the reverse index.
	if req.Revision != 0 {
		return
	}

	var (
		iv  *adt.IntervalValue
		ivl adt.Interval
	)
	if len(req.RangeEnd) != 0 {
		ivl = adt.NewStringAffineInterval(string(req.Key), string(req.RangeEnd))
	} else {
		ivl = adt.NewStringAffinePoint(string(req.Key))
	}

	iv = c.cachedRanges.Find(ivl)

	if iv == nil {
		val := map[string]struct{}{key: {}}
		c.cachedRanges.Insert(ivl, val)
	} else {
		val := iv.Val.(map[string]struct{})
		val[key] = struct{}{}
		iv.Val = val
	}
}

// Get looks up the caching response for a given request.
// Get is also responsible for lazy eviction when accessing compacted entries.
func (c *cache) Get(req *pb.RangeRequest) (*pb.RangeResponse, error) {
	key := keyFunc(req)

	resp, err := c.get(req, key)

	if errors.Is(err, ErrCompacted) {
		c.mu.Lock()
		c.cache.Remove(key)
		c.mu.Unlock()
	}

	return resp, err
}

func (c *cache) get(req *pb.RangeRequest, key string) (*pb.RangeResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if req.Revision > 0 && req.Revision < c.compactedRev {
		return nil, ErrCompacted
	}

	if resp, ok := c.cache.Get(key, time.Now()); ok {
		return resp, nil
	}
	return nil, ErrNotExists
}

// Invalidate invalidates the cache entries that intersecting with the given range from key to endkey.
func (c *cache) Invalidate(key, endkey []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var (
		ivs []*adt.IntervalValue
		ivl adt.Interval
	)
	if len(endkey) == 0 {
		ivl = adt.NewStringAffinePoint(string(key))
	} else {
		ivl = adt.NewStringAffineInterval(string(key), string(endkey))
	}

	ivs = c.cachedRanges.Stab(ivl)
	for _, iv := range ivs {
		keys := iv.Val.(map[string]struct{})
		for key := range keys {
			c.cache.Remove(key)
		}
	}
	// delete after removing all keys since it is destructive to 'ivs'
	c.cachedRanges.Delete(ivl)
}

// Compact invalidate all caching response before the given rev.
// Replace with the invalidation is lazy. The actual removal happens when the entries is accessed.
func (c *cache) Compact(revision int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if revision > c.compactedRev {
		c.compactedRev = revision
	}
}

func (c *cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Size()
}

type nullCache struct {
}

func (n *nullCache) Add(req *pb.RangeRequest, resp *pb.RangeResponse)    {}
func (n *nullCache) Get(req *pb.RangeRequest) (*pb.RangeResponse, error) { return nil, ErrNotExists }
func (n *nullCache) Compact(revision int64)                              {}
func (n *nullCache) Invalidate(key []byte, endkey []byte)                {}
func (n *nullCache) Size() int                                           { return 0 }
func (n *nullCache) Close()                                              {}
