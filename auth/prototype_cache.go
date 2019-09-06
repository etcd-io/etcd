package auth

import (
	"github.com/coreos/etcd/auth/authpb"
)

type CachedPrototype struct {
	idx    int64
	orig   *authpb.Prototype
	fields map[string]*authpb.PrototypeField
}

type PrototypeCache struct {
	rev    int64
	byIdx  map[int64]*CachedPrototype
	byName map[string]*CachedPrototype
}

func newPrototypeCache(rev int64, prototypes []*authpb.Prototype) *PrototypeCache {
	pc := &PrototypeCache{
		rev:    rev,
		byIdx:  make(map[int64]*CachedPrototype),
		byName: make(map[string]*CachedPrototype),
	}

	// TODO: fill in the cache

	return pc
}

func (pc *PrototypeCache) Update(prototype *authpb.Prototype) *PrototypeCache {
	// TODO: update, return new cache if changed
	return nil
}

func (pc *PrototypeCache) GetIdx(name string) int64 {
	// TODO: get prototype index
	return 0
}

func (pc *PrototypeCache) GetPrototype(idx int64) *CachedPrototype {
	// TODO: get prototype by idx
	return nil
}
