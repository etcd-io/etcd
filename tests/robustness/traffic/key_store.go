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
	"sync"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

// Stores the key pool to use for operations. This allows keys used in Put and Delete operations to be more unique by probabilistically swapping out used keys.
type keyStore struct {
	mu sync.Mutex

	counter   int
	keys      []string
	keyPrefix string
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

func (k *keyStore) replaceKey(index int) {
	if rand.Intn(100) < 90 {
		k.keys[index] = fmt.Sprintf("%s%d", k.keyPrefix, k.counter)
		k.counter++
	}
}
