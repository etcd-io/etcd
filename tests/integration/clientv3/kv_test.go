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

package clientv3test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/api/v3/version"
	clientv3 "go.etcd.io/etcd/client/v3"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestKVPutError(t *testing.T) {
	integration2.BeforeTest(t)

	var (
		maxReqBytes = 1.5 * 1024 * 1024                                // hard coded max in v3_server.go
		quota       = int64(int(maxReqBytes*1.2) + 8*os.Getpagesize()) // make sure we have enough overhead in backend quota. See discussion in #6486.
	)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1, QuotaBackendBytes: quota, ClientMaxCallSendMsgSize: 100 * 1024 * 1024})
	defer clus.Terminate(t)

	kv := clus.RandClient()
	ctx := context.TODO()

	_, err := kv.Put(ctx, "", "bar")
	if !errors.Is(err, rpctypes.ErrEmptyKey) {
		t.Fatalf("expected %v, got %v", rpctypes.ErrEmptyKey, err)
	}

	_, err = kv.Put(ctx, "key", strings.Repeat("a", int(maxReqBytes+100)))
	if !errors.Is(err, rpctypes.ErrRequestTooLarge) {
		t.Fatalf("expected %v, got %v", rpctypes.ErrRequestTooLarge, err)
	}

	_, err = kv.Put(ctx, "foo1", strings.Repeat("a", int(maxReqBytes-50)))
	require.NoError(t, err) // below quota

	time.Sleep(1 * time.Second) // give enough time for commit

	_, err = kv.Put(ctx, "foo2", strings.Repeat("a", int(maxReqBytes-50)))
	if !errors.Is(err, rpctypes.ErrNoSpace) { // over quota
		t.Fatalf("expected %v, got %v", rpctypes.ErrNoSpace, err)
	}
}

func TestKVPutWithLease(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	lapi := clus.RandClient()

	kv := clus.RandClient()
	ctx := context.TODO()

	lease, err := lapi.Grant(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to create lease %v", err)
	}

	key := "hello"
	val := "world"
	if _, err = kv.Put(ctx, key, val, clientv3.WithLease(lease.ID)); err != nil {
		t.Fatalf("couldn't put %q (%v)", key, err)
	}
	resp, err := kv.Get(ctx, key)
	if err != nil {
		t.Fatalf("couldn't get key (%v)", err)
	}
	if len(resp.Kvs) != 1 {
		t.Fatalf("expected 1 key, got %d", len(resp.Kvs))
	}
	if !bytes.Equal([]byte(val), resp.Kvs[0].Value) {
		t.Errorf("val = %s, want %s", val, resp.Kvs[0].Value)
	}
	if lease.ID != clientv3.LeaseID(resp.Kvs[0].Lease) {
		t.Errorf("val = %d, want %d", lease.ID, resp.Kvs[0].Lease)
	}
}

// TestKVPutWithIgnoreValue ensures that Put with WithIgnoreValue does not clobber the old value.
func TestKVPutWithIgnoreValue(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kv := clus.RandClient()

	_, err := kv.Put(context.TODO(), "foo", "", clientv3.WithIgnoreValue())
	if !errors.Is(err, rpctypes.ErrKeyNotFound) {
		t.Fatalf("err expected %v, got %v", rpctypes.ErrKeyNotFound, err)
	}

	_, err = kv.Put(context.TODO(), "foo", "bar")
	require.NoError(t, err)

	_, err = kv.Put(context.TODO(), "foo", "", clientv3.WithIgnoreValue())
	require.NoError(t, err)
	rr, rerr := kv.Get(context.TODO(), "foo")
	require.NoError(t, rerr)
	if len(rr.Kvs) != 1 {
		t.Fatalf("len(rr.Kvs) expected 1, got %d", len(rr.Kvs))
	}
	if !bytes.Equal(rr.Kvs[0].Value, []byte("bar")) {
		t.Fatalf("value expected 'bar', got %q", rr.Kvs[0].Value)
	}
}

