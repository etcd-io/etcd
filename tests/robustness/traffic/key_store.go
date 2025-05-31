// Copyright 2023 The etcd Authors
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

package traffic

import (
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/robustness/model"
)

// Stores the key pool to use for operations. This allows keys used in Put and Delete operations to be more unique by probabilistically swapping out used keys.
type keyStore struct {
	mu sync.Mutex

	counter        int
	keys           []string
	keyPrefix      string
	latestRevision int64
}

func NewKeyStore(size int, keyPrefix string) *keyStore {
	k := &keyStore{
		keys:      make([]string, size),
		counter:   0,
		keyPrefix: keyPrefix,
	}

	// Fill with default values i.e. key0-key9
	for ; k.counter < len(k.keys); k.counter++ {
		k.keys[k.counter] = fmt.Sprintf("%s%d", k.keyPrefix, k.counter)
	}

	return k
}

func (k *keyStore) GetKey() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	useKey := k.keys[rand.Intn(len(k.keys))]

	return useKey
}

func (k *keyStore) GetKeyForDelete() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	useKeyIndex := rand.Intn(len(k.keys))
	useKey := k.keys[useKeyIndex]

	k.replaceKey(useKeyIndex)

	return useKey
}

func (k *keyStore) GetKeysForMultiTxnOps(ops []model.OperationType) []string {
	k.mu.Lock()
	defer k.mu.Unlock()

	numOps := len(ops)

	if numOps > len(k.keys) {
		panic("GetKeysForMultiTxnOps: number of operations is more than the key pool size")
	}

	keys := make([]string, numOps)

	permutedKeyIndexes := rand.Perm(len(k.keys))
	for i, op := range ops {
		keys[i] = k.keys[permutedKeyIndexes[i]]

		if op == model.DeleteOperation {
			k.replaceKey(permutedKeyIndexes[i])
		}
	}

	return keys
}

func (k *keyStore) GetPrefix() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.keyPrefix
}

// SyncKeys reconciles our local key store with the keys currently in etcd.
//
// SyncKeys resolves this by:
//  1. Getting the list request result of all the keys.
//  2. Adding any keys that exist in etcd but are missing in the key store.
//  3. Maintaining the key store size by removing the higher numbered keys.
//
// Notice that higher numbered keys will eventually be added
// again into the keystore, so it is safe to temporarily remove them from the.
// key store.
func (k *keyStore) SyncKeys(resp *clientv3.GetResponse) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if resp.Header.GetRevision() < k.latestRevision {
		return
	}

	k.latestRevision = resp.Header.GetRevision()

	listKeys := make([]string, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		listKeys[i] = string(kv.Key)
	}

	for _, key := range k.keys {
		if !slices.Contains(listKeys, key) {
			listKeys = append(listKeys, key)
		}
	}

	sort.Slice(listKeys, func(i, j int) bool {
		keyNumI := k.getKeyNum(listKeys[i])
		keyNumJ := k.getKeyNum(listKeys[j])
		return keyNumI < keyNumJ
	})

	k.keys = listKeys[:len(k.keys)]

	lastKeyNum := k.getKeyNum(k.keys[len(k.keys)-1])
	k.counter = lastKeyNum + 1
}

func (k *keyStore) getKeyNum(key string) int {
	if len(key) < len(k.keyPrefix) {
		return 0
	}

	numStr := key[len(k.keyPrefix):]
	num, _ := strconv.Atoi(numStr)
	return num
}

func (k *keyStore) replaceKey(index int) {
	if rand.Intn(100) < 90 {
		k.keys[index] = fmt.Sprintf("%s%d", k.keyPrefix, k.counter)
		k.counter++
	}
}
