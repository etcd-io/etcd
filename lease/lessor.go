// Copyright 2015 CoreOS, Inc.
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

package lease

import (
	"fmt"
	"sync"
	"time"

	"github.com/coreos/etcd/pkg/idutil"
)

var (
	minLeaseTerm = 5 * time.Second
)

// a lessor is the owner of leases. It can grant, revoke,
// renew and modify leases for lessee.
// TODO: persist lease on to stable backend for failure recovery.
// TODO: use clockwork for testability.
type lessor struct {
	mu sync.Mutex
	// TODO: probably this should be a heap with a secondary
	// id index.
	// Now it is O(N) to loop over the leases to find expired ones.
	// We want to make Grant, Revoke, and FindExpired all O(logN) and
	// Renew O(1).
	// FindExpired and Renew should be the most frequent operations.
	leaseMap map[uint64]*lease

	idgen *idutil.Generator
}

func NewLessor(lessorID uint8) *lessor {
	return &lessor{
		leaseMap: make(map[uint64]*lease),
		idgen:    idutil.NewGenerator(lessorID, time.Now()),
	}
}

// Grant grants a lease that expires at least at the given expiry
// time.
// TODO: when leassor is under highload, it should give out lease
// with longer term to reduce renew load.
func (le *lessor) Grant(expiry time.Time) *lease {
	expiry = minExpiry(time.Now(), expiry)

	id := le.idgen.Next()

	le.mu.Lock()
	defer le.mu.Unlock()

	l := &lease{id: id, expiry: expiry}
	if _, ok := le.leaseMap[id]; ok {
		panic("lease: unexpected duplicate ID!")
	}

	le.leaseMap[id] = l
	return l
}

// Revoke revokes a lease with given ID. The item attached to the
// given lease will be removed. If the ID does not exist, an error
// will be returned.
func (le *lessor) Revoke(id uint64) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	l := le.leaseMap[id]
	if l == nil {
		return fmt.Errorf("lease: cannot find lease %x", id)
	}

	delete(le.leaseMap, l.id)

	// TODO: remove attached items
	return nil
}

// Renew renews an existing lease with at least the given expiry.
// If the given lease does not exist or has expired, an error will
// be returned.
func (le *lessor) Renew(id uint64, expiry time.Time) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	l := le.leaseMap[id]
	if l == nil {
		return fmt.Errorf("lease: cannot find lease %x", id)
	}

	l.expiry = minExpiry(time.Now(), expiry)
	return nil
}

// Attach attaches items to the lease with given ID. When the lease
// expires, the attached items will be automatically removed.
// If the given lease does not exist, an error will be returned.
func (le *lessor) Attach(id uint64, items []leaseItem) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	l := le.leaseMap[id]
	if l == nil {
		return fmt.Errorf("lease: cannot find lease %x", id)
	}

	for _, it := range items {
		l.itemSet[it] = struct{}{}
	}
	return nil
}

// findExpiredLeases loops all the leases in the leaseMap and returns the expired
// leases that needed to be revoked.
func (le *lessor) findExpiredLeases() []*lease {
	le.mu.Lock()
	defer le.mu.Unlock()

	leases := make([]*lease, 0, 16)
	now := time.Now()

	for _, l := range le.leaseMap {
		if l.expiry.Sub(now) <= 0 {
			leases = append(leases, l)
		}
	}

	return leases
}

// get gets the lease with given id.
// get is a helper fucntion for testing, at least for now.
func (le *lessor) get(id uint64) *lease {
	le.mu.Lock()
	defer le.mu.Unlock()

	return le.leaseMap[id]
}

type lease struct {
	id uint64

	itemSet map[leaseItem]struct{}
	// expiry time in unixnano
	expiry time.Time
}

type leaseItem struct {
	key      string
	endRange string
}

// minExpiry returns a minimal expiry. A minimal expiry is the larger on
// between now + minLeaseTerm and the given expectedExpiry.
func minExpiry(now time.Time, expectedExpiry time.Time) time.Time {
	minExpiry := time.Now().Add(minLeaseTerm)
	if expectedExpiry.Sub(minExpiry) < 0 {
		expectedExpiry = minExpiry
	}
	return expectedExpiry
}