// TestKVPutWithIgnoreLease ensures that Put with WithIgnoreLease does not affect the existing lease for the key.
func TestKVPutWithIgnoreLease(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kv := clus.RandClient()

	lapi := clus.RandClient()

	resp, err := lapi.Grant(context.Background(), 10)
	if err != nil {
		t.Errorf("failed to create lease %v", err)
	}

	if _, err = kv.Put(context.TODO(), "zoo", "bar", clientv3.WithIgnoreLease()); !errors.Is(err, rpctypes.ErrKeyNotFound) {
		t.Fatalf("err expected %v, got %v", rpctypes.ErrKeyNotFound, err)
	}

	_, err = kv.Put(context.TODO(), "zoo", "bar", clientv3.WithLease(resp.ID))
	require.NoError(t, err)

	_, err = kv.Put(context.TODO(), "zoo", "bar1", clientv3.WithIgnoreLease())
	require.NoError(t, err)

	rr, rerr := kv.Get(context.TODO(), "zoo")
	require.NoError(t, rerr)
	if len(rr.Kvs) != 1 {
		t.Fatalf("len(rr.Kvs) expected 1, got %d", len(rr.Kvs))
	}
	if rr.Kvs[0].Lease != int64(resp.ID) {
		t.Fatalf("lease expected %v, got %v", resp.ID, rr.Kvs[0].Lease)
	}
}

func TestKVPutWithRequireLeader(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)

	// wait for election timeout, then member[0] will not have a leader.
	var (
		electionTicks = 10
		tickDuration  = 10 * time.Millisecond
	)
	time.Sleep(time.Duration(3*electionTicks) * tickDuration)

	kv := clus.Client(0)
	_, err := kv.Put(clientv3.WithRequireLeader(context.Background()), "foo", "bar")
	if !errors.Is(err, rpctypes.ErrNoLeader) {
		t.Fatal(err)
	}

	cnt, err := clus.Members[0].Metric(
		"etcd_server_client_requests_total",
		`type="unary"`,
		fmt.Sprintf(`client_api_version="%v"`, version.APIVersion),
	)
	require.NoError(t, err)
	cv, err := strconv.ParseInt(cnt, 10, 32)
	require.NoError(t, err)
	if cv < 1 { // >1 when retried
		t.Fatalf("expected at least 1, got %q", cnt)
	}

	// clients may give timeout errors since the members are stopped; take
	// the clients so that terminating the cluster won't complain
	clus.Client(1).Close()
	clus.Client(2).Close()
	clus.TakeClient(1)
	clus.TakeClient(2)
}

