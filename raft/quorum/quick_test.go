// Copyright 2019 The etcd Authors
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

package quorum

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

// TestQuick uses quickcheck to heuristically assert that the main
// implementation of (MajorityConfig).CommittedIndex agrees with a "dumb"
// alternative version.
func TestQuick(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 50000,
	}

	t.Run("majority_commit", func(t *testing.T) {
		fn1 := func(c memberMap, l idxMap) uint64 {
			return uint64(MajorityConfig(c).CommittedIndex(mapAckIndexer(l)))
		}
		fn2 := func(c memberMap, l idxMap) uint64 {
			return uint64(alternativeMajorityCommittedIndex(MajorityConfig(c), mapAckIndexer(l)))
		}
		if err := quick.CheckEqual(fn1, fn2, cfg); err != nil {
			t.Fatal(err)
		}
	})
}

// smallRandIdxMap returns a reasonably sized map of ids to commit indexes.
func smallRandIdxMap(rand *rand.Rand, size int) map[uint64]Index {
	// Hard-code a reasonably small size here (quick will hard-code 50, which
	// is not useful here).
	size = 10

	n := rand.Intn(size)
	ids := rand.Perm(2 * n)[:n]
	idxs := make([]int, len(ids))
	for i := range idxs {
		idxs[i] = rand.Intn(n)
	}

	m := map[uint64]Index{}
	for i := range ids {
		m[uint64(ids[i])] = Index(idxs[i])
	}
	return m
}

type idxMap map[uint64]Index

func (idxMap) Generate(rand *rand.Rand, size int) reflect.Value {
	m := smallRandIdxMap(rand, size)
	return reflect.ValueOf(m)
}

type memberMap map[uint64]struct{}

func (memberMap) Generate(rand *rand.Rand, size int) reflect.Value {
	m := smallRandIdxMap(rand, size)
	mm := map[uint64]struct{}{}
	for id := range m {
		mm[id] = struct{}{}
	}
	return reflect.ValueOf(mm)
}

// This is an alternative implementation of (MajorityConfig).CommittedIndex(l).
func alternativeMajorityCommittedIndex(c MajorityConfig, l AckedIndexer) Index {
	if len(c) == 0 {
		return math.MaxUint64
	}

	idToIdx := map[uint64]Index{}
	for id := range c {
		if idx, ok := l.AckedIndex(id); ok {
			idToIdx[id] = idx
		}
	}

	// Build a map from index to voters who have acked that or any higher index.
	idxToVotes := map[Index]int{}
	for _, idx := range idToIdx {
		idxToVotes[idx] = 0
	}

	for _, idx := range idToIdx {
		for idy := range idxToVotes {
			if idy > idx {
				continue
			}
			idxToVotes[idy]++
		}
	}

	// Find the maximum index that has achieved quorum.
	q := len(c)/2 + 1
	var maxQuorumIdx Index
	for idx, n := range idxToVotes {
		if n >= q && idx > maxQuorumIdx {
			maxQuorumIdx = idx
		}
	}

	return maxQuorumIdx
}
