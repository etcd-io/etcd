// Copyright 2017 The etcd Authors
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
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/pkg/v3/traceutil"
	"go.etcd.io/etcd/server/v3/lease/leasepb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestV3StorageQuotaApply tests the V3 server respects quotas during apply
func TestV3StorageQuotaApply(t *testing.T) {
	integration.BeforeTest(t)
	quotasize := int64(16 * os.Getpagesize())

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 2})
	defer clus.Terminate(t)
	kvc1 := integration.ToGRPC(clus.Client(1)).KV

	// Set a quota on one node
	clus.Members[0].QuotaBackendBytes = quotasize
	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)
	clus.WaitMembersForLeader(t, clus.Members)
	kvc0 := integration.ToGRPC(clus.Client(0)).KV
	waitForRestart(t, kvc0)

	key := []byte("abc")

	// test small put still works
	smallbuf := make([]byte, 1024)
	_, serr := kvc0.Put(t.Context(), &pb.PutRequest{Key: key, Value: smallbuf})
	require.NoError(t, serr)

	// test big put
	bigbuf := make([]byte, quotasize)
	_, err := kvc1.Put(t.Context(), &pb.PutRequest{Key: key, Value: bigbuf})
	require.NoError(t, err)

	// quorum get should work regardless of whether alarm is raised
	_, err = kvc0.Range(t.Context(), &pb.RangeRequest{Key: []byte("foo")})
	require.NoError(t, err)

	// wait until alarm is raised for sure-- poll the alarms
	stopc := time.After(5 * time.Second)
	for {
		req := &pb.AlarmRequest{Action: pb.AlarmRequest_GET}
		resp, aerr := clus.Members[0].Server.Alarm(t.Context(), req)
		require.NoError(t, aerr)
		if len(resp.Alarms) != 0 {
			break
		}
		select {
		case <-stopc:
			t.Fatalf("timed out waiting for alarm")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// txn with non-mutating Ops should go through when NOSPACE alarm is raised
	_, err = kvc0.Txn(t.Context(), &pb.TxnRequest{
		Compare: []*pb.Compare{
			{
				Key:         key,
				Result:      pb.Compare_EQUAL,
				Target:      pb.Compare_CREATE,
				TargetUnion: &pb.Compare_CreateRevision{CreateRevision: 0},
			},
		},
		Success: []*pb.RequestOp{
			{
				Request: &pb.RequestOp_RequestDeleteRange{
					RequestDeleteRange: &pb.DeleteRangeRequest{
						Key: key,
					},
				},
			},
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), integration.RequestWaitTimeout)
	defer cancel()

	// small quota machine should reject put
	_, err = kvc0.Put(ctx, &pb.PutRequest{Key: key, Value: smallbuf})
	require.Errorf(t, err, "past-quota instance should reject put")

	// large quota machine should reject put
	_, err = kvc1.Put(ctx, &pb.PutRequest{Key: key, Value: smallbuf})
	require.Errorf(t, err, "past-quota instance should reject put")

	// reset large quota node to ensure alarm persisted
	clus.Members[1].Stop(t)
	clus.Members[1].Restart(t)
	clus.WaitMembersForLeader(t, clus.Members)

	_, err = kvc1.Put(t.Context(), &pb.PutRequest{Key: key, Value: smallbuf})
	require.Errorf(t, err, "alarmed instance should reject put after reset")
}

// TestV3AlarmDeactivate ensures that space alarms can be deactivated so puts go through.
func TestV3AlarmDeactivate(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	kvc := integration.ToGRPC(clus.RandClient()).KV
	mt := integration.ToGRPC(clus.RandClient()).Maintenance

	alarmReq := &pb.AlarmRequest{
		MemberID: 123,
		Action:   pb.AlarmRequest_ACTIVATE,
		Alarm:    pb.AlarmType_NOSPACE,
	}
	_, err := mt.Alarm(t.Context(), alarmReq)
	require.NoError(t, err)

	key := []byte("abc")
	smallbuf := make([]byte, 512)
	_, err = kvc.Put(t.Context(), &pb.PutRequest{Key: key, Value: smallbuf})
	if err == nil && !eqErrGRPC(err, rpctypes.ErrGRPCNoSpace) {
		t.Fatalf("put got %v, expected %v", err, rpctypes.ErrGRPCNoSpace)
	}

	alarmReq.Action = pb.AlarmRequest_DEACTIVATE
	_, err = mt.Alarm(t.Context(), alarmReq)
	require.NoError(t, err)

	_, err = kvc.Put(t.Context(), &pb.PutRequest{Key: key, Value: smallbuf})
	require.NoError(t, err)
}

func TestV3CorruptAlarm(t *testing.T) {
	integration.BeforeTest(t)
	lg := zaptest.NewLogger(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, UseBridge: true})
	defer clus.Terminate(t)

	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			if _, err := clus.Client(0).Put(t.Context(), "k", "v"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	// Corrupt member 0 by modifying backend offline.
	clus.Members[0].Stop(t)
	fp := filepath.Join(clus.Members[0].DataDir, "member", "snap", "db")
	be := backend.NewDefaultBackend(lg, fp)
	s := mvcc.NewStore(lg, be, nil, mvcc.StoreConfig{})
	// NOTE: cluster_proxy mode with namespacing won't set 'k', but namespace/'k'.
	s.Put([]byte("abc"), []byte("def"), 0)
	s.Put([]byte("xyz"), []byte("123"), 0)
	s.Compact(traceutil.TODO(), 5)
	s.Commit()
	s.Close()
	be.Close()

	clus.Members[1].WaitOK(t)
	clus.Members[2].WaitOK(t)
	time.Sleep(time.Second * 2)

	// Wait for cluster so Puts succeed in case member 0 was the leader.
	_, err := clus.Client(1).Get(t.Context(), "k")
	require.NoError(t, err)
	_, err = clus.Client(1).Put(t.Context(), "xyz", "321")
	require.NoError(t, err)
	_, err = clus.Client(1).Put(t.Context(), "abc", "fed")
	require.NoError(t, err)

	// Restart with corruption checking enabled.
	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)
	for _, m := range clus.Members {
		m.CorruptCheckTime = time.Second
		m.Restart(t)
	}
	clus.WaitLeader(t)
	time.Sleep(time.Second * 2)

	clus.Members[0].WaitStarted(t)
	resp0, err0 := clus.Client(0).Get(t.Context(), "abc")
	require.NoError(t, err0)
	clus.Members[1].WaitStarted(t)
	resp1, err1 := clus.Client(1).Get(t.Context(), "abc")
	require.NoError(t, err1)

	require.NotEqualf(t, resp0.Kvs[0].ModRevision, resp1.Kvs[0].ModRevision, "matching ModRevision values")

	for i := 0; i < 5; i++ {
		presp, perr := clus.Client(0).Put(t.Context(), "abc", "aaa")
		if perr != nil {
			if eqErrGRPC(perr, rpctypes.ErrCorrupt) {
				return
			}
			t.Fatalf("expected %v, got %+v (%v)", rpctypes.ErrCorrupt, presp, perr)
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("expected error %v after %s", rpctypes.ErrCorrupt, 5*time.Second)
}

func TestV3CorruptAlarmWithLeaseCorrupted(t *testing.T) {
	integration.BeforeTest(t)
	lg := zaptest.NewLogger(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		CorruptCheckTime:           time.Second,
		Size:                       3,
		SnapshotCount:              10,
		SnapshotCatchUpEntries:     5,
		DisableStrictReconfigCheck: true,
	})
	defer clus.Terminate(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	lresp, err := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(ctx, &pb.LeaseGrantRequest{ID: 1, TTL: 60})
	if err != nil {
		t.Errorf("could not create lease 1 (%v)", err)
	}
	if lresp.ID != 1 {
		t.Errorf("got id %v, wanted id %v", lresp.ID, 1)
	}

	putr := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar"), Lease: lresp.ID}
	// Trigger snapshot from the leader to new member
	for i := 0; i < 15; i++ {
		_, err = integration.ToGRPC(clus.RandClient()).KV.Put(ctx, putr)
		if err != nil {
			t.Errorf("#%d: couldn't put key (%v)", i, err)
		}
	}

	require.NoError(t, clus.RemoveMember(t, clus.Client(1), uint64(clus.Members[2].ID())))
	clus.WaitMembersForLeader(t, clus.Members)

	clus.AddMember(t)
	clus.WaitMembersForLeader(t, clus.Members)
	// Wait for new member to catch up
	integration.WaitClientV3(t, clus.Members[2].Client)

	// Corrupt member 2 by modifying backend lease bucket offline.
	clus.Members[2].Stop(t)
	fp := filepath.Join(clus.Members[2].DataDir, "member", "snap", "db")
	bcfg := backend.DefaultBackendConfig(lg)
	bcfg.Path = fp
	be := backend.New(bcfg)

	olpb := leasepb.Lease{ID: int64(1), TTL: 60}
	tx := be.BatchTx()
	schema.UnsafeDeleteLease(tx, &olpb)
	lpb := leasepb.Lease{ID: int64(2), TTL: 60}
	schema.MustUnsafePutLease(tx, &lpb)
	tx.Commit()

	require.NoError(t, be.Close())

	require.NoError(t, clus.Members[2].Restart(t))

	clus.Members[1].WaitOK(t)
	clus.Members[2].WaitOK(t)

	// Revoke lease should remove key except the member with corruption
	_, err = integration.ToGRPC(clus.Members[0].Client).Lease.LeaseRevoke(ctx, &pb.LeaseRevokeRequest{ID: lresp.ID})
	require.NoError(t, err)
	resp0, err0 := clus.Members[1].Client.KV.Get(t.Context(), "foo")
	require.NoError(t, err0)
	resp1, err1 := clus.Members[2].Client.KV.Get(t.Context(), "foo")
	require.NoError(t, err1)

	require.NotEqualf(t, resp0.Header.Revision, resp1.Header.Revision, "matching Revision values")

	// Wait for CorruptCheckTime
	time.Sleep(time.Second)
	presp, perr := clus.Client(0).Put(t.Context(), "abc", "aaa")
	if perr != nil {
		if eqErrGRPC(perr, rpctypes.ErrCorrupt) {
			return
		}
		t.Fatalf("expected %v, got %+v (%v)", rpctypes.ErrCorrupt, presp, perr)
	}
}
