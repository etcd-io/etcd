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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestRaftLogSnapshotAlwaysExistsClusterOf1(t *testing.T) {
	testRaftLogSnapshotExistsPostStartUp(t, 1)
}

func TestRaftLogSnapshotAlwaysExistsClusterOf3(t *testing.T) {
	testRaftLogSnapshotExistsPostStartUp(t, 3)
}

// testRaftLogSnapshotExistsPostStartUp ensures
// - a non-empty raft log snapshot is present after the server starts up
// - the snapshot index is as expected
// - subsequent snapshots work as they used to
func testRaftLogSnapshotExistsPostStartUp(t *testing.T, size int) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   size,
		SnapshotCount:          100,
		SnapshotCatchUpEntries: 10,
	})
	defer clus.Terminate(t)

	// expect the first snapshot to appear
	//
	// NOTE: When starting a new cluster with N member, each member will
	// apply N ConfChange directly at the beginning, setting the applied index to N.
	expectedSnapIndex := size
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	for _, m := range clus.Members {
		lines, err := m.LogObserver.Expect(ctx, "saved snapshot", 1)
		for _, line := range lines {
			t.Logf("[expected line]: %v", line)
		}

		if err != nil {
			t.Fatalf("failed to expect (log:%s, count:%v): %v", "saved snapshot", 1, err)
		}

		assert.Contains(t, lines[0], fmt.Sprintf("{\"snapshot-index\": %d}", expectedSnapIndex))
	}

	// increase applied index from size to size + 101, to trigger the second snapshot
	expectedSnapIndex = size + 101
	kvc := integration.ToGRPC(clus.RandClient()).KV
	for i := 0; i < expectedSnapIndex; i++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Fatalf("#%d: couldn't put key (%v)", i, err)
		}
	}

	// expect the second snapshot to appear
	ctx2, cancel2 := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel2()
	for _, m := range clus.Members {
		lines, err := m.LogObserver.Expect(ctx2, "saved snapshot", 2)
		for _, line := range lines {
			t.Logf("[expected line]: %v", line)
		}

		if err != nil {
			t.Fatalf("failed to expect (log:%s, count:%v): %v", "saved snapshot", 2, err)
		}

		assert.Contains(t, lines[1], fmt.Sprintf("{\"snapshot-index\": %d}", expectedSnapIndex))
	}

	// expect the third snapshot doesn't appear
	errC := make(chan error)
	ctx3, cancel3 := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel3()
	for _, m := range clus.Members {
		go func() {
			// m.LogObserver.Expect should return a DeadlineExceeded error to confirm there are no more snapshots
			_, err := m.LogObserver.Expect(ctx3, "saved snapshot", 3)
			if !errors.Is(err, context.DeadlineExceeded) {
				errC <- fmt.Errorf("expected a DeadlineExceeded error, got %v,  max snapshots allowed is %d", err, 2)
			}
		}()
	}

	select {
	case err := <-errC:
		t.Fatal(err)
	case <-ctx3.Done():
	}
}
