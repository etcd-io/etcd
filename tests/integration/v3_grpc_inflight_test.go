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
	"context"
	"sync"
	"testing"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestV3MaintenanceDefragmentInflightRange ensures inflight range requests
// does not panic the mvcc backend while defragment is running.
func TestV3MaintenanceDefragmentInflightRange(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	kvc := integration.ToGRPC(cli).KV
	if _, err := kvc.Put(context.Background(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		kvc.Range(ctx, &pb.RangeRequest{Key: []byte("foo")})
	}()

	mvc := integration.ToGRPC(cli).Maintenance
	mvc.Defragment(context.Background(), &pb.DefragmentRequest{})
	cancel()

	<-donec
}

// TestV3KVInflightRangeRequests ensures that inflight requests
// (sent before server shutdown) are gracefully handled by server-side.
// They are either finished or canceled, but never crash the backend.
// See https://github.com/etcd-io/etcd/issues/7322 for more detail.
func TestV3KVInflightRangeRequests(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, UseBridge: true})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	kvc := integration.ToGRPC(cli).KV

	if _, err := kvc.Put(context.Background(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	reqN := 10 // use 500+ for fast machine
	var wg sync.WaitGroup
	wg.Add(reqN)
	for i := 0; i < reqN; i++ {
		go func() {
			defer wg.Done()
			_, err := kvc.Range(ctx, &pb.RangeRequest{Key: []byte("foo"), Serializable: true}, grpc.WaitForReady(true))
			if err != nil {
				errCode := status.Convert(err).Code()
				errDesc := rpctypes.ErrorDesc(err)
				if err != nil && !(errDesc == context.Canceled.Error() || errCode == codes.Canceled || errCode == codes.Unavailable) {
					t.Errorf("inflight request should be canceled with '%v' or code Canceled or Unavailable, got '%v' with code '%s'", context.Canceled.Error(), errDesc, errCode)
				}
			}
		}()
	}

	clus.Members[0].Stop(t)
	cancel()

	wg.Wait()
}