func TestKVRange(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clus.RandClient()
	ctx := context.TODO()

	keySet := []string{"a", "b", "c", "c", "c", "foo", "foo/abc", "fop"}
	for i, key := range keySet {
		if _, err := kv.Put(ctx, key, ""); err != nil {
			t.Fatalf("#%d: couldn't put %q (%v)", i, key, err)
		}
	}
	resp, err := kv.Get(ctx, keySet[0])
	if err != nil {
		t.Fatalf("couldn't get key (%v)", err)
	}
	wheader := resp.Header

	tests := []struct {
		begin, end string
		rev        int64
		opts       []clientv3.OpOption

		wantSet []*mvccpb.KeyValue
	}{
		// fetch entire keyspace using WithFromKey
		{
			"\x00", "",
			0,
			[]clientv3.OpOption{clientv3.WithFromKey(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)},

			[]*mvccpb.KeyValue{
				{Key: []byte("a"), Value: nil, CreateRevision: 2, ModRevision: 2, Version: 1},
				{Key: []byte("b"), Value: nil, CreateRevision: 3, ModRevision: 3, Version: 1},
				{Key: []byte("c"), Value: nil, CreateRevision: 4, ModRevision: 6, Version: 3},
				{Key: []byte("foo"), Value: nil, CreateRevision: 7, ModRevision: 7, Version: 1},
				{Key: []byte("foo/abc"), Value: nil, CreateRevision: 8, ModRevision: 8, Version: 1},
				{Key: []byte("fop"), Value: nil, CreateRevision: 9, ModRevision: 9, Version: 1},
			},
		},
	}

	for i, tt := range tests {
		opts := []clientv3.OpOption{clientv3.WithRange(tt.end), clientv3.WithRev(tt.rev)}
		opts = append(opts, tt.opts...)
		resp, err := kv.Get(ctx, tt.begin, opts...)
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

func TestKVGetErrConnClosed(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.Client(0)

	donec := make(chan struct{})
	require.NoError(t, cli.Close())
	clus.TakeClient(0)

	go func() {
		defer close(donec)
		_, err := cli.Get(context.TODO(), "foo")
		if !clientv3.IsConnCanceled(err) {
			t.Errorf("expected %v, got %v", context.Canceled, err)
		}
	}()

	select {
	case <-time.After(integration2.RequestWaitTimeout):
		t.Fatal("kv.Get took too long")
	case <-donec:
	}
}

func TestKVNewAfterClose(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.Client(0)
	clus.TakeClient(0)
	require.NoError(t, cli.Close())

	donec := make(chan struct{})
	go func() {
		_, err := cli.Get(context.TODO(), "foo")
		if !clientv3.IsConnCanceled(err) {
			t.Errorf("expected %v, got %v", context.Canceled, err)
		}
		close(donec)
	}()
	select {
	case <-time.After(integration2.RequestWaitTimeout):
		t.Fatal("kv.Get took too long")
	case <-donec:
	}
}

func TestKVDeleteRange(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clus.RandClient()
	ctx := context.TODO()

	tests := []struct {
		key   string
		opts  []clientv3.OpOption
		wkeys []string
	}{
		// *
		{
			key:   "\x00",
			opts:  []clientv3.OpOption{clientv3.WithFromKey()},
			wkeys: nil,
		},
	}

	for i, tt := range tests {
		keySet := []string{"a", "b", "c", "c/abc", "d"}
		for j, key := range keySet {
			if _, err := kv.Put(ctx, key, ""); err != nil {
				t.Fatalf("#%d: couldn't put %q (%v)", j, key, err)
			}
		}

		_, err := kv.Delete(ctx, tt.key, tt.opts...)
		if err != nil {
			t.Fatalf("#%d: couldn't delete range (%v)", i, err)
		}

		resp, err := kv.Get(ctx, "a", clientv3.WithFromKey())
		if err != nil {
			t.Fatalf("#%d: couldn't get keys (%v)", i, err)
		}
		var keys []string
		for _, kv := range resp.Kvs {
			keys = append(keys, string(kv.Key))
		}
		if !reflect.DeepEqual(tt.wkeys, keys) {
			t.Errorf("#%d: resp.Kvs got %v, expected %v", i, keys, tt.wkeys)
		}
	}
}

func TestKVCompactError(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kv := clus.RandClient()
	ctx := context.TODO()

	for i := 0; i < 5; i++ {
		if _, err := kv.Put(ctx, "foo", "bar"); err != nil {
			t.Fatalf("couldn't put 'foo' (%v)", err)
		}
	}
	_, err := kv.Compact(ctx, 6)
	if err != nil {
		t.Fatalf("couldn't compact 6 (%v)", err)
	}

	_, err = kv.Compact(ctx, 6)
	if !errors.Is(err, rpctypes.ErrCompacted) {
		t.Fatalf("expected %v, got %v", rpctypes.ErrCompacted, err)
	}

	_, err = kv.Compact(ctx, 100)
	if !errors.Is(err, rpctypes.ErrFutureRev) {
		t.Fatalf("expected %v, got %v", rpctypes.ErrFutureRev, err)
	}
}

func TestKVCompact(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	kv := clus.RandClient()
	ctx := context.TODO()

	for i := 0; i < 10; i++ {
		if _, err := kv.Put(ctx, "foo", "bar"); err != nil {
			t.Fatalf("couldn't put 'foo' (%v)", err)
		}
	}

	_, err := kv.Compact(ctx, 7)
	if err != nil {
		t.Fatalf("couldn't compact kv space (%v)", err)
	}
	_, err = kv.Compact(ctx, 7)
	if err == nil || !errors.Is(err, rpctypes.ErrCompacted) {
		t.Fatalf("error got %v, want %v", err, rpctypes.ErrCompacted)
	}

	wcli := clus.RandClient()
	// new watcher could precede receiving the compaction without quorum first
	wcli.Get(ctx, "quorum-get")

	wchan := wcli.Watch(ctx, "foo", clientv3.WithRev(3))

	wr := <-wchan
	if wr.CompactRevision != 7 {
		t.Fatalf("wchan CompactRevision got %v, want 7", wr.CompactRevision)
	}
	if !wr.Canceled {
		t.Fatalf("expected canceled watcher on compacted revision, got %v", wr.Canceled)
	}
	if !errors.Is(wr.Err(), rpctypes.ErrCompacted) {
		t.Fatalf("watch response error expected %v, got %v", rpctypes.ErrCompacted, wr.Err())
	}
	wr, ok := <-wchan
	if ok {
		t.Fatalf("wchan got %v, expected closed", wr)
	}
	if wr.Err() != nil {
		t.Fatalf("watch response error expected nil, got %v", wr.Err())
	}

	_, err = kv.Compact(ctx, 1000)
	if err == nil || !errors.Is(err, rpctypes.ErrFutureRev) {
		t.Fatalf("error got %v, want %v", err, rpctypes.ErrFutureRev)
	}
}

// TestKVGetRetry ensures get will retry on disconnect.
func TestKVGetRetry(t *testing.T) {
	integration2.BeforeTest(t)

	clusterSize := 3
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: clusterSize, UseBridge: true})
	defer clus.Terminate(t)

	// because killing leader and following election
	// could give no other endpoints for client reconnection
	fIdx := (clus.WaitLeader(t) + 1) % clusterSize

	kv := clus.Client(fIdx)
	ctx := context.TODO()

	_, err := kv.Put(ctx, "foo", "bar")
	require.NoError(t, err)

	clus.Members[fIdx].Stop(t)

	donec := make(chan struct{}, 1)
	go func() {
		// Get will fail, but reconnect will trigger
		gresp, gerr := kv.Get(ctx, "foo")
		if gerr != nil {
			t.Error(gerr)
		}
		wkvs := []*mvccpb.KeyValue{
			{
				Key:            []byte("foo"),
				Value:          []byte("bar"),
				CreateRevision: 2,
				ModRevision:    2,
				Version:        1,
			},
		}
		if !reflect.DeepEqual(gresp.Kvs, wkvs) {
			t.Errorf("bad get: got %v, want %v", gresp.Kvs, wkvs)
		}
		donec <- struct{}{}
	}()

	time.Sleep(100 * time.Millisecond)
	clus.Members[fIdx].Restart(t)
	clus.Members[fIdx].WaitOK(t)

	select {
	case <-time.After(20 * time.Second):
		t.Fatalf("timed out waiting for get")
	case <-donec:
	}
}

