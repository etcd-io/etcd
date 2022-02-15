// Copyright 2021 The etcd Authors
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

package namespacequota

import (
	"errors"
	"go.etcd.io/etcd/server/v3/namespacequota/namespacequotapb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.uber.org/zap"
	"math"
	"sync"
	"time"
)

type NamespaceQuotaEnforcement int8

const (
	DISABLED NamespaceQuotaEnforcement = 0
	SOFTMODE NamespaceQuotaEnforcement = 1
	HARDMODE NamespaceQuotaEnforcement = 2
)

var (
	// namespaceQuotaBucketName defines the name of bucket in the backend
	namespaceQuotaBucketName       = []byte("namespacequota")
	ErrNamespaceQuotaExceeded      = errors.New("namespace quota exceeded")
	ErrNamespaceQuotaNotFound      = errors.New("namespace quota not found")
	ErrNamespaceQuotaRestoreFailed = errors.New("namespace quota restore failed")
)

// NamespaceQuota represents a namespace quota
type NamespaceQuota struct {
	// Key represents the namespace key
	Key []byte
	// QuotaByteCount quota byte count represents the byte quota for respective Key
	QuotaByteCount uint64
	// QuotaKeyCount quota key count represents the quota key quota for respective Key
	QuotaKeyCount uint64
	// UsageByteCount represents the current usage of bytes for respective Key
	UsageByteCount uint64
	// UsageKeyCount represents the current usage of bytes for respective Key
	UsageKeyCount uint64
}

// ReadView is a ReadView that only permits reads. Defined here
// to avoid circular dependency with mvcc.
type ReadView interface {
	RangeValueSize(key, end []byte) (keys [][]byte, valueSizes []int)
	GetValueSize(key []byte) (valueSize int, isFound bool)
}

// NamespaceQuotaManagerConfig represents the namespace quota manager configuration
type NamespaceQuotaManagerConfig struct {
	NamespaceQuotaEnforcement int
}

// NamespaceQuotaManager owns NamespaceQuota. NamespaceQuotaManager can create, update, read, and delete NamespaceQuota.
type NamespaceQuotaManager interface {

	// SetReadView lets the namespace quota manager create ReadView to the store.
	// NamespaceQuotaManager ranges over the items
	SetReadView(rv ReadView)

	// SetNamespaceQuota creates or updates NamespaceQuota
	// If quota exists, it will be updated
	// Else created
	SetNamespaceQuota(key []byte, quotaByteCount uint64, quotaKeyCount uint64) (quota *NamespaceQuota, err error)

	// GetNamespaceQuota returns the NamespaceQuota on the key if it exists, else responds with an error
	GetNamespaceQuota(key []byte) (quota *NamespaceQuota, err error)

	// ListNamespaceQuotas returns all the NamespaceQuota available in the tree
	ListNamespaceQuotas() (quotas []*NamespaceQuota)

	// DeleteNamespaceQuota deletes the NamespaceQuota if exists
	DeleteNamespaceQuota(key []byte) (quota *NamespaceQuota, err error)

	// UpdateNamespaceQuotaUsage updates the usage of a key
	UpdateNamespaceQuotaUsage(key []byte, byteDiff int, keyDiff int)

	// DeleteRangeUpdateUsage will update usage in the quota tree when an etcd delete range operation is performed
	DeleteRangeUpdateUsage(keys [][]byte, valueSizes []int)

	// DiffNamespaceQuotaUsage diffs namespace quota usage, if incoming new key exceeds quota, error is returned
	DiffNamespaceQuotaUsage(key, newValue []byte) (byteDiff int, keyDiff int)

	// IsQuotaExceeded checks the quota based on the byteDiff/keyDiff, and enforcement status
	IsQuotaExceeded(key []byte, byteDiff int, keyDiff int) bool

	// Recover recovers the namespace quota manager
	Recover(b backend.Backend, rv ReadView)

	// Promote promotes the namespace quota manager to be primary
	Promote()

	// Demote demotes the namespace quota manager from being the primary.
	Demote()
}

