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

package cache

import (
	"hash"
	"hash/adler32"
	"sync"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/davecgh/go-spew/spew"
	"github.com/golang/groupcache/lru"
)

const DefaultMaxEntries = 2048

type Cache interface {
	Add(req *pb.RangeRequest, resp *pb.RangeResponse)
	Get(req *pb.RangeRequest) (*pb.RangeResponse, bool)
	Compact(revision int64)
}

// keyFunc returns the key of an request, which is used to look up in the cache for it's caching response.
func keyFunc(req *pb.RangeRequest) uint64 {
	hash := adler32.New()
	deepHashObject(hash, req)
	return uint64(hash.Sum32())
}

func NewCache(maxCacheEntries int) Cache {
	return &cache{
		lru: lru.New(maxCacheEntries),
	}
}

// cache implements Cache
type cache struct {
	lock                sync.RWMutex
	lru                 *lru.Cache
	latestCompactionRev int64
}

// Add adds response for a request to the cache.
func (c *cache) Add(req *pb.RangeRequest, resp *pb.RangeResponse) {
	key := keyFunc(req)

	c.lock.Lock()
	defer c.lock.Unlock()

	if req.Revision > c.latestCompactionRev {
		c.lru.Add(key, resp)
	}
}

// Get looks up the caching response for a given request.
func (c *cache) Get(req *pb.RangeRequest) (*pb.RangeResponse, bool) {
	key := keyFunc(req)

	c.lock.RLock()
	defer c.lock.RUnlock()

	if req.Revision > c.latestCompactionRev {
		c.lru.Remove(key)
		return nil, false
	}

	if resp, ok := c.lru.Get(key); ok {
		return resp.(*pb.RangeResponse), ok
	}
	return nil, false
}

// Compact invalidate all caching response before the given rev.
// Note: the invalidation is lazy
func (c *cache) Compact(revision int64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if revision > c.latestCompactionRev {
		c.latestCompactionRev = revision
	}
}

// deepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
// This function is borrowed from kubernetes project.
func deepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", objectToWrite)
}
