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
	"fmt"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/etcdserver/api/v3rpc/rpctypes"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"
	"go.etcd.io/etcd/v3/pkg/testutil"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestV3LeasePrmote ensures the newly elected leader can promote itself
// to the primary lessor, refresh the leases and start to manage leases.
// TODO: use customized clock to make this test go faster?
func TestV3LeasePrmote(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// create lease
	lresp, err := toGRPC(clus.RandClient()).Lease.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: 3})
	ttl := time.Duration(lresp.TTL) * time.Second
	afterGrant := time.Now()
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}

	// wait until the lease is going to expire.
	time.Sleep(time.Until(afterGrant.Add(ttl - time.Second)))

	// kill the current leader, all leases should be refreshed.
	toStop := clus.waitLeader(t, clus.Members)
	beforeStop := time.Now()
	clus.Members[toStop].Stop(t)

	var toWait []*member
	for i, m := range clus.Members {
		if i != toStop {
			toWait = append(toWait, m)
		}
	}
	clus.waitLeader(t, toWait)
	clus.Members[toStop].Restart(t)
	clus.waitLeader(t, clus.Members)
	afterReelect := time.Now()

	// ensure lease is refreshed by waiting for a "long" time.
	// it was going to expire anyway.
	time.Sleep(time.Until(beforeStop.Add(ttl - time.Second)))

	if !leaseExist(t, clus, lresp.ID) {
		t.Error("unexpected lease not exists")
	}

	// wait until the renewed lease is expected to expire.
	time.Sleep(time.Until(afterReelect.Add(ttl)))

	// wait for up to 10 seconds for lease to expire.
	expiredCondition := func() (bool, error) {
		return !leaseExist(t, clus, lresp.ID), nil
	}
	expired, err := testutil.Poll(100*time.Millisecond, 10*time.Second, expiredCondition)
	if err != nil {
		t.Error(err)
	}

	if !expired {
		t.Error("unexpected lease exists")
	}
}

// TestV3LeaseRevoke ensures a key is deleted once its lease is revoked.
func TestV3LeaseRevoke(t *testing.T) {
	defer testutil.AfterTest(t)
	testLeaseRemoveLeasedKey(t, func(clus *ClusterV3, leaseID int64) error {
		lc := toGRPC(clus.RandClient()).Lease
		_, err := lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: leaseID})
		return err
	})
}

// TestV3LeaseGrantById ensures leases may be created by a given id.
func TestV3LeaseGrantByID(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// create fixed lease
	lresp, err := toGRPC(clus.RandClient()).Lease.LeaseGrant(
		context.TODO(),
		&pb.LeaseGrantRequest{ID: 1, TTL: 1})
	if err != nil {
		t.Errorf("could not create lease 1 (%v)", err)
	}
	if lresp.ID != 1 {
		t.Errorf("got id %v, wanted id %v", lresp.ID, 1)
	}

	// create duplicate fixed lease
	_, err = toGRPC(clus.RandClient()).Lease.LeaseGrant(
		context.TODO(),
		&pb.LeaseGrantRequest{ID: 1, TTL: 1})
	if !eqErrGRPC(err, rpctypes.ErrGRPCLeaseExist) {
		t.Error(err)
	}

	// create fresh fixed lease
	lresp, err = toGRPC(clus.RandClient()).Lease.LeaseGrant(
		context.TODO(),
		&pb.LeaseGrantRequest{ID: 2, TTL: 1})
	if err != nil {
		t.Errorf("could not create lease 2 (%v)", err)
	}
	if lresp.ID != 2 {
		t.Errorf("got id %v, wanted id %v", lresp.ID, 2)
	}
}

// TestV3LeaseExpire ensures a key is deleted once a key expires.
func TestV3LeaseExpire(t *testing.T) {
	defer testutil.AfterTest(t)
	testLeaseRemoveLeasedKey(t, func(clus *ClusterV3, leaseID int64) error {
		// let lease lapse; wait for deleted key

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wStream, err := toGRPC(clus.RandClient()).Watch.Watch(ctx)
		if err != nil {
			return err
		}

		wreq := &pb.WatchRequest{RequestUnion: &pb.WatchRequest_CreateRequest{
			CreateRequest: &pb.WatchCreateRequest{
				Key: []byte("foo"), StartRevision: 1}}}
		if err := wStream.Send(wreq); err != nil {
			return err
		}
		if _, err := wStream.Recv(); err != nil {
			// the 'created' message
			return err
		}
		if _, err := wStream.Recv(); err != nil {
			// the 'put' message
			return err
		}

		errc := make(chan error, 1)
		go func() {
			resp, err := wStream.Recv()
			switch {
			case err != nil:
				errc <- err
			case len(resp.Events) != 1:
				fallthrough
			case resp.Events[0].Type != mvccpb.DELETE:
				errc <- fmt.Errorf("expected key delete, got %v", resp)
			default:
				errc <- nil
			}
		}()

		select {
		case <-time.After(15 * time.Second):
			return fmt.Errorf("lease expiration too slow")
		case err := <-errc:
			return err
		}
	})
}