// namespaceQuotaManager implements NamespaceQuotaManager interface.
type namespaceQuotaManager struct {
	mu sync.RWMutex

	// rv get value sizes over a range
	rv ReadView

	// namespaceQuotaTree stores the map of a key to the Namespace struct
	namespaceQuotaTree QuotaIndex

	// namespaceQuotaEnforcement namespace quota enforcement
	namespaceQuotaEnforcement NamespaceQuotaEnforcement

	// namespaceQuotaExceedMap map of quotas that have exceeded map
	namespaceQuotaExceedMap map[string]string

	// b backend to persist namespace quotas
	// persists the quota and usage along with the ID
	b backend.Backend

	// demotec is set when the namespace quota manager is the primary.
	// demotec will be closed if the namespace quota manager is demoted.
	demotec chan struct{}

	// lg zap logger
	lg *zap.Logger
}

func (nqm *namespaceQuotaManager) SetReadView(rv ReadView) {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()
	nqm.rv = rv
}

func (nqm *namespaceQuotaManager) SetNamespaceQuota(key []byte, quotaByteCount uint64, quotaKeyCount uint64) (*NamespaceQuota, error) {
	// Step 1: Check if the quota exist already, if so this is an update, else a create
	treeItem := nqm.namespaceQuotaTree.Get(key)

	namespaceQuota := &NamespaceQuota{Key: key}

	// Step 2a: If quota already exist, just update the newly set fields
	if treeItem != nil {
		if nqm.lg != nil {
			nqm.lg.Debug("updating a quota, current quota values",
				zap.ByteString("key", treeItem.Key),
				zap.Uint64("quota byte count", treeItem.QuotaByteCount),
				zap.Uint64("quota key count", treeItem.QuotaKeyCount),
				zap.Uint64("usage byte count", treeItem.UsageByteCount),
				zap.Uint64("usage key count", treeItem.UsageKeyCount))
		}

		// Populate the existing fields that can't change in a set operation
		namespaceQuota.UsageByteCount = treeItem.UsageByteCount
		namespaceQuota.UsageKeyCount = treeItem.UsageKeyCount

		// 0 means the quota is not set by the user via the client.
		// So if any of the incoming fields are 0, just pull them from the existing quota
		// If the user is setting the quota to 0, then they should simply delete the quota, since at that point
		// the namespace owner wouldn't be able to add anything.
		// The user can set:
		// 1. Byte Quota Count
		if quotaByteCount == 0 {
			quotaByteCount = treeItem.QuotaByteCount
		}

		// 2. Key Quota Count
		if quotaKeyCount == 0 {
			quotaKeyCount = treeItem.QuotaKeyCount
		}
	} else {
		// 2b. value is empty, create a new quota
		if nqm.lg != nil {
			nqm.lg.Debug("creating new quota")
		}

		// If either of the quotas are absent, default them to having a very large value
		if quotaByteCount == 0 {
			quotaByteCount = math.MaxUint64
		}
		if quotaKeyCount == 0 {
			quotaKeyCount = math.MaxUint64
		}

		// Fetch the size of the values stored, and add them up
		// The range value size will fetch everything across the range and return the list of values
		keys, valueSizes := nqm.rv.RangeValueSize(key, GetPrefixRangeEnd(key))

		// walk through all the values and sum them
		// add the size of each key to the sum
		var sum uint64
		for i, value := range valueSizes {
			sum += uint64(value)
			sum += uint64(len(string(keys[i])))
		}

		// Fill up the missing fields
		namespaceQuota.UsageByteCount = sum
		// TODO: Does the key quota count really need to be uint64?
		namespaceQuota.UsageKeyCount = uint64(len(valueSizes))
	}

	namespaceQuota.QuotaByteCount = quotaByteCount
	namespaceQuota.QuotaKeyCount = quotaKeyCount

	if namespaceQuota.QuotaByteCount < namespaceQuota.UsageByteCount || namespaceQuota.QuotaKeyCount < namespaceQuota.UsageKeyCount {
		if nqm.lg != nil {
			nqm.lg.Warn("quota exceeded at the time of setting quota", zap.ByteString("key", namespaceQuota.Key))
		}
		nqm.namespaceQuotaExceedMap[string(namespaceQuota.Key)] = string(namespaceQuota.Key)
	}

	// override the existing quota with updated values
	_, isUpdated := nqm.namespaceQuotaTree.Put(namespaceQuota)
	if nqm.lg != nil {
		nqm.lg.Debug("new quota values",
			zap.ByteString("key", namespaceQuota.Key),
			zap.Uint64("quota byte count", namespaceQuota.QuotaByteCount),
			zap.Uint64("quota key count", namespaceQuota.QuotaKeyCount),
			zap.Uint64("usage byte count", namespaceQuota.UsageByteCount),
			zap.Uint64("usage key count", namespaceQuota.UsageKeyCount))
	}

	nqm.mu.Lock()
	namespaceQuota.persistTo(nqm.b)
	nqm.mu.Unlock()
	if isUpdated {
		// TODO: update with a namespace quota updated metric instead of create
	} else {
		namespaceQuotaCreatedCounter.Inc()
	}
	return namespaceQuota, nil
}

