// Copyright 2016 CoreOS, Inc.
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

package integration

import (
	"bytes"
	"testing"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/pkg/testutil"
)

func TestKVPut(t *testing.T) {
	defer testutil.AfterTest(t)

	tests := []struct {
		key, val string
		leaseID  lease.LeaseID
	}{
		{"foo", "bar", lease.NoLease},

		// TODO: test with leaseID
	}

	for i, tt := range tests {
		clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
		defer clus.Terminate(t)

		kv := clientv3.NewKV(clus.RandClient())

		if _, err := kv.Put(tt.key, tt.val, tt.leaseID); err != nil {
			t.Fatalf("#%d: couldn't put %q (%v)", i, tt.key, err)
		}

		resp, err := kv.Get(tt.key, 0)
		if err != nil {
			t.Fatalf("#%d: couldn't get key (%v)", i, err)
		}
		if len(resp.Kvs) != 1 {
			t.Fatalf("#%d: expected 1 key, got %d", i, len(resp.Kvs))
		}
		if !bytes.Equal([]byte(tt.val), resp.Kvs[0].Value) {
			t.Errorf("#%d: val = %s, want %s", i, tt.val, resp.Kvs[0].Value)
		}
		if tt.leaseID != lease.LeaseID(resp.Kvs[0].Lease) {
			t.Errorf("#%d: val = %d, want %d", i, tt.leaseID, resp.Kvs[0].Lease)
		}
	}
}
