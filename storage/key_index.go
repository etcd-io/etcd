package storage

import (
	"bytes"
	"errors"
	"log"

	"github.com/google/btree"
)

var (
	ErrIndexNotFound = errors.New("index: not found")
)

// keyIndex stores the index of an key in the backend.
// Each keyIndex has at least one key generation.
// Each generation might have several key versions.
// Tombstone on a key appends an tombstone version at the end
// of the current generation and creates a new empty generation.
// Each version of a key has an index pointing to the backend.
//
// For example: put(1);put(2);tombstone(3);put(4);tombstone(5) on key "foo"
// generate a keyIndex:
// key:     "foo"
// index: 5
// generations:
//    {empty}
//    {4, 5(t)}
//    {1, 2, 3(t)}
//
// Compact a keyIndex removes the versions with smaller or equal to
// index except the largest one. If the generations becomes empty
// during compaction, it will be removed. if all the generations get
// removed, the keyIndex Should be removed.

// For example:
// compact(2) on the previous example
// generations:
//    {empty}
//    {4, 5(t)}
//    {2, 3(t)}
//
// compact(4)
// generations:
//    {empty}
//    {4, 5(t)}
//
// compact(5):
// generations:
//    {empty}
//    {5(t)}
//
// compact(6):
// generations:
//    {empty} -> key SHOULD be removed.
type keyIndex struct {
	key         []byte
	index       uint64
	generations []generation
}

// put puts an index to the keyIndex.
func (ki *keyIndex) put(index uint64) {
	if index < ki.index {
		log.Panicf("store.keyindex: put with unexpected smaller index [%d / %d]", index, ki.index)
	}
	if len(ki.generations) == 0 {
		ki.generations = append(ki.generations, generation{})
	}
	g := &ki.generations[len(ki.generations)-1]
	g.cont = append(g.cont, index)
	g.ver++
	ki.index = index
}

// tombstone puts an index, pointing to a tombstone, to the keyIndex.
// It also creates a new empty generation in the keyIndex.
func (ki *keyIndex) tombstone(index uint64) {
	if ki.isEmpty() {
		log.Panicf("store.keyindex: unexpected tombstone on empty keyIndex %s", string(ki.key))
	}
	ki.put(index)
	ki.generations = append(ki.generations, generation{})
}

// get gets the index of thk that satisfies the given atIndex.
// Index must be lower or equal to the given atIndex.
func (ki *keyIndex) get(atIndex uint64) (index uint64, err error) {
	if ki.isEmpty() {
		log.Panicf("store.keyindex: unexpected get on empty keyIndex %s", string(ki.key))
	}
	g := ki.findGeneration(atIndex)
	if g.isEmpty() {
		return 0, ErrIndexNotFound
	}

	f := func(index, ver uint64) bool {
		if index <= atIndex {
			return false
		}
		return true
	}

	_, n := g.walk(f)
	if n != -1 {
		return g.cont[n], nil
	}
	return 0, ErrIndexNotFound
}

// compact compacts a keyIndex by removing the versions with smaller or equal
// index than the given atIndex except the largest one.
// If a generation becomes empty during compaction, it will be removed.
func (ki *keyIndex) compact(atIndex uint64, available map[uint64]struct{}) {
	if ki.isEmpty() {
		log.Panic("store.keyindex: unexpected compact on empty keyIndex %s", string(ki.key))
	}
	// walk until reaching the first content that has an index smaller or equal to
	// the atIndex.
	// add all the reached indexes into available map.
	f := func(index, _ uint64) bool {
		available[index] = struct{}{}
		if index <= atIndex {
			return false
		}
		return true
	}

	g := ki.findGeneration(atIndex)
	i := len(ki.generations) - 1
	for i >= 0 {
		wg := &ki.generations[i]
		if wg == g {
			break
		}
		wg.walk(f)
		i--
	}

	_, n := g.walk(f)

	// remove the previous contents.
	if n != -1 {
		g.cont = g.cont[n:]
	}
	// remove the previous generations.
	ki.generations = ki.generations[i:]

	return
}

func (ki *keyIndex) isEmpty() bool {
	return len(ki.generations) == 1 && ki.generations[0].isEmpty()
}

// findGeneartion finds out the generation of the keyIndex that the
// given index belongs to.
func (ki *keyIndex) findGeneration(index uint64) *generation {
	g, youngerg := len(ki.generations)-1, len(ki.generations)-2

	// If the head index of a younger generation is smaller than
	// the given index, the index cannot be in the younger
	// generation.
	for youngerg >= 0 && ki.generations[youngerg].cont != nil {
		yg := ki.generations[youngerg]
		if yg.cont[len(yg.cont)-1] < index {
			break
		}
		g--
		youngerg--
	}
	if g < 0 {
		return nil
	}
	return &ki.generations[g]
}

func (a *keyIndex) Less(b btree.Item) bool {
	return bytes.Compare(a.key, b.(*keyIndex).key) == -1
}

type generation struct {
	ver  uint64
	cont []uint64
}

func (g *generation) isEmpty() bool { return len(g.cont) == 0 }

func (g *generation) walk(f func(index, ver uint64) bool) (uint64, int) {
	ver := g.ver
	l := len(g.cont)
	for i := range g.cont {
		ok := f(g.cont[l-i-1], ver)
		if !ok {
			return ver, l - i - 1
		}
		ver--
	}
	return 0, -1
}
