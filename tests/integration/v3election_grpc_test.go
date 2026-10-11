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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	epb "go.etcd.io/etcd/server/v3/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestV3ElectionCampaign checks that Campaign will not give
// simultaneous leadership to multiple campaigners.
func TestV3ElectionCampaign(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	lease1, err1 := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(t.Context(), &pb.LeaseGrantRequest{TTL: 30})
	require.NoError(t, err1)
	lease2, err2 := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(t.Context(), &pb.LeaseGrantRequest{TTL: 30})
	require.NoError(t, err2)

	lc := integration.ToGRPC(clus.Client(0)).Election
	req1 := &epb.CampaignRequest{Name: []byte("foo"), Lease: lease1.ID, Value: []byte("abc")}
	l1, lerr1 := lc.Campaign(t.Context(), req1)
	require.NoError(t, lerr1)

	campaignc := make(chan struct{})
	go func() {
		defer close(campaignc)
		req2 := &epb.CampaignRequest{Name: []byte("foo"), Lease: lease2.ID, Value: []byte("def")}
		l2, lerr2 := lc.Campaign(t.Context(), req2)
		if lerr2 != nil {
			t.Error(lerr2)
		}
		if l1.Header.Revision >= l2.Header.Revision {
			t.Errorf("expected l1 revision < l2 revision, got %d >= %d", l1.Header.Revision, l2.Header.Revision)
		}
	}()

	select {
	case <-time.After(200 * time.Millisecond):
	case <-campaignc:
		t.Fatalf("got leadership before resign")
	}

	_, uerr := lc.Resign(t.Context(), &epb.ResignRequest{Leader: l1.Leader})
	require.NoError(t, uerr)

	select {
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("campaigner unelected after resign")
	case <-campaignc:
	}

	lval, lverr := lc.Leader(t.Context(), &epb.LeaderRequest{Name: []byte("foo")})
	require.NoError(t, lverr)

	if string(lval.Kv.Value) != "def" {
		t.Fatalf("got election value %q, expected %q", string(lval.Kv.Value), "def")
	}
}

// TestV3ElectionObserve checks that an Observe stream receives
// proclamations from different leaders uninterrupted.
func TestV3ElectionObserve(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	lc := integration.ToGRPC(clus.Client(0)).Election

	// observe leadership events
	observec := make(chan struct{}, 1)
	go func() {
		defer close(observec)
		s, err := lc.Observe(t.Context(), &epb.LeaderRequest{Name: []byte("foo")})
		observec <- struct{}{}
		if err != nil {
			t.Error(err)
		}
		for i := 0; i < 10; i++ {
			resp, rerr := s.Recv()
			if rerr != nil {
				t.Error(rerr)
			}
			respV := 0
			fmt.Sscanf(string(resp.Kv.Value), "%d", &respV)
			// leader transitions should not go backwards
			if respV < i {
				t.Errorf(`got observe value %q, expected >= "%d"`, string(resp.Kv.Value), i)
			}
			i = respV
		}
	}()

	select {
	case <-observec:
	case <-time.After(time.Second):
		t.Fatalf("observe stream took too long to start")
	}

	lease1, err1 := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(t.Context(), &pb.LeaseGrantRequest{TTL: 30})
	require.NoError(t, err1)
	c1, cerr1 := lc.Campaign(t.Context(), &epb.CampaignRequest{Name: []byte("foo"), Lease: lease1.ID, Value: []byte("0")})
	require.NoError(t, cerr1)

	// overlap other leader so it waits on resign
	leader2c := make(chan struct{})
	go func() {
		defer close(leader2c)

		lease2, err2 := integration.ToGRPC(clus.RandClient()).Lease.LeaseGrant(t.Context(), &pb.LeaseGrantRequest{TTL: 30})
		if err2 != nil {
			t.Error(err2)
		}
		c2, cerr2 := lc.Campaign(t.Context(), &epb.CampaignRequest{Name: []byte("foo"), Lease: lease2.ID, Value: []byte("5")})
		if cerr2 != nil {
			t.Error(cerr2)
		}
		for i := 6; i < 10; i++ {
			v := []byte(fmt.Sprintf("%d", i))
			req := &epb.ProclaimRequest{Leader: c2.Leader, Value: v}
			if _, err := lc.Proclaim(t.Context(), req); err != nil {
				t.Error(err)
			}
		}
	}()

	for i := 1; i < 5; i++ {
		v := []byte(fmt.Sprintf("%d", i))
		req := &epb.ProclaimRequest{Leader: c1.Leader, Value: v}
		_, err := lc.Proclaim(t.Context(), req)
		require.NoError(t, err)
	}
	// start second leader
	lc.Resign(t.Context(), &epb.ResignRequest{Leader: c1.Leader})

	select {
	case <-observec:
	case <-time.After(time.Second):
		t.Fatalf("observe did not observe all events in time")
	}

	<-leader2c
}