// TestKVPutFailGetRetry ensures a get will retry following a failed put.
func TestKVPutFailGetRetry(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3, UseBridge: true})
	defer clus.Terminate(t)

	kv := clus.Client(0)
	clus.Members[0].Stop(t)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	_, err := kv.Put(ctx, "foo", "bar")
	if err == nil {
		t.Fatalf("got success on disconnected put, wanted error")
	}

	donec := make(chan struct{}, 1)
	go func() {
		// Get will fail, but reconnect will trigger
		gresp, gerr := kv.Get(context.TODO(), "foo")
		if gerr != nil {
			t.Error(gerr)
		}
		if len(gresp.Kvs) != 0 {
			t.Errorf("bad get kvs: got %+v, want empty", gresp.Kvs)
		}
		donec <- struct{}{}
	}()

	time.Sleep(100 * time.Millisecond)
	clus.Members[0].Restart(t)

	select {
	case <-time.After(20 * time.Second):
		t.Fatalf("timed out waiting for get")
	case <-donec:
	}
}

// TestKVGetCancel tests that a context cancel on a Get terminates as expected.
func TestKVGetCancel(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	oldconn := clus.Client(0).ActiveConnection()
	kv := clus.Client(0)

	ctx, cancel := context.WithCancel(context.TODO())
	cancel()

	resp, err := kv.Get(ctx, "abc")
	if err == nil {
		t.Fatalf("cancel on get response %v, expected context error", resp)
	}
	newconn := clus.Client(0).ActiveConnection()
	if oldconn != newconn {
		t.Fatalf("cancel on get broke client connection")
	}
}

// TestKVGetStoppedServerAndClose ensures closing after a failed Get works.
func TestKVGetStoppedServerAndClose(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.Client(0)
	clus.Members[0].Stop(t)
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	// this Get fails and triggers an asynchronous connection retry
	_, err := cli.Get(ctx, "abc")
	cancel()
	if err != nil && !(IsCanceled(err) || IsClientTimeout(err)) {
		t.Fatal(err)
	}
}

// TestKVPutStoppedServerAndClose ensures closing after a failed Put works.
func TestKVPutStoppedServerAndClose(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.Client(0)
	clus.Members[0].Stop(t)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	// get retries on all errors.
	// so here we use it to eat the potential broken pipe error for the next put.
	// grpc client might see a broken pipe error when we issue the get request before
	// grpc finds out the original connection is down due to the member shutdown.
	_, err := cli.Get(ctx, "abc")
	cancel()
	if err != nil && !(IsCanceled(err) || IsClientTimeout(err)) {
		t.Fatal(err)
	}

	ctx, cancel = context.WithTimeout(context.TODO(), time.Second)
	// this Put fails and triggers an asynchronous connection retry
	_, err = cli.Put(ctx, "abc", "123")
	cancel()
	if err != nil && !(IsCanceled(err) || IsClientTimeout(err) || IsUnavailable(err)) {
		t.Fatal(err)
	}
}

