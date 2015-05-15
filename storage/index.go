package storage

import (
	"log"
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/google/btree"
)

type index interface {
	Get(key []byte, atIndex uint64) (index uint64, err error)
	Put(key []byte, index uint64)
	Tombstone(key []byte, index uint64) error
	Compact(index uint64) map[uint64]struct{}
}

type treeIndex struct {
	sync.RWMutex
	tree *btree.BTree
}

func newTreeIndex() index {
	return &treeIndex{
		tree: btree.New(32),
	}
}

func (ti *treeIndex) Put(key []byte, index uint64) {
	keyi := &keyIndex{key: key}

	ti.Lock()
	defer ti.Unlock()
	item := ti.tree.Get(keyi)
	if item == nil {
		keyi.put(index)
		ti.tree.ReplaceOrInsert(keyi)
		return
	}
	okeyi := item.(*keyIndex)
	okeyi.put(index)
}

func (ti *treeIndex) Get(key []byte, atIndex uint64) (index uint64, err error) {
	keyi := &keyIndex{key: key}

	ti.RLock()
	defer ti.RUnlock()
	item := ti.tree.Get(keyi)
	if item == nil {
		return 0, ErrIndexNotFound
	}

	keyi = item.(*keyIndex)
	return keyi.get(atIndex)
}

func (ti *treeIndex) Tombstone(key []byte, index uint64) error {
	keyi := &keyIndex{key: key}

	ti.Lock()
	defer ti.Unlock()
	item := ti.tree.Get(keyi)
	if item == nil {
		return ErrIndexNotFound
	}

	ki := item.(*keyIndex)
	ki.tombstone(index)
	return nil
}

func (ti *treeIndex) Compact(index uint64) map[uint64]struct{} {
	available := make(map[uint64]struct{})
	emptyki := make([]*keyIndex, 0)
	log.Printf("store.index: compact %d", index)
	// TODO: do not hold the lock for long time?
	// This is probably OK. Compacting 10M keys takes O(10ms).
	ti.Lock()
	defer ti.Unlock()
	ti.tree.Ascend(compactIndex(index, available, &emptyki))
	for _, ki := range emptyki {
		item := ti.tree.Delete(ki)
		if item == nil {
			log.Panic("store.index: unexpected delete failure during compaction")
		}
	}
	return available
}

func compactIndex(index uint64, available map[uint64]struct{}, emptyki *[]*keyIndex) func(i btree.Item) bool {
	return func(i btree.Item) bool {
		keyi := i.(*keyIndex)
		keyi.compact(index, available)
		if keyi.isEmpty() {
			*emptyki = append(*emptyki, keyi)
		}
		return true
	}
}
