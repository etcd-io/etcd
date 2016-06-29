// Copyright 2016 The etcd Authors
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
	"reflect"
	"testing"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/testutil"
	"golang.org/x/net/context"
)

// TestV3PutOverwrite puts a key with the v3 api to a random cluster member,
// overwrites it, then checks that the change was applied.
func TestV3PutOverwrite(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.RandClient()).KV
	key := []byte("foo")
	reqput := &pb.PutRequest{Key: key, Value: []byte("bar")}

	respput, err := kvc.Put(context.TODO(), reqput)
	if err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}

	// overwrite
	reqput.Value = []byte("baz")
	respput2, err := kvc.Put(context.TODO(), reqput)
	if err != nil {
		t.Fatalf("couldn't put key (%v)", err)
	}
	if respput2.Header.Revision <= respput.Header.Revision {
		t.Fatalf("expected newer revision on overwrite, got %v <= %v",
			respput2.Header.Revision, respput.Header.Revision)
	}

	reqrange := &pb.RangeRequest{Key: key}
	resprange, err := kvc.Range(context.TODO(), reqrange)
	if err != nil {
		t.Fatalf("couldn't get key (%v)", err)
	}
	if len(resprange.Kvs) != 1 {
		t.Fatalf("expected 1 key, got %v", len(resprange.Kvs))
	}

	kv := resprange.Kvs[0]
	if kv.ModRevision <= kv.CreateRevision {
		t.Errorf("expected modRev > createRev, got %d <= %d",
			kv.ModRevision, kv.CreateRevision)
	}
	if !reflect.DeepEqual(reqput.Value, kv.Value) {
		t.Errorf("expected value %v, got %v", reqput.Value, kv.Value)
	}
}