func (nqm *namespaceQuotaManager) GetNamespaceQuota(key []byte) (*NamespaceQuota, error) {
	// Check if quota exist, else return error
	currQuota := nqm.namespaceQuotaTree.Get(key)
	if currQuota != nil {
		return currQuota, nil
	}
	return nil, ErrNamespaceQuotaNotFound
}

func (nqm *namespaceQuotaManager) ListNamespaceQuotas() []*NamespaceQuota {
	return nqm.namespaceQuotaTree.GetAllNamespaceQuotas()
}

func (nqm *namespaceQuotaManager) DiffNamespaceQuotaUsage(key, newValue []byte) (byteDiff int, keyDiff int) {
	if nqm.namespaceQuotaEnforcement == DISABLED {
		return 0, 0
	}
	start := time.Now()
	// Step 0: get keys and value sizes
	// a nil response indicates the absence of key in the key tree, and is a new addition to the key tree
	valueSize, isFound := nqm.rv.GetValueSize(key)

	// Step 1a: set defaults as if a new key was supposed to be
	// increment key usage count by 1
	// increment byte usage by len(newValue)
	keyDiff = 1
	byteDiff = len(newValue)

	// Step 1b: If key exist, no key count changes
	// update the byteCount
	if isFound {
		// Step 1b: the key exist, no changes to key count, check diff for value size
		keyDiff = 0
		// remove the current size and add the new incoming valueSize
		byteDiff = len(newValue) - valueSize
		return byteDiff, keyDiff
	}
	// Step 1c: if the key does not exist, a new key will be created
	namespaceQuotaGetDiffValueDurationSec.Observe(time.Since(start).Seconds())
	return byteDiff, keyDiff
}

