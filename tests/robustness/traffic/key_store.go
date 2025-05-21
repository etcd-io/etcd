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

// For delete operations we want to make sure that we are not deleting the same key over and over again. So we will probabilistically swap out the key for a new one.
func (k *keyStore) GetKeyForDelete() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	useKeyIndex := rand.Intn(len(k.keys))
	useKey := k.keys[useKeyIndex]

	if rand.Intn(100) < 90 {
		k.keys[useKeyIndex] = fmt.Sprintf("%s%d", k.keyPrefix, k.counter)
		k.counter++
	}

	return useKey
}

func (k *keyStore) GetKey() string {
	k.mu.Lock()
	defer k.mu.Unlock()

	useKey := k.keys[rand.Intn(len(k.keys))]

	return useKey
}
