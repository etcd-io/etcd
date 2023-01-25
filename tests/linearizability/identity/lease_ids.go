// Copyright 2022 The etcd Authors
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

package identity

import (
	"math/rand"
	"sync"
)

type LeaseIdStorage interface {
	LeaseId(int) int64
	AddLeaseId(int, int64)
	RemoveLeaseId(int)
	GetRandomClientWithLease() (int, int64)
}

type clientLeaseMapping struct {
	clientId int
	leaseId  int64
}

func NewLeaseIdStorage() LeaseIdStorage {
	return &atomicClientId2LeaseIdMapper{m: map[int]int{}}
}

type atomicClientId2LeaseIdMapper struct {
	sync.RWMutex
	s []clientLeaseMapping
	// m is used to store mapping of clientId to index in the slice.
	m map[int]int
}

func (lm *atomicClientId2LeaseIdMapper) LeaseId(clientId int) int64 {
	lm.RLock()
	defer lm.RUnlock()
	if index, ok := lm.m[clientId]; ok {
		return lm.s[index].leaseId
	}
	return 0
}

func (lm *atomicClientId2LeaseIdMapper) AddLeaseId(clientId int, leaseId int64) {
	lm.Lock()
	defer lm.Unlock()
	mp := clientLeaseMapping{clientId: clientId, leaseId: leaseId}
	lm.s = append(lm.s, mp)
	lm.m[clientId] = len(lm.s) - 1
}

func (lm *atomicClientId2LeaseIdMapper) RemoveLeaseId(clientId int) {
	lm.Lock()
	defer lm.Unlock()
	removed, index := lm.removeFromMapUnsafe(clientId)
	if removed {
		lm.removeFromSliceUnsafe(index)
	}
}

func (lm *atomicClientId2LeaseIdMapper) removeFromMapUnsafe(clientId int) (bool, int) {
	if _, ok := lm.m[clientId]; ok {
		index := lm.m[clientId]
		delete(lm.m, clientId)
		return true, index
	}
	return false, -1
}

func (lm *atomicClientId2LeaseIdMapper) removeFromSliceUnsafe(index int) {
	//swap the item at index with last element and remove the last item
	lm.s[index], lm.s[len(lm.s)-1] = lm.s[len(lm.s)-1], lm.s[index]
	if _, ok := lm.m[lm.s[index].clientId]; ok {
		lm.m[lm.s[index].clientId] = index
	}
	lm.s = lm.s[:len(lm.s)-1]
}

func (lm *atomicClientId2LeaseIdMapper) GetRandomClientWithLease() (int, int64) {
	lm.RLock()
	defer lm.RUnlock()

	if len(lm.s) == 0 {
		return -1, 0
	}
	index := rand.Intn(len(lm.s))
	return lm.s[index].clientId, lm.s[index].leaseId
}