func (nqm *namespaceQuotaManager) IsQuotaExceeded(key []byte, byteDiff int, keyDiff int) bool {
	if nqm.namespaceQuotaEnforcement == DISABLED {
		return false
	}

	// Step 1: Check quota path for any keys exceeding quotas and return an error if any quotas are exceeded
	exceededQuota := nqm.namespaceQuotaTree.CheckQuotaPath(key, uint64(byteDiff), uint64(keyDiff))
	if exceededQuota == nil {
		return false
	}

	if nqm.namespaceQuotaEnforcement == SOFTMODE {
		if nqm.lg != nil {
			value, ok := nqm.namespaceQuotaExceedMap[string(exceededQuota.Key)]
			// 3a. If key not present and quota exceeded, add to map, and log
			if !ok && (exceededQuota.QuotaByteCount < exceededQuota.UsageByteCount || exceededQuota.QuotaKeyCount < exceededQuota.UsageKeyCount) {
				nqm.lg.Warn("quota exceeded but allowed", zap.ByteString("key", exceededQuota.Key))
				nqm.namespaceQuotaExceedMap[string(exceededQuota.Key)] = string(key)
			} else if ok && (exceededQuota.QuotaByteCount >= exceededQuota.UsageByteCount && exceededQuota.QuotaKeyCount >= exceededQuota.UsageKeyCount) {
				// 3b. If key present and quota reduced, remove from map and log
				nqm.lg.Warn("quota back in limits", zap.ByteString("key", exceededQuota.Key))
				delete(nqm.namespaceQuotaExceedMap, value)
				delete(nqm.namespaceQuotaExceedMap, string(exceededQuota.Key))
			}
		}
		return false
	}

	if nqm.lg != nil {
		nqm.lg.Debug("quota violated", zap.ByteString("key", key))
	}

	return true
}

func (nqm *namespaceQuotaManager) UpdateNamespaceQuotaUsage(key []byte, byteDiff int, keyDiff int) {
	if nqm.namespaceQuotaEnforcement == DISABLED {
		return
	}
	nqm.namespaceQuotaTree.UpdateQuotaPath(key, byteDiff, keyDiff)
}

func (nqm *namespaceQuotaManager) DeleteNamespaceQuota(key []byte) (quota *NamespaceQuota, err error) {
	deletedQuota := nqm.namespaceQuotaTree.Delete(key)
	if deletedQuota != nil {
		namespaceQuotaDeletedCounter.Inc()
		return deletedQuota, nil
	}
	return nil, ErrNamespaceQuotaNotFound
}

func (nqm *namespaceQuotaManager) DeleteRangeUpdateUsage(keys [][]byte, valueSizes []int) {
	if nqm.namespaceQuotaEnforcement == DISABLED {
		return
	}
	// Walk through each key, and decrease the usage for each key
	for i := 0; i < len(keys); i++ {
		nqm.namespaceQuotaTree.UpdateQuotaPath(keys[i], -1*(len(keys[i])+valueSizes[i]), -1)
	}
	nqm.lg.Debug("delete range, updating usage complete")
}

func (nqm *namespaceQuotaManager) Recover(b backend.Backend, rv ReadView) {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()

	nqm.b = b
	nqm.rv = rv
	// TODO: Restore the tree
	nqm.initAndRecover()
}

func (nqm *namespaceQuotaManager) Promote() {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()
	nqm.demotec = make(chan struct{})
}

func (nqm *namespaceQuotaManager) Demote() {
	nqm.mu.Lock()
	defer nqm.mu.Unlock()
	if nqm.demotec != nil {
		close(nqm.demotec)
		nqm.demotec = nil
	}
}

// persistTo persists data to backend
func (nq *NamespaceQuota) persistTo(b backend.Backend) {
	nqpb := namespacequotapb.NamespaceQuota{
		Key:            string(nq.Key),
		QuotaByteCount: nq.QuotaByteCount,
		QuotaKeyCount:  nq.QuotaKeyCount,
		UsageByteCount: nq.UsageByteCount,
		UsageKeyCount:  nq.UsageKeyCount,
	}

	tx := b.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	schema.MustUnsafePutNamespaceQuota(tx, &nqpb)
}

// NewNamespaceQuotaManager returns a new NamespaceQuotaManager
func NewNamespaceQuotaManager(lg *zap.Logger, b backend.Backend, cfg NamespaceQuotaManagerConfig) NamespaceQuotaManager {
	return newNamespaceQuotaManager(lg, b, cfg)
}

