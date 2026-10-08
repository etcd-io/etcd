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
	"math"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"k8s.io/utils/lru"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/pkg/v3/adt"
)

var (
	DefaultMaxEntries = 2048
	ErrCompacted      = rpctypes.ErrGRPCCompacted
)

// cacheEntry wraps a cached response with its expiry time. Entries expire
// after a configurable TTL to prevent stale reads when auth tokens expire
// on the backend but the proxy cache still holds the response.
type cacheEntry struct {
	resp      *pb.RangeResponse
	expiresAt time.Time
}

type Cache interface {
	// Add stores resp for (req, authKey). authKey scopes the entry to a
	// single caller identity so a cached response can never be replayed to
	// a different principal; pass "" when the request carries no credentials.
	// The entry expires after the TTL configured on the cache.
	Add(req *pb.RangeRequest, resp *pb.RangeResponse, authKey string)
	// Get returns the cached response for (req, authKey). Expired entries
	// are treated as misses.
	Get(req *pb.RangeRequest, authKey string) (*pb.RangeResponse, error)
	// Clear drops every cached entry. Used when the authorization
	// configuration may have changed (user/role/permission updates) so no
	// principal keeps reading data cached under stale permissions.
	Clear()
	Compact(revision int64)
	Invalidate(key []byte, endkey []byte)
	Size() int
	Close()
}

// keyFunc returns the key of a request, which is used to look up its caching response in the cache.
// The caller identity (authKey) is mixed in so that a response cached for
// one authenticated user is never served to a different user.
func keyFunc(req *pb.RangeRequest, authKey string) string {
	// TODO: use marshalTo to reduce allocation
	b, err := proto.Marshal(req)
	if err != nil {
		panic(err)
	}
	// Pre-size the buffer to avoid repeated allocation while appending.
	// proto.Marshal output is bounded by MaxRequestBytes (typically
	// 1.5 MiB) and authKey is a bounded user identifier, so the sum
	// cannot overflow on any supported platform. The explicit guard
	// documents that invariant and silences CodeQL's allocation-size
	// overflow heuristic.
	if len(b) > math.MaxInt-len(authKey)-1 {
		panic("request too large")
	}
	out := make([]byte, 0, len(b)+1+len(authKey))
	out = append(out, b...)
	out = append(out, 0)
	out = append(out, authKey...)
	return string(out)
}

// NewCache creates a cache with no entry TTL (entries never expire based on
// time). Use NewCacheWithTTL to set a maximum entry lifetime.
func NewCache(maxCacheEntries int) Cache {
	return NewCacheWithTTL(maxCacheEntries, 0)
}

// NewCacheWithTTL creates a cache where entries expire after ttl. A ttl of 0
// means entries never expire based on time (only via LRU eviction, Clear, or
// Invalidate). The TTL should be set to match or slightly less than the
// backend auth token TTL to prevent serving stale reads after token expiry.
func NewCacheWithTTL(maxCacheEntries int, ttl time.Duration) Cache {
	return &cache{
		lru:          lru.New(maxCacheEntries),
		maxEntries:   maxCacheEntries,
		cachedRanges: adt.NewIntervalTree(),
		compactedRev: -1,
		entryTTL:     ttl,
	}
}

func (c *cache) Close() {}

// cache implements Cache
type cache struct {
	mu         sync.RWMutex
	lru        *lru.Cache
	maxEntries int

	// a reverse index for cache invalidation
	cachedRanges adt.IntervalTree

	compactedRev int64

	// entryTTL is the maximum lifetime of a cache entry. 0 means no
	// time-based expiry.
	entryTTL time.Duration
}

// Add adds the response of a request to the cache if its revision is larger than the compacted revision of the cache.
func (c *cache) Add(req *pb.RangeRequest, resp *pb.RangeResponse, authKey string) {
	key := keyFunc(req, authKey)

	c.mu.Lock()
	defer c.mu.Unlock()

	if req.Revision > c.compactedRev {
		entry := &cacheEntry{resp: resp}
		if c.entryTTL > 0 {
			entry.expiresAt = time.Now().Add(c.entryTTL)
		}
		c.lru.Add(key, entry)
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
// Get is also responsible for lazy eviction when accessing compacted entries
// and for dropping expired entries when a TTL is configured.
func (c *cache) Get(req *pb.RangeRequest, authKey string) (*pb.RangeResponse, error) {
	key := keyFunc(req, authKey)

	c.mu.Lock()
	defer c.mu.Unlock()

	if req.Revision > 0 && req.Revision < c.compactedRev {
		c.lru.Remove(key)
		return nil, ErrCompacted
	}

	if item, ok := c.lru.Get(key); ok {
		entry := item.(*cacheEntry)
		// Check if the entry has expired (when TTL is configured)
		if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
			c.lru.Remove(key)
			return nil, errors.New("entry expired")
		}
		return entry.resp, nil
	}
	return nil, errors.New("not exist")
}

// Clear drops all cached entries and resets the reverse invalidation index.
// It is called when the authorization configuration may have changed so no
// principal can keep reading data cached under stale permissions.
func (c *cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lru = lru.New(c.maxEntries)
	c.cachedRanges = adt.NewIntervalTree()
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
			c.lru.Remove(key)
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
	return c.lru.Len()
}
