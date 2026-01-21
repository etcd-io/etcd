// Copyright 2015 The etcd Authors
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

package mvcc

import (
	"bytes"
	"fmt"

	"github.com/google/btree"
	"go.uber.org/zap"
)

type index interface {
	Get(key []byte, atRev int64) (rev, created Revision, ver int64, err error)
	Range(key, end []byte, atRev int64) ([][]byte, []Revision)
	Revisions(key, end []byte, atRev int64, limit int) ([]Revision, int)
	CountRevisions(key, end []byte, atRev int64) int
	Put(key []byte, rev Revision)
	Tombstone(key []byte, rev Revision) error
	Compact(rev int64) map[Revision]struct{}
	Keep(rev int64) map[Revision]struct{}
}

type treeIndex struct {
	baseRev      int64
	revisionTree []*btree.BTreeG[keyRev]
	lg           *zap.Logger
}

type keyRev struct {
	key          []byte
	mod, created Revision
	version      int64
}

var lessThen btree.LessFunc[keyRev] = func(k keyRev, k2 keyRev) bool {
	return compare(k, k2) == -1
}

func compare(k keyRev, k2 keyRev) int {
	return bytes.Compare(k.key, k2.key)
}

func newTreeIndex(lg *zap.Logger) index {
	return &treeIndex{
		baseRev: 1,
		lg:      lg,
		revisionTree: []*btree.BTreeG[keyRev]{
			btree.NewG(32, lessThen),
		},
	}
}

func (ti *treeIndex) Put(key []byte, rev Revision) {
	prevTree := ti.revisionTree[len(ti.revisionTree)-1]
	item, found := prevTree.Get(keyRev{key: key})
	created := rev
	var version int64 = 1
	if found {
		created = item.created
		version = item.version + 1
	}
	tirev := ti.rev()
	if rev.Main == tirev {
		prevTree.ReplaceOrInsert(keyRev{
			key:     key,
			mod:     rev,
			created: created,
			version: version,
		})
	} else if rev.Main == tirev+1 {
		newTree := prevTree.Clone()
		newTree.ReplaceOrInsert(keyRev{
			key:     key,
			mod:     rev,
			created: created,
			version: version,
		})
		ti.revisionTree = append(ti.revisionTree, newTree)
	} else {
		panic(fmt.Sprintf("append only, lastRev: %d, putRev: %d", ti.rev(), rev.Main))
	}
}

func (ti *treeIndex) rev() int64 {
	return ti.baseRev + int64(len(ti.revisionTree)) - 1
}

func (ti *treeIndex) Get(key []byte, atRev int64) (modified, created Revision, ver int64, err error) {
	tree, err := ti.forRev(atRev)
	if err != nil {
		return modified, created, ver, err
	}
	keyRev, found := tree.Get(keyRev{key: key})
	if !found {
		return Revision{}, Revision{}, 0, ErrRevisionNotFound
	}
	return keyRev.mod, keyRev.created, keyRev.version, nil
}

// Revisions returns limited number of revisions from key(included) to end(excluded)
// at the given rev. The returned slice is sorted in the order of key. There is no limit if limit <= 0.
// The second return parameter isn't capped by the limit and reflects the total number of revisions.
func (ti *treeIndex) Revisions(key, end []byte, atRev int64, limit int) (revs []Revision, total int) {
	if end == nil {
		rev, _, _, err := ti.Get(key, atRev)
		if err != nil {
			return nil, 0
		}
		return []Revision{rev}, 1
	}
	tree, err := ti.forRev(atRev)
	if err != nil {
		return revs, total
	}
	tree.AscendRange(keyRev{key: key}, keyRev{key: end}, func(kr keyRev) bool {
		if limit <= 0 || len(revs) < limit {
			revs = append(revs, kr.mod)
		}
		total++
		return true
	})
	return revs, total
}

// CountRevisions returns the number of revisions
// from key(included) to end(excluded) at the given rev.
func (ti *treeIndex) CountRevisions(key, end []byte, atRev int64) int {
	if end == nil {
		_, _, _, err := ti.Get(key, atRev)
		if err != nil {
			return 0
		}
		return 1
	}
	total := 0
	tree, err := ti.forRev(atRev)
	if err != nil {
		return total
	}
	tree.AscendRange(keyRev{key: key}, keyRev{key: end}, func(kr keyRev) bool {
		total++
		return true
	})
	return total
}

func (ti *treeIndex) Range(key, end []byte, atRev int64) (keys [][]byte, revs []Revision) {
	if end == nil {
		rev, _, _, err := ti.Get(key, atRev)
		if err != nil {
			return nil, nil
		}
		return [][]byte{key}, []Revision{rev}
	}
	tree, err := ti.forRev(atRev)
	if err != nil {
		return keys, revs
	}
	tree.AscendRange(keyRev{key: key}, keyRev{key: end}, func(kr keyRev) bool {
		revs = append(revs, kr.mod)
		keys = append(keys, kr.key)
		return true
	})
	return keys, revs
}

func (ti *treeIndex) Tombstone(key []byte, rev Revision) error {
	prevTree := ti.revisionTree[len(ti.revisionTree)-1]
	tirev := ti.rev()
	var found bool
	if rev.Main == tirev {
		_, found = prevTree.Delete(keyRev{
			key: key,
		})
	} else if rev.Main == tirev+1 {
		newTree := prevTree.Clone()
		_, found = newTree.Delete(keyRev{
			key: key,
		})
		ti.revisionTree = append(ti.revisionTree, newTree)
	} else {
		panic(fmt.Sprintf("append only, lastRev: %d, putRev: %d", ti.rev(), rev.Main))
	}
	if !found {
		return ErrRevisionNotFound
	}
	return nil
}

func (ti *treeIndex) Compact(rev int64) map[Revision]struct{} {
	available := make(map[Revision]struct{})
	ti.lg.Info("compact tree index", zap.Int64("revision", rev))
	idx := rev - ti.baseRev
	ti.revisionTree = ti.revisionTree[idx:]
	ti.baseRev = rev
	return available
}

// Keep finds all revisions to be kept for a Compaction at the given rev.
func (ti *treeIndex) Keep(rev int64) map[Revision]struct{} {
	available := make(map[Revision]struct{})
	tree, err := ti.forRev(rev)
	if err != nil {
		return available
	}
	tree.Ascend(func(item keyRev) bool {
		available[item.mod] = struct{}{}
		return true
	})
	return available
}

func (ti *treeIndex) forRev(rev int64) (*btree.BTreeG[keyRev], error) {
	idx := rev - ti.baseRev
	if idx < 0 || idx >= int64(len(ti.revisionTree)) {
		return nil, ErrRevisionNotFound
	}
	return ti.revisionTree[idx], nil
}