func TestV3ElectionCampaignAuthTokenExpiration(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:      1,
		AuthToken: integration.DefaultTokenJWT,
	})
	defer clus.Terminate(t)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	rootc1, cerr1 := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, cerr1)
	defer rootc1.Close()

	authResp, aerr := rootc1.Authenticate(t.Context(), "root", "123")
	require.NoError(t, aerr)

	rootc2, cerr2 := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Token:     authResp.Token,
	})
	require.NoError(t, cerr2)
	defer rootc2.Close()

	lease1, err1 := rootc1.Grant(t.Context(), 30)
	require.NoError(t, err1)
	lease2, err2 := rootc2.Grant(t.Context(), 30)
	require.NoError(t, err2)

	lc1 := integration.ToGRPC(rootc1).Election
	lc2 := integration.ToGRPC(rootc2).Election

	req1 := &epb.CampaignRequest{Name: []byte("foo"), Lease: int64(lease1.ID), Value: []byte("leader1")}
	l1, lerr1 := lc1.Campaign(t.Context(), req1)
	require.NoError(t, lerr1)

	campaignc := make(chan error, 1)
	go func() {
		req2 := &epb.CampaignRequest{Name: []byte("foo"), Lease: int64(lease2.ID), Value: []byte("leader2")}
		_, lerr2 := lc2.Campaign(t.Context(), req2)
		campaignc <- lerr2
	}()

	// Wait 3 seconds so the JWT auth token (ttl=2s) expires while leader 2 is blocked in Campaign
	time.Sleep(3 * time.Second)

	_, uerr := lc1.Resign(t.Context(), &epb.ResignRequest{Leader: l1.Leader})
	require.NoError(t, uerr)

	select {
	case err := <-campaignc:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("campaigner timed out after resign")
	}
}

func TestV3ElectionCampaignAuthRevisionUpdate(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:      1,
		AuthToken: integration.DefaultTokenJWT,
	})
	defer clus.Terminate(t)

	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	rootc1, cerr1 := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Username:  "root",
		Password:  "123",
	})
	require.NoError(t, cerr1)
	defer rootc1.Close()

	authResp, aerr := rootc1.Authenticate(t.Context(), "root", "123")
	require.NoError(t, aerr)

	rootc2, cerr2 := integration.NewClient(t, clientv3.Config{
		Endpoints: clus.Client(0).Endpoints(),
		Token:     authResp.Token,
	})
	require.NoError(t, cerr2)
	defer rootc2.Close()

	lease1, err1 := rootc1.Grant(t.Context(), 30)
	require.NoError(t, err1)
	lease2, err2 := rootc2.Grant(t.Context(), 30)
	require.NoError(t, err2)

	lc1 := integration.ToGRPC(rootc1).Election
	lc2 := integration.ToGRPC(rootc2).Election

	req1 := &epb.CampaignRequest{Name: []byte("foo"), Lease: int64(lease1.ID), Value: []byte("leader1")}
	l1, lerr1 := lc1.Campaign(t.Context(), req1)
	require.NoError(t, lerr1)

	campaignc := make(chan error, 1)
	go func() {
		req2 := &epb.CampaignRequest{Name: []byte("foo"), Lease: int64(lease2.ID), Value: []byte("leader2")}
		_, lerr2 := lc2.Campaign(t.Context(), req2)
		campaignc <- lerr2
	}()

	// Give follower time to block in Campaign waitDeletes
	time.Sleep(500 * time.Millisecond)

	// Modify auth revision while follower is blocked
	_, err := rootc1.RoleAdd(t.Context(), "new-role-revision-bump")
	require.NoError(t, err)

	_, uerr := lc1.Resign(t.Context(), &epb.ResignRequest{Leader: l1.Leader})
	require.NoError(t, uerr)

	select {
	case err := <-campaignc:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatalf("campaigner timed out after resign")
	}
}
