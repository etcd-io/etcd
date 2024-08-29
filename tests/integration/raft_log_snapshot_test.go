// Copyright 2024 The etcd Authors
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
	"errors"
	"testing"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestRaftLogSnapshotExistsPostStartUp ensures a non-empty raft log snapshot exists after startup
func TestRaftLogSnapshotExistsPostStartUp(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   1,
		SnapshotCount:          100,
		SnapshotCatchUpEntries: 10,
	})
	defer clus.Terminate(t)

	m := clus.Members[0]

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	_, err := m.LogObserver.Expect(ctx, "saved snapshot", 1)
	if err != nil {
		t.Fatalf("failed to expect (log:%s, count:%v): %v", "saved snapshot", 1, err)
	}

	kvc := integration.ToGRPC(clus.RandClient()).KV

	// In order to trigger another snapshot, we should increase applied index from 1 to 102.
	//
	// NOTE: When starting a new cluster with 1 member, the member will
	// apply 3 ConfChange directly at the beginning, setting the applied index to 4.
	for i := 0; i < 102-4; i++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Fatalf("#%d: couldn't put key (%v)", i, err)
		}
	}

	ctx2, cancel2 := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel2()

	_, err = m.LogObserver.Expect(ctx2, "saved snapshot", 2)
	if err != nil {
		t.Fatalf("failed to expect (log:%s, count:%v): %v", "saved snapshot", 1, err)
	}

	ctx3, cancel3 := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel3()

	// Expect function should return a DeadlineExceeded error to ensure no more snapshots are present
	_, err = m.LogObserver.Expect(ctx3, "saved snapshot", 3)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected error, max snapshots allowed is %d: %v", 2, err)
	}
}
