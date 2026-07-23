// Copyright 2026 The etcd Authors
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
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	v3pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	v3 "go.etcd.io/etcd/client/v3"
)

// TestLeaseCacheLockWriteOpsRangeConcurrency exercises the range branch of
// LockWriteOps concurrently with Add and Evict to surface data races on
// lc.entries.
func TestLeaseCacheLockWriteOpsRangeConcurrency(t *testing.T) {
	lc := &leaseCache{
		entries: make(map[string]*leaseKey),
		revokes: make(map[string]time.Time),
	}

	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("k/%d", i)
		lc.Add(key, getResponseForKey(key, int64(i+2)), v3.OpGet(key))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			wcs := lc.LockWriteOps([]v3.Op{
				v3.OpDelete("k/", v3.WithRange("k0")),
			})
			closeAll(wcs)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; ctx.Err() == nil; i++ {
			key := fmt.Sprintf("k/%d", i%16)
			lc.Add(key, getResponseForKey(key, int64(i+2)), v3.OpGet(key))
			if i%2 == 0 {
				lc.Evict(key)
			}
		}
	}()
	wg.Wait()
}

func getResponseForKey(key string, rev int64) *v3.GetResponse {
	return &v3.GetResponse{
		Header: &v3pb.ResponseHeader{Revision: rev},
		Kvs: []*mvccpb.KeyValue{
			{
				Key:            []byte(key),
				Value:          []byte("value"),
				CreateRevision: rev,
				ModRevision:    rev,
				Version:        1,
			},
		},
		Count: 1,
	}
}