// TestV3LeaseKeepAlive ensures keepalive keeps the lease alive.
func TestV3LeaseKeepAlive(t *testing.T) {
	defer testutil.AfterTest(t)
	testLeaseRemoveLeasedKey(t, func(clus *ClusterV3, leaseID int64) error {
		lc := toGRPC(clus.RandClient()).Lease
		lreq := &pb.LeaseKeepAliveRequest{ID: leaseID}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		lac, err := lc.LeaseKeepAlive(ctx)
		if err != nil {
			return err
		}
		defer lac.CloseSend()

		// renew long enough so lease would've expired otherwise
		for i := 0; i < 3; i++ {
			if err = lac.Send(lreq); err != nil {
				return err
			}
			lresp, rxerr := lac.Recv()
			if rxerr != nil {
				return rxerr
			}
			if lresp.ID != leaseID {
				return fmt.Errorf("expected lease ID %v, got %v", leaseID, lresp.ID)
			}
			time.Sleep(time.Duration(lresp.TTL/2) * time.Second)
		}
		_, err = lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: leaseID})
		return err
	})
}

// TestV3LeaseCheckpoint ensures a lease checkpoint results in a remaining TTL being persisted
// across leader elections.
func TestV3LeaseCheckpoint(t *testing.T) {
	var ttl int64 = 300
	leaseInterval := 2 * time.Second
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{
		Size:                    3,
		EnableLeaseCheckpoint:   true,
		LeaseCheckpointInterval: leaseInterval,
	})
	defer clus.Terminate(t)

	// create lease
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := toGRPC(clus.RandClient())
	lresp, err := c.Lease.LeaseGrant(ctx, &pb.LeaseGrantRequest{TTL: ttl})
	if err != nil {
		t.Fatal(err)
	}

	// wait for a checkpoint to occur
	time.Sleep(leaseInterval + 1*time.Second)

	// Force a leader election
	leaderId := clus.WaitLeader(t)
	leader := clus.Members[leaderId]
	leader.Stop(t)
	time.Sleep(time.Duration(3*electionTicks) * tickDuration)
	leader.Restart(t)
	newLeaderId := clus.WaitLeader(t)
	c2 := toGRPC(clus.Client(newLeaderId))

	time.Sleep(250 * time.Millisecond)

	// Check the TTL of the new leader
	var ttlresp *pb.LeaseTimeToLiveResponse
	for i := 0; i < 10; i++ {
		if ttlresp, err = c2.Lease.LeaseTimeToLive(ctx, &pb.LeaseTimeToLiveRequest{ID: lresp.ID}); err != nil {
			if status, ok := status.FromError(err); ok && status.Code() == codes.Unavailable {
				time.Sleep(time.Millisecond * 250)
			} else {
				t.Fatal(err)
			}
		}
	}

	expectedTTL := ttl - int64(leaseInterval.Seconds())
	if ttlresp.TTL < expectedTTL-1 || ttlresp.TTL > expectedTTL {
		t.Fatalf("expected lease to be checkpointed after restart such that %d < TTL <%d, but got TTL=%d", expectedTTL-1, expectedTTL, ttlresp.TTL)
	}
}

// TestV3LeaseExists creates a lease on a random client and confirms it exists in the cluster.
func TestV3LeaseExists(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	// create lease
	ctx0, cancel0 := context.WithCancel(context.Background())
	defer cancel0()
	lresp, err := toGRPC(clus.RandClient()).Lease.LeaseGrant(
		ctx0,
		&pb.LeaseGrantRequest{TTL: 30})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}

	if !leaseExist(t, clus, lresp.ID) {
		t.Error("unexpected lease not exists")
	}
}

