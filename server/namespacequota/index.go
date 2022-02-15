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
	"bytes"
	iradix "github.com/hashicorp/go-immutable-radix"
	"go.uber.org/zap"
	"time"
)

// QuotaIndex quota index defines the interface to perform any quota tree operations.
type QuotaIndex interface {
	// Get get the key from quota tree
	Get(key []byte) (quota *NamespaceQuota)
	// GetAllNamespaceQuotas creates and returns a map of all NamespaceQuota in the tree
	GetAllNamespaceQuotas() (quotas []*NamespaceQuota)
	// GetParentQuotas gets all the parents for the given key
	GetParentQuotas(key []byte) (quotas []*NamespaceQuota)
	// Put insert a namespace quota to the tree, returns if updated
	Put(quota *NamespaceQuota) (updatedQuota *NamespaceQuota, isUpdated bool)
	// Delete deletes a namespace quota to the tree
	Delete(key []byte) (quota *NamespaceQuota)
	// CheckQuotaPath check the tree path for any key exceeding quota
	CheckQuotaPath(key []byte, valueDiff uint64, keyDiff uint64) (quotas *NamespaceQuota)
	// UpdateQuotaPath update the quota
	UpdateQuotaPath(key []byte, valueDiff int, keyDiff int)
}

// quotaTreeIndex represents a quota tree
type quotaTreeIndex struct {
	tree *iradix.Tree
	lg   *zap.Logger
}

// NewQuotaTreeIndex creates a new quota tree
func NewQuotaTreeIndex(lg *zap.Logger) QuotaIndex {
	return &quotaTreeIndex{
		tree: iradix.New(),
		lg:   lg,
	}
}

func (q *quotaTreeIndex) Get(key []byte) (quota *NamespaceQuota) {
	if item, isFound := q.tree.Get(key); isFound {
		return item.(*NamespaceQuota)
	}
	return nil
}

func (q *quotaTreeIndex) GetAllNamespaceQuotas() (quotas []*NamespaceQuota) {
	start, _, _ := q.tree.Root().Minimum()
	end, _, _ := q.tree.Root().Maximum()
	it := q.tree.Root().Iterator()
	it.SeekLowerBound(start)
	for key, val, ok := it.Next(); ok; key, val, ok = it.Next() {
		if bytes.Equal(key, end) {
			quotas = append(quotas, val.(*NamespaceQuota))
			break
		}
		quotas = append(quotas, val.(*NamespaceQuota))
	}
	return quotas
}

func (q *quotaTreeIndex) Put(quota *NamespaceQuota) (updatedQuota *NamespaceQuota, isUpdated bool) {
	tree, item, _ := q.tree.Insert(quota.Key, quota)
	q.tree = tree
	if item == nil {
		return nil, isUpdated
	}
	updatedQuota = item.(*NamespaceQuota)
	return item.(*NamespaceQuota), isUpdated
}

func (q *quotaTreeIndex) Delete(key []byte) (quota *NamespaceQuota) {
	newTree, item, isDeleted := q.tree.Delete(key)
	if isDeleted {
		q.tree = newTree
		return item.(*NamespaceQuota)
	}
	return nil
}

func (q *quotaTreeIndex) CheckQuotaPath(key []byte, byteDiff uint64, keyDiff uint64) (quotas *NamespaceQuota) {
	var currQuota *NamespaceQuota
	// Use the delta and see if quota is violated at any level in the quota branch.
	// eg: If the key being PUT is /foo/bar/baz/far, quota manager will walk through each key in that quota branch
	// like first check if /foo quota is violated, if not, then /foo/bar is checked, and /foo/bar/baz, so on and
	// so forth. The first violation is returned, there maybe more violations deeper in the branch but quota manager
	// will return the first violation itself. Byte quota violations will be returned if they exist before key quota
	// violation
	f := iradix.WalkFn(func(key []byte, value interface{}) bool {
		// Step 1: If the quota exist, add the byteDiff/keyDiff and see if it exceeds quota
		if value != nil {
			item := value.(*NamespaceQuota)
			// 1a: If byte quota is violated
			if item.UsageByteCount+byteDiff > item.QuotaByteCount {
				currQuota = item
				// return true is to exit the tree traversal and immediately fail
				return true
			}
			namespaceQuotaByteQuotaExceededTotal.Inc()
			// 1b: If key quota is violated
			if item.UsageKeyCount+keyDiff > item.QuotaKeyCount {
				currQuota = item
				// return true is to exit the tree traversal and immediately fail
				return true
			}
			namespaceQuotaKeyQuotaExceededTotal.Inc()
		}
		return false
	})
	q.tree.Root().WalkPath(key, f)
	// this will be nil if no quota was violated
	return currQuota
}

func (q *quotaTreeIndex) GetParentQuotas(key []byte) (quotas []*NamespaceQuota) {
	f := iradix.WalkFn(func(key []byte, value interface{}) bool {
		if value != nil {
			item := value.(*NamespaceQuota)
			quotas = append(quotas, item)
		}
		return false
	})
	q.tree.Root().WalkPath(key, f)
	return quotas
}

func (q *quotaTreeIndex) UpdateQuotaPath(key []byte, byteDiff int, keyDiff int) {
	f := iradix.WalkFn(func(key []byte, value interface{}) bool {
		// Step 1: If the quota exist, add/subtract the byteDiff/keyDiff
		if value != nil {
			item := value.(*NamespaceQuota)
			if byteDiff > 0 {
				item.UsageByteCount = item.UsageByteCount + uint64(byteDiff)
			} else {
				item.UsageByteCount = item.UsageByteCount - uint64(-1*byteDiff)
			}
			namespaceQuotaByteUsage.WithLabelValues(string(item.Key)).Set(float64(item.UsageByteCount))
			if keyDiff > 0 {
				item.UsageKeyCount = item.UsageKeyCount + uint64(keyDiff)
			} else {
				item.UsageKeyCount = item.UsageKeyCount - uint64(-1*keyDiff)
			}
			namespaceQuotaKeyUsage.WithLabelValues(string(item.Key)).Set(float64(item.UsageKeyCount))
		}
		return false
	})
	start := time.Now()
	q.tree.Root().WalkPath(key, f)
	namespaceQuotaTreeUpdateDurationSec.Observe(time.Since(start).Seconds())
}
