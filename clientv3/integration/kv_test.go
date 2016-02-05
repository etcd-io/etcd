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
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver/api/v3rpc"
	"github.com/coreos/etcd/integration"
	"github.com/coreos/etcd/lease"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/storage/storagepb"
)

func TestKVPut(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	lapi := clientv3.NewLease(clus.RandClient())
	defer lapi.Close()

	kv := clientv3.NewKV(clus.RandClient())

	resp, err := lapi.Create(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to create lease %v", err)
	}

	tests := []struct {
		key, val string
		leaseID  lease.LeaseID
	}{
		{"foo", "bar", lease.NoLease},
		{"hello", "world", lease.LeaseID(resp.ID)},
	}

	for i, tt := range tests {
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

func TestKVRange(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clientv3.NewKV(clus.RandClient())

	keySet := []string{"a", "b", "c", "c", "c", "foo", "foo/abc", "fop"}
	for i, key := range keySet {
		if _, err := kv.Put(key, "", lease.NoLease); err != nil {
			t.Fatalf("#%d: couldn't put %q (%v)", i, key, err)
		}
	}
	resp, err := kv.Get(keySet[0], 0)
	if err != nil {
		t.Fatalf("couldn't get key (%v)", err)
	}
	wheader := resp.Header

	tests := []struct {
		begin, end string
		rev        int64
		sortOption *clientv3.SortOption

		wantSet []*storagepb.KeyValue
	}{
		// range first two
		{
			"a", "c",
			0,
			nil,

			[]*storagepb.KeyValue{
				{Key: []byte("a"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
				{Key: []byte("b"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
			},
		},
		// range all with rev
		{
			"a", "x",
			2,
			nil,

			[]*storagepb.KeyValue{
				{Key: []byte("a"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
			},
		},
		// range all with SortByKey, SortAscend
		{
			"a", "x",
			0,
			&clientv3.SortOption{Target: clientv3.SortByKey, Order: clientv3.SortAscend},

			[]*storagepb.KeyValue{
				{Key: []byte("a"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
				{Key: []byte("b"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
				{Key: []byte("c"), Value: nil, CreateRevision: 4, ModRevision: 6, Version: 3},
				{Key: []byte("foo"), Value: nil, CreateRevision: 7, ModRevision: 7, Version: 1},
				{Key: []byte("foo/abc"), Value: nil, CreateRevision: 8, ModRevision: 8, Version: 1},
				{Key: []byte("fop"), Value: nil, CreateRevision: 9, ModRevision: 9, Version: 1},
			},
		},
		// range all with SortByCreatedRev, SortDescend
		{
			"a", "x",
			0,
			&clientv3.SortOption{Target: clientv3.SortByCreatedRev, Order: clientv3.SortDescend},

			[]*storagepb.KeyValue{
				{Key: []byte("fop"), Value: nil, CreateRevision: 9, ModRevision: 9, Version: 1},
				{Key: []byte("foo/abc"), Value: nil, CreateRevision: 8, ModRevision: 8, Version: 1},
				{Key: []byte("foo"), Value: nil, CreateRevision: 7, ModRevision: 7, Version: 1},
				{Key: []byte("c"), Value: nil, CreateRevision: 4, ModRevision: 6, Version: 3},
				{Key: []byte("b"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
				{Key: []byte("a"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
			},
		},
		// range all with SortByModifiedRev, SortDescend
		{
			"a", "x",
			0,
			&clientv3.SortOption{Target: clientv3.SortByModifiedRev, Order: clientv3.SortDescend},

			[]*storagepb.KeyValue{
				{Key: []byte("fop"), Value: nil, CreateRevision: 9, ModRevision: 9, Version: 1},
				{Key: []byte("foo/abc"), Value: nil, CreateRevision: 8, ModRevision: 8, Version: 1},
				{Key: []byte("foo"), Value: nil, CreateRevision: 7, ModRevision: 7, Version: 1},
				{Key: []byte("c"), Value: nil, CreateRevision: 4, ModRevision: 6, Version: 3},
				{Key: []byte("b"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
				{Key: []byte("a"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
			},
		},
	}

	for i, tt := range tests {
		resp, err := kv.Range(tt.begin, tt.end, 0, tt.rev, tt.sortOption)
		if err != nil {
			t.Fatalf("#%d: couldn't range (%v)", i, err)
		}
		if !reflect.DeepEqual(wheader, resp.Header) {
			t.Fatalf("#%d: wheader expected %+v, got %+v", i, wheader, resp.Header)
		}
		if !reflect.DeepEqual(tt.wantSet, resp.Kvs) {
			t.Fatalf("#%d: resp.Kvs expected %+v, got %+v", i, tt.wantSet, resp.Kvs)
		}
	}
}

func TestKVDeleteRange(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clientv3.NewKV(clus.RandClient())

	keySet := []string{"a", "b", "c", "c", "c", "d", "e", "f"}
	for i, key := range keySet {
		if _, err := kv.Put(key, "", lease.NoLease); err != nil {
			t.Fatalf("#%d: couldn't put %q (%v)", i, key, err)
		}
	}

	tests := []struct {
		key, end string
		delRev   int64
	}{
		{"a", "b", int64(len(keySet) + 2)}, // delete [a, b)
		{"d", "f", int64(len(keySet) + 3)}, // delete [d, f)
	}

	for i, tt := range tests {
		dresp, err := kv.DeleteRange(tt.key, tt.end)
		if err != nil {
			t.Fatalf("#%d: couldn't delete range (%v)", i, err)
		}
		if dresp.Header.Revision != tt.delRev {
			t.Fatalf("#%d: dresp.Header.Revision got %d, want %d", i, dresp.Header.Revision, tt.delRev)
		}
		resp, err := kv.Range(tt.key, tt.end, 0, 0, nil)
		if err != nil {
			t.Fatalf("#%d: couldn't get key (%v)", i, err)
		}
		if len(resp.Kvs) > 0 {
			t.Fatalf("#%d: resp.Kvs expected none, but got %+v", i, resp.Kvs)
		}
	}
}

func TestKVDelete(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clientv3.NewKV(clus.RandClient())

	presp, err := kv.Put("foo", "", lease.NoLease)
	if err != nil {
		t.Fatalf("couldn't put 'foo' (%v)", err)
	}
	if presp.Header.Revision != 2 {
		t.Fatalf("presp.Header.Revision got %d, want %d", presp.Header.Revision, 2)
	}
	resp, err := kv.Delete("foo")
	if err != nil {
		t.Fatalf("couldn't delete key (%v)", err)
	}
	if resp.Header.Revision != 3 {
		t.Fatalf("resp.Header.Revision got %d, want %d", resp.Header.Revision, 3)
	}
	gresp, err := kv.Get("foo", 0)
	if err != nil {
		t.Fatalf("couldn't get key (%v)", err)
	}
	if len(gresp.Kvs) > 0 {
		t.Fatalf("gresp.Kvs got %+v, want none", gresp.Kvs)
	}
}

func TestKVCompact(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clientv3.NewKV(clus.RandClient())

	for i := 0; i < 10; i++ {
		if _, err := kv.Put("foo", "bar", lease.NoLease); err != nil {
			t.Fatalf("couldn't put 'foo' (%v)", err)
		}
	}

	err := kv.Compact(7)
	if err != nil {
		t.Fatalf("couldn't compact kv space (%v)", err)
	}
	err = kv.Compact(7)
	if err == nil || err != v3rpc.ErrCompacted {
		t.Fatalf("error got %v, want %v", err, v3rpc.ErrFutureRev)
	}

	wc := clientv3.NewWatcher(clus.RandClient())
	defer wc.Close()
	wchan := wc.Watch(context.TODO(), "foo", 3)

	_, ok := <-wchan
	if ok {
		t.Fatalf("wchan ok got %v, want false", ok)
	}

	err = kv.Compact(1000)
	if err == nil || err != v3rpc.ErrFutureRev {
		t.Fatalf("error got %v, want %v", err, v3rpc.ErrFutureRev)
	}
}

// TestKVGetRetry ensures get will retry on disconnect.
func TestKVGetRetry(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clientv3.NewKV(clus.Client(0))

	if _, err := kv.Put("foo", "bar", 0); err != nil {
		t.Fatal(err)
	}

	clus.Members[0].Stop(t)
	<-clus.Members[0].StopNotify()

	donec := make(chan struct{})
	go func() {
		// Get will fail, but reconnect will trigger
		gresp, gerr := kv.Get("foo", 0)
		if gerr != nil {
			t.Fatal(gerr)
		}
		wkvs := []*storagepb.KeyValue{
			{
				Key:            []byte("foo"),
				Value:          []byte("bar"),
				CreateRevision: 2,
				ModRevision:    2,
				Version:        1,
			},
		}
		if !reflect.DeepEqual(gresp.Kvs, wkvs) {
			t.Fatalf("bad get: got %v, want %v", gresp.Kvs, wkvs)
		}
		donec <- struct{}{}
	}()

	time.Sleep(100 * time.Millisecond)
	clus.Members[0].Restart(t)

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for get")
	case <-donec:
	}
}

// TestKVPutFailGetRetry ensures a get will retry following a failed put.
func TestKVPutFailGetRetry(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clientv3.NewKV(clus.Client(0))
	clus.Members[0].Stop(t)
	<-clus.Members[0].StopNotify()

	_, err := kv.Put("foo", "bar", 0)
	if err == nil {
		t.Fatalf("got success on disconnected put, wanted error")
	}

	donec := make(chan struct{})
	go func() {
		// Get will fail, but reconnect will trigger
		gresp, gerr := kv.Get("foo", 0)
		if gerr != nil {
			t.Fatal(gerr)
		}
		if len(gresp.Kvs) != 0 {
			t.Fatalf("bad get kvs: got %+v, want empty", gresp.Kvs)
		}
		donec <- struct{}{}
	}()

	time.Sleep(100 * time.Millisecond)
	clus.Members[0].Restart(t)

	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for get")
	case <-donec:
	}
}
