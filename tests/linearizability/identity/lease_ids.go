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
	"sync"
)

type LeaseIdStorage interface {
	LeaseId(int) int64
	AddLeaseId(int, int64)
	RemoveLeaseId(int)
}

func NewLeaseIdStorage() LeaseIdStorage {
	return &atomicClientId2LeaseIdMapper{m: map[int]int64{}}
}

type atomicClientId2LeaseIdMapper struct {
	sync.RWMutex
	// m is used to store clientId to leaseId mapping.
	m map[int]int64
}

func (lm *atomicClientId2LeaseIdMapper) LeaseId(clientId int) int64 {
	lm.RLock()
	defer lm.RUnlock()
	return lm.m[clientId]
}

func (lm *atomicClientId2LeaseIdMapper) AddLeaseId(clientId int, leaseId int64) {
	lm.Lock()
	defer lm.Unlock()
	lm.m[clientId] = leaseId
}

func (lm *atomicClientId2LeaseIdMapper) RemoveLeaseId(clientId int) {
	lm.Lock()
	defer lm.Unlock()
	delete(lm.m, clientId)
}
