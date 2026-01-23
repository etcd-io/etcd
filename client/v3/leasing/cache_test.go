// Copyright 2017 The etcd Authors
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

package leasing

import (
	"fmt"
	"sync"
	"testing"
	"time"

	v3 "go.etcd.io/etcd/client/v3"
)

// TestLockWriteOpsRace tests for data race in LockWriteOps when called
// concurrently with other cache operations.
// This test should be run with -race flag to detect the race condition.
func TestLockWriteOpsRace(t *testing.T) {
	lc := &leaseCache{
		entries: make(map[string]*leaseKey),
		revokes: make(map[string]time.Time),
	}

	// Add some entries to the cache
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		lc.entries[key] = &leaseKey{
			response: &v3.GetResponse{},
			rev:      int64(i),
			waitc:    closedCh,
		}
	}

	var wg sync.WaitGroup
	// Number of goroutines for each operation
	numGoroutines := 100

	// Create ops for LockWriteOps
	ops := []v3.Op{
		v3.OpPut("key1", "value1"),
		v3.OpPut("key2", "value2"),
		v3.OpDelete("key3"),
	}

	// Start goroutines that call LockWriteOps concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = lc.LockWriteOps(ops)
		}()
	}

	// Start goroutines that call Lock concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", idx%10)
			_, _ = lc.Lock(key)
		}(i)
	}

	// Start goroutines that call Rev concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", idx%10)
			_ = lc.Rev(key)
		}(i)
	}

	wg.Wait()
}