// TestV3LeaseLeases creates leases and confirms list RPC fetches created ones.
func TestV3LeaseLeases(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx0, cancel0 := context.WithCancel(context.Background())
	defer cancel0()

	// create leases
	ids := []int64{}
	for i := 0; i < 5; i++ {
		lresp, err := toGRPC(clus.RandClient()).Lease.LeaseGrant(
			ctx0,
			&pb.LeaseGrantRequest{TTL: 30})
		if err != nil {
			t.Fatal(err)
		}
		if lresp.Error != "" {
			t.Fatal(lresp.Error)
		}
		ids = append(ids, lresp.ID)
	}

	lresp, err := toGRPC(clus.RandClient()).Lease.LeaseLeases(
		context.Background(),
		&pb.LeaseLeasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range lresp.Leases {
		if lresp.Leases[i].ID != ids[i] {
			t.Fatalf("#%d: lease ID expected %d, got %d", i, ids[i], lresp.Leases[i].ID)
		}
	}
}

// TestV3LeaseRenewStress keeps creating lease and renewing it immediately to ensure the renewal goes through.
// it was oberserved that the immediate lease renewal after granting a lease from follower resulted lease not found.
// related issue https://github.com/etcd-io/etcd/issues/6978
func TestV3LeaseRenewStress(t *testing.T) {
	testLeaseStress(t, stressLeaseRenew)
}

// TestV3LeaseTimeToLiveStress keeps creating lease and retrieving it immediately to ensure the lease can be retrieved.
// it was oberserved that the immediate lease retrieval after granting a lease from follower resulted lease not found.
// related issue https://github.com/etcd-io/etcd/issues/6978
func TestV3LeaseTimeToLiveStress(t *testing.T) {
	testLeaseStress(t, stressLeaseTimeToLive)
}

func testLeaseStress(t *testing.T, stresser func(context.Context, pb.LeaseClient) error) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	errc := make(chan error)

	for i := 0; i < 30; i++ {
		for j := 0; j < 3; j++ {
			go func(i int) { errc <- stresser(ctx, toGRPC(clus.Client(i)).Lease) }(j)
		}
	}

	for i := 0; i < 90; i++ {
		if err := <-errc; err != nil {
			t.Fatal(err)
		}
	}
}

func stressLeaseRenew(tctx context.Context, lc pb.LeaseClient) (reterr error) {
	defer func() {
		if tctx.Err() != nil {
			reterr = nil
		}
	}()
	lac, err := lc.LeaseKeepAlive(tctx)
	if err != nil {
		return err
	}
	for tctx.Err() == nil {
		resp, gerr := lc.LeaseGrant(tctx, &pb.LeaseGrantRequest{TTL: 60})
		if gerr != nil {
			continue
		}
		err = lac.Send(&pb.LeaseKeepAliveRequest{ID: resp.ID})
		if err != nil {
			continue
		}
		rresp, rxerr := lac.Recv()
		if rxerr != nil {
			continue
		}
		if rresp.TTL == 0 {
			return fmt.Errorf("TTL shouldn't be 0 so soon")
		}
	}
	return nil
}

func stressLeaseTimeToLive(tctx context.Context, lc pb.LeaseClient) (reterr error) {
	defer func() {
		if tctx.Err() != nil {
			reterr = nil
		}
	}()
	for tctx.Err() == nil {
		resp, gerr := lc.LeaseGrant(tctx, &pb.LeaseGrantRequest{TTL: 60})
		if gerr != nil {
			continue
		}
		_, kerr := lc.LeaseTimeToLive(tctx, &pb.LeaseTimeToLiveRequest{ID: resp.ID})
		if rpctypes.Error(kerr) == rpctypes.ErrLeaseNotFound {
			return kerr
		}
	}
	return nil
}

func TestV3PutOnNonExistLease(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	badLeaseID := int64(0x12345678)
	putr := &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar"), Lease: badLeaseID}
	_, err := toGRPC(clus.RandClient()).KV.Put(ctx, putr)
	if !eqErrGRPC(err, rpctypes.ErrGRPCLeaseNotFound) {
		t.Errorf("err = %v, want %v", err, rpctypes.ErrGRPCLeaseNotFound)
	}
}

