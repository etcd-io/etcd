// Copyright 2022 The etcd Authors
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

package identity

import (
	"math/rand"
	"sync"
)

type LeaseIdStorage interface {
	// Add adds lease to storage.
	Add(int64)

	// Remove permanently removes lease from storage.
	Remove(int64)

	// Take returns an unused lease from storage
	Take() int64

	// Return returns a used lease to storage so it can be used later.
	Return(int64)

	// GetRandom returns a random lease (could be in use or not)
	GetRandom() int64

	// Count returns count of leases (could be in use or not)
	Count() int
}

func NewLeaseIdStorage() LeaseIdStorage {
	return &atomicLeaseKeeper{ma: map[int64]int{}, mf: map[int64]int{}}
}

const noLease int64 = 0

// atomicLeaseKeeper stores the leases in two lists - one for free leases and one for all leases.
// two auxiliary maps are kept to allow fast deletion from the lists
type atomicLeaseKeeper struct {
	sync.RWMutex
	// la is used to keep all leases
	la []int64
	// ma is used to store mapping of leaseId to index in the slice la.
	ma map[int64]int
	// lf stores leases that are free(not in use)
	lf []int64
	// mf is used to store mapping of leaseId to index in the slice lf.
	mf map[int64]int
}

func (lm *atomicLeaseKeeper) Add(leaseId int64) {
	lm.Lock()
	defer lm.Unlock()

	lm.addToGlobalUnsafe(leaseId)
	lm.addToFreeUnsafe(leaseId)
}

func (lm *atomicLeaseKeeper) addToGlobalUnsafe(leaseId int64) {
	if _, ok := lm.ma[leaseId]; ok {
		return
	}

	lm.la = append(lm.la, leaseId)
	lm.ma[leaseId] = len(lm.la) - 1
}

func (lm *atomicLeaseKeeper) addToFreeUnsafe(leaseId int64) {
	if _, ok := lm.mf[leaseId]; ok {
		return
	}

	lm.lf = append(lm.lf, leaseId)
	lm.mf[leaseId] = len(lm.lf) - 1
}

func (lm *atomicLeaseKeeper) Remove(leaseId int64) {
	lm.Lock()
	defer lm.Unlock()

	lm.removeFromGlobalUnsafe(leaseId)
	lm.removeFromFreeUnsafe(leaseId)
}

func (lm *atomicLeaseKeeper) removeFromGlobalUnsafe(leaseId int64) {
	if index, ok := lm.ma[leaseId]; ok {
		delete(lm.ma, leaseId)
		lm.la = lm.removeFromSliceUnsafe(lm.ma, lm.la, index)
	}
}

func (lm *atomicLeaseKeeper) removeFromFreeUnsafe(leaseId int64) {
	if index, ok := lm.mf[leaseId]; ok {
		delete(lm.mf, leaseId)
		lm.lf = lm.removeFromSliceUnsafe(lm.mf, lm.lf, index)
	}
}

func (lm *atomicLeaseKeeper) removeFromSliceUnsafe(m map[int64]int, s []int64, i int) []int64 {
	//swap the item at index with last element and remove the last item
	s[i], s[len(s)-1] = s[len(s)-1], s[i]
	if _, ok := m[s[i]]; ok {
		m[s[i]] = i
	}
	s = s[:len(s)-1]
	return s
}

func (lm *atomicLeaseKeeper) Take() int64 {
	lm.Lock()
	defer lm.Unlock()

	if len(lm.lf) == 0 {
		return noLease
	}

	leaseId := lm.getRandomUnsafe(lm.lf)
	lm.removeFromFreeUnsafe(leaseId)
	return leaseId
}

func (lm *atomicLeaseKeeper) GetRandom() int64 {
	lm.RLock()
	defer lm.RUnlock()

	if len(lm.la) == 0 {
		return noLease
	}

	return lm.getRandomUnsafe(lm.la)
}

func (lm *atomicLeaseKeeper) getRandomUnsafe(s []int64) int64 {
	i := rand.Intn(len(s))
	return s[i]
}

func (lm *atomicLeaseKeeper) Return(leaseId int64) {
	lm.Lock()
	defer lm.Unlock()

	//if the lease is revoked by now, dont add
	if _, ok := lm.ma[leaseId]; !ok {
		return
	}
	lm.addToFreeUnsafe(leaseId)
}

func (lm *atomicLeaseKeeper) Count() int {
	lm.RLock()
	defer lm.RUnlock()

	return len(lm.la)
}
