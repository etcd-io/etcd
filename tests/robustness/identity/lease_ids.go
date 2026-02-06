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

type LeaseIDStorage interface {
	LeaseID(int) int64
	AddLeaseID(int, int64)
	RemoveLeaseID(int)
}

func NewLeaseIDStorage() LeaseIDStorage {
	return &atomicClientID2LeaseIDMapper{m: map[int]int64{}}
}

type atomicClientID2LeaseIDMapper struct {
	sync.RWMutex
	// m is used to store clientId to leaseId mapping.
	m map[int]int64
}

func (lm *atomicClientID2LeaseIDMapper) LeaseID(clientID int) int64 {
	lm.RLock()
	defer lm.RUnlock()
	return lm.m[clientID]
}

func (lm *atomicClientID2LeaseIDMapper) AddLeaseID(clientID int, leaseID int64) {
	lm.Lock()
	defer lm.Unlock()
	lm.m[clientID] = leaseID
}

func (lm *atomicClientID2LeaseIDMapper) RemoveLeaseID(clientID int) {
	lm.Lock()
	defer lm.Unlock()
	delete(lm.m, clientID)
}
