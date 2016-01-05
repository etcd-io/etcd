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
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/etcd/lease/leasepb"
	"github.com/coreos/etcd/pkg/idutil"
	"github.com/coreos/etcd/storage/backend"
)

var (
	minLeaseTerm = 5 * time.Second

	leaseBucketName = []byte("lease")
)

// DeleteableRange defines an interface with DeleteRange method.
// We define this interface only for lessor to limit the number
// of methods of storage.KV to what lessor actually needs.
//
// Having a minimum interface makes testing easy.
type DeleteableRange interface {
	DeleteRange(key, end []byte) (int64, int64)
}

// a lessor is the owner of leases. It can grant, revoke,
// renew and modify leases for lessee.
// TODO: use clockwork for testability.
type lessor struct {
	mu sync.Mutex
	// TODO: probably this should be a heap with a secondary
	// id index.
	// Now it is O(N) to loop over the leases to find expired ones.
	// We want to make Grant, Revoke, and FindExpired all O(logN) and
	// Renew O(1).
	// FindExpired and Renew should be the most frequent operations.
	leaseMap map[int64]*lease

	// A DeleteableRange the lessor operates on.
	// When a lease expires, the lessor will delete the
	// leased range (or key) from the DeleteableRange.
	dr DeleteableRange

	// backend to persist leases. We only persist lease ID and expiry for now.
	// The leased items can be recovered by iterating all the keys in kv.
	b backend.Backend

	idgen *idutil.Generator
}

func NewLessor(lessorID uint8, b backend.Backend, dr DeleteableRange) *lessor {
	l := &lessor{
		leaseMap: make(map[int64]*lease),
		b:        b,
		dr:       dr,
		idgen:    idutil.NewGenerator(lessorID, time.Now()),
	}

	tx := l.b.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(leaseBucketName)
	tx.Unlock()
	l.b.ForceCommit()

	// TODO: recover from previous state in backend.

	return l
}

// Grant grants a lease that expires at least after TTL seconds.
// TODO: when lessor is under high load, it should give out lease
// with longer TTL to reduce renew load.
func (le *lessor) Grant(ttl int64) *lease {
	// TODO: define max TTL
	expiry := time.Now().Add(time.Duration(ttl) * time.Second)
	expiry = minExpiry(time.Now(), expiry)

	id := int64(le.idgen.Next())

	le.mu.Lock()
	defer le.mu.Unlock()

	l := &lease{id: id, ttl: ttl, expiry: expiry, itemSet: make(map[leaseItem]struct{})}
	if _, ok := le.leaseMap[id]; ok {
		panic("lease: unexpected duplicate ID!")
	}

	le.leaseMap[id] = l
	l.persistTo(le.b)

	return l
}

// Revoke revokes a lease with given ID. The item attached to the
// given lease will be removed. If the ID does not exist, an error
// will be returned.
func (le *lessor) Revoke(id int64) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	l := le.leaseMap[id]
	if l == nil {
		return fmt.Errorf("lease: cannot find lease %x", id)
	}

	for item := range l.itemSet {
		le.dr.DeleteRange([]byte(item.key), nil)
	}

	delete(le.leaseMap, l.id)
	l.removeFrom(le.b)

	return nil
}

// Renew renews an existing lease. If the given lease does not exist or
// has expired, an error will be returned.
// TODO: return new TTL?
func (le *lessor) Renew(id int64) error {
	le.mu.Lock()
	defer le.mu.Unlock()

	l := le.leaseMap[id]
	if l == nil {
		return fmt.Errorf("lease: cannot find lease %x", id)
	}

	expiry := time.Now().Add(time.Duration(l.ttl) * time.Second)
	l.expiry = minExpiry(time.Now(), expiry)
	return nil
}

// Attach attaches items to the lease with given ID. When the lease
// expires, the attached items will be automatically removed.
// If the given lease does not exist, an error will be returned.
func (le *lessor) Attach(id int64, items []leaseItem) error {
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
func (le *lessor) get(id int64) *lease {
	le.mu.Lock()
	defer le.mu.Unlock()

	return le.leaseMap[id]
}

type lease struct {
	id  int64
	ttl int64 // time to live in seconds

	itemSet map[leaseItem]struct{}
	// expiry time in unixnano
	expiry time.Time
}

func (l lease) persistTo(b backend.Backend) {
	key := int64ToBytes(l.id)

	lpb := leasepb.Lease{ID: l.id, TTL: int64(l.ttl)}
	val, err := lpb.Marshal()
	if err != nil {
		panic("failed to marshal lease proto item")
	}

	b.BatchTx().Lock()
	b.BatchTx().UnsafePut(leaseBucketName, key, val)
	b.BatchTx().Unlock()
}

func (l lease) removeFrom(b backend.Backend) {
	key := int64ToBytes(l.id)

	b.BatchTx().Lock()
	b.BatchTx().UnsafeDelete(leaseBucketName, key)
	b.BatchTx().Unlock()
}

type leaseItem struct {
	key string
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

func int64ToBytes(n int64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(n))
	return bytes
}
