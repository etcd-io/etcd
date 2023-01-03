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

package linearizability

import (
	"sync"
)

type leaseIdProvider interface {
	LeaseId(int) int64
	AddLeaseId(int, int64)
	RemoveLeaseId(int, int64)
}

func newLeaseIdProvider() leaseIdProvider {
	return &atomicLeaseIdProvider{m: map[int]int64{}}
}

type atomicLeaseIdProvider struct {
	sync.RWMutex
	m map[int]int64
}

func (lp *atomicLeaseIdProvider) LeaseId(clientId int) int64 {
	lp.RLock()
	defer lp.RUnlock()
	return lp.m[clientId]
}

func (lp *atomicLeaseIdProvider) AddLeaseId(clientId int, leaseId int64) {
	lp.Lock()
	defer lp.Unlock()
	lp.m[clientId] = leaseId
}

func (lp *atomicLeaseIdProvider) RemoveLeaseId(clientId int, leaseId int64) {
	lp.Lock()
	defer lp.Unlock()
	delete(lp.m, clientId)
}
