package auth

import (
	"github.com/coreos/etcd/auth/authpb"
)

type CachedPrototype struct {
	Idx    int64
	Orig   *authpb.Prototype
	Fields map[string]*authpb.PrototypeField
}

func newCachedPrototype(idx int64, orig *authpb.Prototype) (*CachedPrototype, error) {
	cachedProto := &CachedPrototype{
		Idx:    idx,
		Orig:   orig,
		Fields: make(map[string]*authpb.PrototypeField),
	}

	for _, field := range orig.Fields {
		_, ok := cachedProto.Fields[field.Key]
		if ok {
			return nil, ErrPrototypeDuplicateKey
		}
		cachedProto.Fields[field.Key] = field
	}

	return cachedProto, nil
}

type PrototypeCache struct {
	Rev     int64
	LastIdx int64
	byIdx   map[int64]*CachedPrototype
	byName  map[string]*CachedPrototype
}

func newPrototypeCache(rev int64, lastIdx int64, protoIdxs []int64, prototypes []*authpb.Prototype) *PrototypeCache {
	pc := &PrototypeCache{
		Rev:     rev,
		LastIdx: lastIdx,
		byIdx:   make(map[int64]*CachedPrototype),
		byName:  make(map[string]*CachedPrototype),
	}

	for i, protoIdx := range protoIdxs {
		proto := prototypes[i]

		if protoIdx > pc.LastIdx {
			plog.Panicf("prototype idx %v > last idx %v", protoIdx, pc.LastIdx)
		}

		_, ok := pc.byIdx[protoIdx]
		if ok {
			plog.Panicf("prototype %v already exists", protoIdx)
		}
		_, ok = pc.byName[string(proto.Name)]
		if ok {
			plog.Panicf("prototype %s already exists", string(proto.Name))
		}

		cachedProto, err := newCachedPrototype(protoIdx, proto)
		if err != nil {
			plog.Panicf("cannot create prototype: %s", err)
		}

		pc.byIdx[protoIdx] = cachedProto
		pc.byName[string(proto.Name)] = cachedProto
	}

	return pc
}

func (pc *PrototypeCache) Update(prototype *authpb.Prototype) (*PrototypeCache, *CachedPrototype, error) {
	newPc := &PrototypeCache{
		Rev:     pc.Rev,
		LastIdx: pc.LastIdx,
		byIdx:   make(map[int64]*CachedPrototype),
		byName:  make(map[string]*CachedPrototype),
	}

	for key, val := range pc.byIdx {
		newPc.byIdx[key] = val
	}
	for key, val := range pc.byName {
		newPc.byName[key] = val
	}

	var idx int64 = 0
	cachedProto, exists := newPc.byName[string(prototype.Name)]
	if exists {
		idx = cachedProto.Idx
	} else {
		idx = newPc.LastIdx + 1
	}

	cachedProto, err := newCachedPrototype(idx, prototype)
	if err != nil {
		return nil, nil, err
	}

	newPc.byIdx[idx] = cachedProto
	newPc.byName[string(prototype.Name)] = cachedProto

	if !exists {
		newPc.LastIdx = idx
	}
	newPc.Rev++

	return newPc, cachedProto, nil
}

func (pc *PrototypeCache) GetIdx(name string) int64 {
	// TODO: get prototype index
	return 0
}

func (pc *PrototypeCache) GetPrototype(idx int64) *CachedPrototype {
	// TODO: get prototype by idx
	return nil
}