// TestKVPutAtMostOnce ensures that a Put will only occur at most once
// in the presence of network errors.
func TestKVPutAtMostOnce(t *testing.T) {
	integration2.BeforeTest(t)
	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 1, UseBridge: true})
	defer clus.Terminate(t)

	_, err := clus.Client(0).Put(context.TODO(), "k", "1")
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		clus.Members[0].Bridge().DropConnections()
		donec := make(chan struct{})
		go func() {
			defer close(donec)
			for i := 0; i < 10; i++ {
				clus.Members[0].Bridge().DropConnections()
				time.Sleep(5 * time.Millisecond)
			}
		}()
		_, err = clus.Client(0).Put(context.TODO(), "k", "v")
		<-donec
		if err != nil {
			break
		}
	}

	resp, err := clus.Client(0).Get(context.TODO(), "k")
	require.NoError(t, err)
	if resp.Kvs[0].Version > 11 {
		t.Fatalf("expected version <= 10, got %+v", resp.Kvs[0])
	}
}

// TestKVLargeRequests tests various client/server side request limits.
func TestKVLargeRequests(t *testing.T) {
	integration2.BeforeTest(t)
	tests := []struct {
		// make sure that "MaxCallSendMsgSize" < server-side default send/recv limit
		maxRequestBytesServer  uint
		maxCallSendBytesClient int
		maxCallRecvBytesClient int

		valueSize   int
		expectError error
	}{
		{
			maxRequestBytesServer:  256,
			maxCallSendBytesClient: 0,
			maxCallRecvBytesClient: 0,
			valueSize:              1024,
			expectError:            rpctypes.ErrRequestTooLarge,
		},

		// without proper client-side receive size limit
		// "code = ResourceExhausted desc = grpc: received message larger than max (5242929 vs. 4194304)"
		{
			maxRequestBytesServer:  7*1024*1024 + 512*1024,
			maxCallSendBytesClient: 7 * 1024 * 1024,
			maxCallRecvBytesClient: 0,
			valueSize:              5 * 1024 * 1024,
			expectError:            nil,
		},
		{
			maxRequestBytesServer:  10 * 1024 * 1024,
			maxCallSendBytesClient: 100 * 1024 * 1024,
			maxCallRecvBytesClient: 0,
			valueSize:              10 * 1024 * 1024,
			expectError:            rpctypes.ErrRequestTooLarge,
		},
		{
			maxRequestBytesServer:  10 * 1024 * 1024,
			maxCallSendBytesClient: 10 * 1024 * 1024,
			maxCallRecvBytesClient: 0,
			valueSize:              10 * 1024 * 1024,
			expectError:            status.Errorf(codes.ResourceExhausted, "trying to send message larger than max "),
		},
		{
			maxRequestBytesServer:  10 * 1024 * 1024,
			maxCallSendBytesClient: 100 * 1024 * 1024,
			maxCallRecvBytesClient: 0,
			valueSize:              10*1024*1024 + 5,
			expectError:            rpctypes.ErrRequestTooLarge,
		},
		{
			maxRequestBytesServer:  10 * 1024 * 1024,
			maxCallSendBytesClient: 10 * 1024 * 1024,
			maxCallRecvBytesClient: 0,
			valueSize:              10*1024*1024 + 5,
			expectError:            status.Errorf(codes.ResourceExhausted, "trying to send message larger than max "),
		},
	}
	for i, test := range tests {
		clus := integration2.NewCluster(t,
			&integration2.ClusterConfig{
				Size:                     1,
				MaxRequestBytes:          test.maxRequestBytesServer,
				ClientMaxCallSendMsgSize: test.maxCallSendBytesClient,
				ClientMaxCallRecvMsgSize: test.maxCallRecvBytesClient,
			},
		)
		cli := clus.Client(0)
		_, err := cli.Put(context.TODO(), "foo", strings.Repeat("a", test.valueSize))

		var etcdErr rpctypes.EtcdError
		if errors.As(err, &etcdErr) {
			if !errors.Is(err, test.expectError) {
				t.Errorf("#%d: expected %v, got %v", i, test.expectError, err)
			}
		} else if err != nil && !strings.HasPrefix(err.Error(), test.expectError.Error()) {
			t.Errorf("#%d: expected error starting with '%s', got '%s'", i, test.expectError.Error(), err.Error())
		}

		// put request went through, now expects large response back
		if err == nil {
			_, err = cli.Get(context.TODO(), "foo")
			if err != nil {
				t.Errorf("#%d: get expected no error, got %v", i, err)
			}
		}

		clus.Terminate(t)
	}
}