// TestV3GetNonExistLease ensures client retrieving nonexistent lease on a follower doesn't result node panic
// related issue https://github.com/etcd-io/etcd/issues/6537
func TestV3GetNonExistLease(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lc := toGRPC(clus.RandClient()).Lease
	lresp, err := lc.LeaseGrant(ctx, &pb.LeaseGrantRequest{TTL: 10})
	if err != nil {
		t.Errorf("failed to create lease %v", err)
	}
	_, err = lc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}

	leaseTTLr := &pb.LeaseTimeToLiveRequest{
		ID:   lresp.ID,
		Keys: true,
	}

	for _, client := range clus.clients {
		// quorum-read to ensure revoke completes before TimeToLive
		if _, err := toGRPC(client).KV.Range(ctx, &pb.RangeRequest{Key: []byte("_")}); err != nil {
			t.Fatal(err)
		}
		resp, err := toGRPC(client).Lease.LeaseTimeToLive(ctx, leaseTTLr)
		if err != nil {
			t.Fatalf("expected non nil error, but go %v", err)
		}
		if resp.TTL != -1 {
			t.Fatalf("expected TTL to be -1, but got %v", resp.TTL)
		}
	}
}

// TestV3LeaseSwitch tests a key can be switched from one lease to another.
func TestV3LeaseSwitch(t *testing.T) {
	defer testutil.AfterTest(t)
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	key := "foo"

	// create lease
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lresp1, err1 := toGRPC(clus.RandClient()).Lease.LeaseGrant(ctx, &pb.LeaseGrantRequest{TTL: 30})
	if err1 != nil {
		t.Fatal(err1)
	}
	lresp2, err2 := toGRPC(clus.RandClient()).Lease.LeaseGrant(ctx, &pb.LeaseGrantRequest{TTL: 30})
	if err2 != nil {
		t.Fatal(err2)
	}

	// attach key on lease1 then switch it to lease2
	put1 := &pb.PutRequest{Key: []byte(key), Lease: lresp1.ID}
	_, err := toGRPC(clus.RandClient()).KV.Put(ctx, put1)
	if err != nil {
		t.Fatal(err)
	}
	put2 := &pb.PutRequest{Key: []byte(key), Lease: lresp2.ID}
	_, err = toGRPC(clus.RandClient()).KV.Put(ctx, put2)
	if err != nil {
		t.Fatal(err)
	}

	// revoke lease1 should not remove key
	_, err = toGRPC(clus.RandClient()).Lease.LeaseRevoke(ctx, &pb.LeaseRevokeRequest{ID: lresp1.ID})
	if err != nil {
		t.Fatal(err)
	}
	rreq := &pb.RangeRequest{Key: []byte("foo")}
	rresp, err := toGRPC(clus.RandClient()).KV.Range(context.TODO(), rreq)
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 1 {
		t.Fatalf("unexpect removal of key")
	}

	// revoke lease2 should remove key
	_, err = toGRPC(clus.RandClient()).Lease.LeaseRevoke(ctx, &pb.LeaseRevokeRequest{ID: lresp2.ID})
	if err != nil {
		t.Fatal(err)
	}
	rresp, err = toGRPC(clus.RandClient()).KV.Range(context.TODO(), rreq)
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 0 {
		t.Fatalf("lease removed but key remains")
	}
}

// TestV3LeaseFailover ensures the old leader drops lease keepalive requests within
// election timeout after it loses its quorum. And the new leader extends the TTL of
// the lease to at least TTL + election timeout.
func TestV3LeaseFailover(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	toIsolate := clus.waitLeader(t, clus.Members)

	lc := toGRPC(clus.Client(toIsolate)).Lease

	// create lease
	lresp, err := lc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: 5})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}

	// isolate the current leader with its followers.
	clus.Members[toIsolate].Pause()

	lreq := &pb.LeaseKeepAliveRequest{ID: lresp.ID}

	md := metadata.Pairs(rpctypes.MetadataRequireLeaderKey, rpctypes.MetadataHasLeader)
	mctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithCancel(mctx)
	defer cancel()
	lac, err := lc.LeaseKeepAlive(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// send keep alive to old leader until the old leader starts
	// to drop lease request.
	var expectedExp time.Time
	for {
		if err = lac.Send(lreq); err != nil {
			break
		}
		lkresp, rxerr := lac.Recv()
		if rxerr != nil {
			break
		}
		expectedExp = time.Now().Add(time.Duration(lkresp.TTL) * time.Second)
		time.Sleep(time.Duration(lkresp.TTL/2) * time.Second)
	}

	clus.Members[toIsolate].Resume()
	clus.waitLeader(t, clus.Members)

	// lease should not expire at the last received expire deadline.
	time.Sleep(time.Until(expectedExp) - 500*time.Millisecond)

	if !leaseExist(t, clus, lresp.ID) {
		t.Error("unexpected lease not exists")
	}
}