// newNamespaceQuotaManager initializes/recovers NamespaceQuotaManager
func newNamespaceQuotaManager(lg *zap.Logger, b backend.Backend, cfg NamespaceQuotaManagerConfig) *namespaceQuotaManager {
	nqm := &namespaceQuotaManager{
		namespaceQuotaTree:        NewQuotaTreeIndex(lg),
		b:                         b,
		lg:                        lg,
		namespaceQuotaExceedMap:   make(map[string]string),
		namespaceQuotaEnforcement: NamespaceQuotaEnforcement(cfg.NamespaceQuotaEnforcement),
	}
	nqm.initAndRecover()
	lg.Warn("namespace quota manager config", zap.Int64("enforcement status", int64(nqm.namespaceQuotaEnforcement)))
	namespaceQuotaEnforcementStatus.Set(float64(nqm.namespaceQuotaEnforcement))
	return nqm
}

// initAndRecover init and recover the NamespaceQuotaManager
func (nqm *namespaceQuotaManager) initAndRecover() {
	tx := nqm.b.BatchTx()
	tx.Lock()

	schema.UnsafeCreateNamespaceQuotaBucket(tx)
	quotas := schema.MustUnsafeGetAllNamespaceQuotas(tx)
	tx.Unlock()
	// TODO: copy vs and do decoding outside tx lock if lock contention becomes an issue.
	for _, q := range quotas {
		nqm.namespaceQuotaTree.Put(&NamespaceQuota{
			Key:            []byte(q.Key),
			QuotaKeyCount:  q.QuotaKeyCount,
			QuotaByteCount: q.QuotaByteCount,
			UsageKeyCount:  q.UsageKeyCount,
			UsageByteCount: q.UsageByteCount,
		})
	}
	nqm.b.ForceCommit()
}

// FakeNamespaceQuotaManager is a fake implementation of NamespaceQuota interface.
// Used for testing only.
type FakeNamespaceQuotaManager struct{}

func (fqm *FakeNamespaceQuotaManager) SetReadView(rv ReadView) {
	return
}

func (fqm *FakeNamespaceQuotaManager) SetNamespaceQuota(key []byte, quotaByteCount uint64, quotaKeyCount uint64) (*NamespaceQuota, error) {
	return nil, nil
}

func (fqm *FakeNamespaceQuotaManager) GetNamespaceQuota(key []byte) (*NamespaceQuota, error) {
	return nil, nil
}

func (fqm *FakeNamespaceQuotaManager) UpdateNamespaceQuotaUsage(key []byte, byteDiff int, keyDiff int) {
	return
}

func (fqm *FakeNamespaceQuotaManager) DeleteRangeUpdateUsage(keys [][]byte, valueSizes []int) {
	return
}

func (fqm *FakeNamespaceQuotaManager) DeleteNamespaceQuota(key []byte) (quota *NamespaceQuota, err error) {
	return
}

func (fqm *FakeNamespaceQuotaManager) ListNamespaceQuotas() []*NamespaceQuota {
	return nil
}

func (fqm *FakeNamespaceQuotaManager) DiffNamespaceQuotaUsage(key, newValue []byte) (int, int) {
	return 0, 0
}

func (fqm *FakeNamespaceQuotaManager) IsQuotaExceeded(key []byte, byteDiff int, keyDiff int) bool {
	return false
}

func (fqm *FakeNamespaceQuotaManager) IsNamespaceQuotaExceeded(key []byte) (bool, error) {
	return false, nil
}

func (fqm *FakeNamespaceQuotaManager) Recover(b backend.Backend, rv ReadView) {
	return
}

func (fqm *FakeNamespaceQuotaManager) Promote() {
	return
}

func (fqm *FakeNamespaceQuotaManager) Demote() {
	return
}

type FakeReadView struct {
	backend.BatchTx
}

func (ftr *FakeReadView) RangeValueSize(key, end []byte) (keys [][]byte, values []int) {
	return nil, nil
}
func (ftr *FakeReadView) GetValueSize(key []byte) (valueSize int, isFound bool) {
	return 1, true
}
func (ftr *FakeReadView) End() { ftr.Unlock() }