// TestKVForLearner ensures learner member only accepts serializable read request.
func TestKVForLearner(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3, DisableStrictReconfigCheck: true})
	defer clus.Terminate(t)

	// we have to add and launch learner member after initial cluster was created, because
	// bootstrapping a cluster with learner member is not supported.
	clus.AddAndLaunchLearnerMember(t)

	learners, err := clus.GetLearnerMembers()
	if err != nil {
		t.Fatalf("failed to get the learner members in cluster: %v", err)
	}
	if len(learners) != 1 {
		t.Fatalf("added 1 learner to cluster, got %d", len(learners))
	}

	if len(clus.Members) != 4 {
		t.Fatalf("expecting 4 members in cluster after adding the learner member, got %d", len(clus.Members))
	}
	// note:
	// 1. clus.Members[3] is the newly added learner member, which was appended to clus.Members
	// 2. we are using member's grpcAddr instead of clientURLs as the endpoint for clientv3.Config,
	// because the implementation of integration test has diverged from embed/etcd.go.
	learnerEp := clus.Members[3].GRPCURL
	cfg := clientv3.Config{
		Endpoints:   []string{learnerEp},
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	// this client only has endpoint of the learner member
	cli, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatalf("failed to create clientv3: %v", err)
	}
	defer cli.Close()

	// wait until learner member is ready
	<-clus.Members[3].ReadyNotify()

	tests := []struct {
		op   clientv3.Op
		wErr bool
	}{
		{
			op:   clientv3.OpGet("foo", clientv3.WithSerializable()),
			wErr: false,
		},
		{
			op:   clientv3.OpGet("foo"),
			wErr: true,
		},
		{
			op:   clientv3.OpPut("foo", "bar"),
			wErr: true,
		},
		{
			op:   clientv3.OpDelete("foo"),
			wErr: true,
		},
		{
			op:   clientv3.OpTxn([]clientv3.Cmp{clientv3.Compare(clientv3.CreateRevision("foo"), "=", 0)}, nil, nil),
			wErr: true,
		},
	}

	for idx, test := range tests {
		_, err := cli.Do(context.TODO(), test.op)
		if err != nil && !test.wErr {
			t.Errorf("%d: expect no error, got %v", idx, err)
		}
		if err == nil && test.wErr {
			t.Errorf("%d: expect error, got nil", idx)
		}
	}
}

// TestBalancerSupportLearner verifies that balancer's retry and failover mechanism supports cluster with learner member
func TestBalancerSupportLearner(t *testing.T) {
	integration2.BeforeTest(t)

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3, DisableStrictReconfigCheck: true})
	defer clus.Terminate(t)

	// we have to add and launch learner member after initial cluster was created, because
	// bootstrapping a cluster with learner member is not supported.
	clus.AddAndLaunchLearnerMember(t)

	learners, err := clus.GetLearnerMembers()
	if err != nil {
		t.Fatalf("failed to get the learner members in cluster: %v", err)
	}
	if len(learners) != 1 {
		t.Fatalf("added 1 learner to cluster, got %d", len(learners))
	}

	// clus.Members[3] is the newly added learner member, which was appended to clus.Members
	learnerEp := clus.Members[3].GRPCURL
	cfg := clientv3.Config{
		Endpoints:   []string{learnerEp},
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	cli, err := integration2.NewClient(t, cfg)
	if err != nil {
		t.Fatalf("failed to create clientv3: %v", err)
	}
	defer cli.Close()

	// wait until learner member is ready
	<-clus.Members[3].ReadyNotify()

	if _, err = cli.Get(context.Background(), "foo"); err == nil {
		t.Fatalf("expect Get request to learner to fail, got no error")
	}
	t.Logf("Expected: Read from learner error: %v", err)

	eps := []string{learnerEp, clus.Members[0].GRPCURL}
	cli.SetEndpoints(eps...)
	if _, err := cli.Get(context.Background(), "foo"); err != nil {
		t.Errorf("expect no error (balancer should retry when request to learner fails), got error: %v", err)
	}
}