// TestV3LeaseRequireLeader ensures that a Recv will get a leader
// loss error if there is no leader.
func TestV3LeaseRequireLeader(t *testing.T) {
	defer testutil.AfterTest(t)

	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	lc := toGRPC(clus.Client(0)).Lease
	clus.Members[1].Stop(t)
	clus.Members[2].Stop(t)

	md := metadata.Pairs(rpctypes.MetadataRequireLeaderKey, rpctypes.MetadataHasLeader)
	mctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithCancel(mctx)
	defer cancel()
	lac, err := lc.LeaseKeepAlive(ctx)
	if err != nil {
		t.Fatal(err)
	}

	donec := make(chan struct{})
	go func() {
		defer close(donec)
		resp, err := lac.Recv()
		if err == nil {
			t.Errorf("got response %+v, expected error", resp)
		}
		if rpctypes.ErrorDesc(err) != rpctypes.ErrNoLeader.Error() {
			t.Errorf("err = %v, want %v", err, rpctypes.ErrNoLeader)
		}
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive leader loss error (in 5-sec)")
	case <-donec:
	}
}

const fiveMinTTL int64 = 300

// TestV3LeaseRecoverAndRevoke ensures that revoking a lease after restart deletes the attached key.
func TestV3LeaseRecoverAndRevoke(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.Client(0)).KV
	lsc := toGRPC(clus.Client(0)).Lease

	lresp, err := lsc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: fiveMinTTL})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}
	_, err = kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar"), Lease: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}

	// restart server and ensure lease still exists
	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)
	clus.waitLeader(t, clus.Members)

	// overwrite old client with newly dialed connection
	// otherwise, error with "grpc: RPC failed fast due to transport failure"
	nc, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatal(err)
	}
	kvc = toGRPC(nc).KV
	lsc = toGRPC(nc).Lease
	defer nc.Close()

	// revoke should delete the key
	_, err = lsc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}
	rresp, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: []byte("foo")})
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 0 {
		t.Fatalf("lease removed but key remains")
	}
}

// TestV3LeaseRevokeAndRecover ensures that revoked key stays deleted after restart.
func TestV3LeaseRevokeAndRecover(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.Client(0)).KV
	lsc := toGRPC(clus.Client(0)).Lease

	lresp, err := lsc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: fiveMinTTL})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}
	_, err = kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar"), Lease: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}

	// revoke should delete the key
	_, err = lsc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}

	// restart server and ensure revoked key doesn't exist
	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)
	clus.waitLeader(t, clus.Members)

	// overwrite old client with newly dialed connection
	// otherwise, error with "grpc: RPC failed fast due to transport failure"
	nc, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatal(err)
	}
	kvc = toGRPC(nc).KV
	defer nc.Close()

	rresp, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: []byte("foo")})
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 0 {
		t.Fatalf("lease removed but key remains")
	}
}

// TestV3LeaseRecoverKeyWithDetachedLease ensures that revoking a detached lease after restart
// does not delete the key.
func TestV3LeaseRecoverKeyWithDetachedLease(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.Client(0)).KV
	lsc := toGRPC(clus.Client(0)).Lease

	lresp, err := lsc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: fiveMinTTL})
	if err != nil {
		t.Fatal(err)
	}
	if lresp.Error != "" {
		t.Fatal(lresp.Error)
	}
	_, err = kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar"), Lease: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}

	// overwrite lease with none
	_, err = kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	if err != nil {
		t.Fatal(err)
	}

	// restart server and ensure lease still exists
	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)
	clus.waitLeader(t, clus.Members)

	// overwrite old client with newly dialed connection
	// otherwise, error with "grpc: RPC failed fast due to transport failure"
	nc, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatal(err)
	}
	kvc = toGRPC(nc).KV
	lsc = toGRPC(nc).Lease
	defer nc.Close()

	// revoke the detached lease
	_, err = lsc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: lresp.ID})
	if err != nil {
		t.Fatal(err)
	}
	rresp, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: []byte("foo")})
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 1 {
		t.Fatalf("only detached lease removed, key should remain")
	}
}

