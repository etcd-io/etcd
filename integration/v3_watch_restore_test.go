// Copyright 2018 The etcd Authors
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
	"fmt"
	"testing"
	"time"

	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
)

// TestV3WatchRestoreSnapshotUnsync tests whether slow follower can restore
// from leader snapshot, and still notify on watchers from an old revision
// that were created in synced watcher group in the first place.
// TODO: fix panic with gRPC proxy "panic: watcher current revision should not exceed current revision"
func TestV3WatchRestoreSnapshotUnsync(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{
		Size:                   3,
		SnapshotCount:          10,
		SnapshotCatchUpEntries: 5,
	})
	defer clus.Terminate(t)

	// spawn a watcher before shutdown, and put it in synced watcher
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	wStream, errW := toGRPC(clus.Client(0)).Watch.Watch(ctx)
	if errW != nil {
		t.Fatal(errW)
	}
	if err := wStream.Send(&pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
		CreateRequest: &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: 5}}}); err != nil {
		t.Fatalf("wStream.Send error: %v", err)
	}
	wresp, errR := wStream.Recv()
	if errR != nil {
		t.Errorf("wStream.Recv error: %v", errR)
	}
	if !wresp.Created {
		t.Errorf("wresp.Created got = %v, want = true", wresp.Created)
	}

	clus.Members[0].InjectPartition(t, clus.Members[1:]...)
	clus.waitLeader(t, clus.Members[1:])
	time.Sleep(2 * time.Second)

	kvc := toGRPC(clus.Client(1)).KV

	// to trigger snapshot from the leader to the stopped follower
	for i := 0; i < 15; i++ {
		_, err := kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", i, err)
		}
	}

	// trigger snapshot send from leader to this slow follower
	// which then calls watchable store Restore
	clus.Members[0].RecoverPartition(t, clus.Members[1:]...)
	lead := clus.WaitLeader(t)

	sends, err := clus.Members[lead].Metric("etcd_network_snapshot_send_inflights_total")
	if err != nil {
		t.Fatal(err)
	}
	if sends != "0" && sends != "1" {
		// 0 if already sent, 1 if sending
		t.Fatalf("inflight snapshot sends expected 0 or 1, got %q", sends)
	}
	receives, err := clus.Members[(lead+1)%3].Metric("etcd_network_snapshot_receive_inflights_total")
	if err != nil {
		t.Fatal(err)
	}
	if receives != "0" && receives != "1" {
		// 0 if already received, 1 if receiving
		t.Fatalf("inflight snapshot receives expected 0 or 1, got %q", receives)
	}

	time.Sleep(2 * time.Second)

	// slow follower now applies leader snapshot
	// should be able to notify on old-revision watchers in unsynced
	// make sure restore watch operation correctly moves watchers
	// between synced and unsynced watchers
	errc := make(chan error)
	go func() {
		cresp, cerr := wStream.Recv()
		if cerr != nil {
			errc <- cerr
			return
		}
		// from start revision 5 to latest revision 16
		if len(cresp.Events) != 12 {
			errc <- fmt.Errorf("expected 12 events, got %+v", cresp.Events)
			return
		}
		errc <- nil
	}()
	select {
	case <-time.After(10 * time.Second):
		t.Fatal("took too long to receive events from restored watcher")
	case err := <-errc:
		if err != nil {
			t.Fatalf("wStream.Recv error: %v", err)
		}
	}
}
