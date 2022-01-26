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

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// MustFetchNotEmptyMetric attempts to fetch given 'metric' from 'member',
// waiting for not-empty value or 'timeout'.
func MustFetchNotEmptyMetric(tb testing.TB, member *integration.Member, metric string, timeout <-chan time.Time) string {
	metricValue := ""
	tick := time.Tick(integration.TickDuration)
	for metricValue == "" {
		tb.Logf("Waiting for metric: %v", metric)
		select {
		case <-timeout:
			tb.Fatalf("Failed to fetch metric %v", metric)
			return ""
		case <-tick:
			var err error
			metricValue, err = member.Metric(metric)
			if err != nil {
				tb.Fatal(err)
			}
		}
	}
	return metricValue
}

// TestV3WatchRestoreSnapshotUnsync tests whether slow follower can restore
// from leader snapshot, and still notify on watchers from an old revision
// that were created in synced watcher group in the first place.
// TODO: fix panic with gRPC proxy "panic: watcher current revision should not exceed current revision"
func TestV3WatchRestoreSnapshotUnsync(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:                   3,
		SnapshotCount:          10,
		SnapshotCatchUpEntries: 5,
	})
	defer clus.Terminate(t)

	// spawn a watcher before shutdown, and put it in synced watcher
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	wStream, errW := integration.ToGRPC(clus.Client(0)).Watch.Watch(ctx)
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
	initialLead := clus.WaitMembersForLeader(t, clus.Members[1:])
	t.Logf("elected lead: %v", clus.Members[initialLead].Server.ID())
	t.Logf("sleeping for 2 seconds")
	time.Sleep(2 * time.Second)
	t.Logf("sleeping for 2 seconds DONE")

	kvc := integration.ToGRPC(clus.Client(1)).KV

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
	// We don't expect leadership change here, just recompute the leader'Server index
	// within clus.Members list.
	lead := clus.WaitLeader(t)

	// Sending is scheduled on fifo 'sched' within EtcdServer::run,
	// so it can start delayed after recovery.
	send := MustFetchNotEmptyMetric(t, clus.Members[lead],
		"etcd_network_snapshot_send_inflights_total",
		time.After(5*time.Second))

	if send != "0" && send != "1" {
		// 0 if already sent, 1 if sending
		t.Fatalf("inflight snapshot snapshot_send_inflights_total expected 0 or 1, got %q", send)
	}

	receives := MustFetchNotEmptyMetric(t, clus.Members[(lead+1)%3],
		"etcd_network_snapshot_receive_inflights_total",
		time.After(5*time.Second))
	if receives != "0" && receives != "1" {
		// 0 if already received, 1 if receiving
		t.Fatalf("inflight snapshot receives expected 0 or 1, got %q", receives)
	}

	t.Logf("sleeping for 2 seconds")
	time.Sleep(2 * time.Second)
	t.Logf("sleeping for 2 seconds DONE")

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