func TestV3LeaseRecoverKeyWithMutipleLease(t *testing.T) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	kvc := toGRPC(clus.Client(0)).KV
	lsc := toGRPC(clus.Client(0)).Lease

	var leaseIDs []int64
	for i := 0; i < 2; i++ {
		lresp, err := lsc.LeaseGrant(context.TODO(), &pb.LeaseGrantRequest{TTL: fiveMinTTL})
		if err != nil {
			t.Fatal(err)
		}
		if lresp.Error != "" {
			t.Fatal(lresp.Error)
		}
		leaseIDs = append(leaseIDs, lresp.ID)

		_, err = kvc.Put(context.TODO(), &pb.PutRequest{Key: []byte("foo"), Value: []byte("bar"), Lease: lresp.ID})
		if err != nil {
			t.Fatal(err)
		}
	}

	// restart server and ensure lease still exists
	clus.Members[0].Stop(t)
	clus.Members[0].Restart(t)
	clus.waitLeader(t, clus.Members)
	for i, leaseID := range leaseIDs {
		if !leaseExist(t, clus, leaseID) {
			t.Errorf("#%d: unexpected lease not exists", i)
		}
	}

	// overwrite old client with newly dialed connection
	// otherwise, error with "grpc: RPC failed fast due to transport failure"
	nc, err := NewClientV3(clus.Members[0])
	if err != nil {
		t.Fatal(err)
	}
	kvc = toGRPC(nc).KV
	lsc = toGRPC(nc).Lease
	defer nc.Close()

	// revoke the old lease
	_, err = lsc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: leaseIDs[0]})
	if err != nil {
		t.Fatal(err)
	}
	// key should still exist
	rresp, err := kvc.Range(context.TODO(), &pb.RangeRequest{Key: []byte("foo")})
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 1 {
		t.Fatalf("only detached lease removed, key should remain")
	}

	// revoke the latest lease
	_, err = lsc.LeaseRevoke(context.TODO(), &pb.LeaseRevokeRequest{ID: leaseIDs[1]})
	if err != nil {
		t.Fatal(err)
	}
	rresp, err = kvc.Range(context.TODO(), &pb.RangeRequest{Key: []byte("foo")})
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 0 {
		t.Fatalf("lease removed but key remains")
	}
}

// acquireLeaseAndKey creates a new lease and creates an attached key.
func acquireLeaseAndKey(clus *ClusterV3, key string) (int64, error) {
	// create lease
	lresp, err := toGRPC(clus.RandClient()).Lease.LeaseGrant(
		context.TODO(),
		&pb.LeaseGrantRequest{TTL: 1})
	if err != nil {
		return 0, err
	}
	if lresp.Error != "" {
		return 0, fmt.Errorf(lresp.Error)
	}
	// attach to key
	put := &pb.PutRequest{Key: []byte(key), Lease: lresp.ID}
	if _, err := toGRPC(clus.RandClient()).KV.Put(context.TODO(), put); err != nil {
		return 0, err
	}
	return lresp.ID, nil
}

// testLeaseRemoveLeasedKey performs some action while holding a lease with an
// attached key "foo", then confirms the key is gone.
func testLeaseRemoveLeasedKey(t *testing.T, act func(*ClusterV3, int64) error) {
	clus := NewClusterV3(t, &ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	leaseID, err := acquireLeaseAndKey(clus, "foo")
	if err != nil {
		t.Fatal(err)
	}

	if err = act(clus, leaseID); err != nil {
		t.Fatal(err)
	}

	// confirm no key
	rreq := &pb.RangeRequest{Key: []byte("foo")}
	rresp, err := toGRPC(clus.RandClient()).KV.Range(context.TODO(), rreq)
	if err != nil {
		t.Fatal(err)
	}
	if len(rresp.Kvs) != 0 {
		t.Fatalf("lease removed but key remains")
	}
}

func leaseExist(t *testing.T, clus *ClusterV3, leaseID int64) bool {
	l := toGRPC(clus.RandClient()).Lease

	_, err := l.LeaseGrant(context.Background(), &pb.LeaseGrantRequest{ID: leaseID, TTL: 5})
	if err == nil {
		_, err = l.LeaseRevoke(context.Background(), &pb.LeaseRevokeRequest{ID: leaseID})
		if err != nil {
			t.Fatalf("failed to check lease %v", err)
		}
		return false
	}

	if eqErrGRPC(err, rpctypes.ErrGRPCLeaseExist) {
		return true
	}
	t.Fatalf("unexpecter error %v", err)

	return true
}
