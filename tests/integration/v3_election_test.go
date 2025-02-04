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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestElectionWait tests if followers can correctly wait for elections.
func TestElectionWait(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	leaders := 3
	followers := 3
	var clients []*clientv3.Client
	newClient := integration.MakeMultiNodeClients(t, clus, &clients)
	defer func() {
		integration.CloseClients(t, clients)
	}()

	electedc := make(chan string)
	var nextc []chan struct{}

	// wait for all elections
	donec := make(chan struct{})
	for i := 0; i < followers; i++ {
		nextc = append(nextc, make(chan struct{}))
		go func(ch chan struct{}) {
			for j := 0; j < leaders; j++ {
				session, err := concurrency.NewSession(newClient())
				if err != nil {
					t.Error(err)
				}
				b := concurrency.NewElection(session, "test-election")

				cctx, cancel := context.WithCancel(context.TODO())
				defer cancel()
				s, ok := <-b.Observe(cctx)
				if !ok {
					t.Errorf("could not observe election; channel closed")
				}
				electedc <- string(s.Kvs[0].Value)
				// wait for next election round
				<-ch
				session.Orphan()
			}
			donec <- struct{}{}
		}(nextc[i])
	}

	// elect some leaders
	for i := 0; i < leaders; i++ {
		go func() {
			session, err := concurrency.NewSession(newClient())
			if err != nil {
				t.Error(err)
			}
			defer session.Orphan()

			e := concurrency.NewElection(session, "test-election")
			ev := fmt.Sprintf("electval-%v", time.Now().UnixNano())
			if err := e.Campaign(context.TODO(), ev); err != nil {
				t.Errorf("failed volunteer (%v)", err)
			}
			// wait for followers to accept leadership
			for j := 0; j < followers; j++ {
				s := <-electedc
				if s != ev {
					t.Errorf("wrong election value got %s, wanted %s", s, ev)
				}
			}
			// let next leader take over
			if err := e.Resign(context.TODO()); err != nil {
				t.Errorf("failed resign (%v)", err)
			}
			// tell followers to start listening for next leader
			for j := 0; j < followers; j++ {
				nextc[j] <- struct{}{}
			}
		}()
	}

	// wait on followers
	for i := 0; i < followers; i++ {
		<-donec
	}
}

// TestElectionFailover tests that an election will
func TestElectionFailover(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	cctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	ss := make([]*concurrency.Session, 3)

	for i := 0; i < 3; i++ {
		var err error
		ss[i], err = concurrency.NewSession(clus.Client(i))
		if err != nil {
			t.Error(err)
		}
		defer ss[i].Orphan()
	}

	// first leader (elected)
	e := concurrency.NewElection(ss[0], "test-election")
	if err := e.Campaign(context.TODO(), "foo"); err != nil {
		t.Fatalf("failed volunteer (%v)", err)
	}

	// check first leader
	resp, ok := <-e.Observe(cctx)
	if !ok {
		t.Fatalf("could not wait for first election; channel closed")
	}
	s := string(resp.Kvs[0].Value)
	if s != "foo" {
		t.Fatalf("wrong election result. got %s, wanted foo", s)
	}

	// next leader
	electedErrC := make(chan error, 1)
	go func() {
		ee := concurrency.NewElection(ss[1], "test-election")
		eer := ee.Campaign(context.TODO(), "bar")
		electedErrC <- eer // If eer != nil, the test will fail by calling t.Fatal(eer)
	}()

	// invoke leader failover
	err := ss[0].Close()
	require.NoError(t, err)

	// check new leader
	e = concurrency.NewElection(ss[2], "test-election")
	resp, ok = <-e.Observe(cctx)
	if !ok {
		t.Fatalf("could not wait for second election; channel closed")
	}
	s = string(resp.Kvs[0].Value)
	if s != "bar" {
		t.Fatalf("wrong election result. got %s, wanted bar", s)
	}

	// leader must ack election (otherwise, Campaign may see closed conn)
	eer := <-electedErrC
	require.NoError(t, eer)
}

// TestElectionSessionRecampaign ensures that campaigning twice on the same election
// with the same lock will Proclaim instead of deadlocking.
func TestElectionSessionRecampaign(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)
	cli := clus.RandClient()

	session, err := concurrency.NewSession(cli)
	if err != nil {
		t.Error(err)
	}
	defer session.Orphan()

	e := concurrency.NewElection(session, "test-elect")
	err = e.Campaign(context.TODO(), "abc")
	require.NoError(t, err)
	e2 := concurrency.NewElection(session, "test-elect")
	err = e2.Campaign(context.TODO(), "def")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	if resp := <-e.Observe(ctx); len(resp.Kvs) == 0 || string(resp.Kvs[0].Value) != "def" {
		t.Fatalf("expected value=%q, got response %v", "def", resp)
	}
}

// TestElectionOnPrefixOfExistingKey checks that a single
// candidate can be elected on a new key that is a prefix
// of an existing key. To wit, check for regression
// of bug #6278. https://github.com/etcd-io/etcd/issues/6278
func TestElectionOnPrefixOfExistingKey(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	_, err := cli.Put(context.TODO(), "testa", "value")
	require.NoError(t, err)
	s, serr := concurrency.NewSession(cli)
	require.NoError(t, serr)
	e := concurrency.NewElection(s, "test")
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	err = e.Campaign(ctx, "abc")
	cancel()
	// after 5 seconds, deadlock results in
	// 'context deadline exceeded' here.
	require.NoError(t, err)
}

// TestElectionOnSessionRestart tests that a quick restart of leader (resulting
// in a new session with the same lease id) does not result in loss of
// leadership.
func TestElectionOnSessionRestart(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)
	cli := clus.RandClient()

	session, err := concurrency.NewSession(cli)
	require.NoError(t, err)

	e := concurrency.NewElection(session, "test-elect")
	require.NoError(t, e.Campaign(context.TODO(), "abc"))

	// ensure leader is not lost to waiter on fail-over
	waitSession, werr := concurrency.NewSession(cli)
	require.NoError(t, werr)
	defer waitSession.Orphan()
	waitCtx, waitCancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer waitCancel()
	go concurrency.NewElection(waitSession, "test-elect").Campaign(waitCtx, "123")

	// simulate restart by reusing the lease from the old session
	newSession, nerr := concurrency.NewSession(cli, concurrency.WithLease(session.Lease()))
	require.NoError(t, nerr)
	defer newSession.Orphan()

	newElection := concurrency.NewElection(newSession, "test-elect")
	require.NoError(t, newElection.Campaign(context.TODO(), "def"))

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	if resp := <-newElection.Observe(ctx); len(resp.Kvs) == 0 || string(resp.Kvs[0].Value) != "def" {
		t.Errorf("expected value=%q, got response %v", "def", resp)
	}
}

// TestElectionObserveCompacted checks that observe can tolerate
// a leader key with a modrev less than the compaction revision.
func TestElectionObserveCompacted(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cli := clus.Client(0)

	session, err := concurrency.NewSession(cli)
	require.NoError(t, err)
	defer session.Orphan()

	e := concurrency.NewElection(session, "test-elect")
	require.NoError(t, e.Campaign(context.TODO(), "abc"))

	presp, perr := cli.Put(context.TODO(), "foo", "bar")
	require.NoError(t, perr)
	_, cerr := cli.Compact(context.TODO(), presp.Header.Revision)
	require.NoError(t, cerr)

	v, ok := <-e.Observe(context.TODO())
	if !ok {
		t.Fatal("failed to observe on compacted revision")
	}
	if string(v.Kvs[0].Value) != "abc" {
		t.Fatalf(`expected leader value "abc", got %q`, string(v.Kvs[0].Value))
	}
}

// TestElectionWithAuthEnabled verifies the election interface when auth is enabled.
// Refer to the discussion in https://github.com/etcd-io/etcd/issues/17502
func TestElectionWithAuthEnabled(t *testing.T) {
	integration.BeforeTest(t)
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	users := []user{
		{
			name:     "user1",
			password: "123",
			role:     "role1",
			key:      "/foo1", // prefix /foo1
			end:      "/foo2",
		},
		{
			name:     "user2",
			password: "456",
			role:     "role2",
			key:      "/bar1", // prefix /bar1
			end:      "/bar2",
		},
	}

	t.Log("Setting rbac info and enable auth.")
	authSetupUsers(t, integration.ToGRPC(clus.Client(0)).Auth, users)
	authSetupRoot(t, integration.ToGRPC(clus.Client(0)).Auth)

	c1, c1err := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user1", Password: "123"})
	require.NoError(t, c1err)
	defer c1.Close()

	c2, c2err := integration.NewClient(t, clientv3.Config{Endpoints: clus.Client(0).Endpoints(), Username: "user2", Password: "456"})
	require.NoError(t, c2err)
	defer c2.Close()

	campaigns := []struct {
		name      string
		c         *clientv3.Client
		pfx       string
		sleepTime time.Duration // time to sleep before campaigning
	}{
		{
			name: "client1 first campaign",
			c:    c1,
			pfx:  "/foo1/a",
		},
		{
			name: "client1 second campaign",
			c:    c1,
			pfx:  "/foo1/a",
		},
		{
			name:      "client2 first campaign",
			c:         c2,
			pfx:       "/bar1/b",
			sleepTime: 5 * time.Second,
		},
		{
			name:      "client2 second campaign",
			c:         c2,
			pfx:       "/bar1/b",
			sleepTime: 6 * time.Second,
		},
	}

	t.Log("Starting to campaign with multiple users.")
	var wg sync.WaitGroup
	errC := make(chan error, 8)
	doneC := make(chan error)
	for _, campaign := range campaigns {
		campaign := campaign
		wg.Add(1)
		go func() {
			defer wg.Done()
			if campaign.sleepTime > 0 {
				time.Sleep(campaign.sleepTime)
			}

			s, serr := concurrency.NewSession(campaign.c, concurrency.WithTTL(10))
			if serr != nil {
				errC <- fmt.Errorf("[NewSession] %s: %w", campaign.name, serr)
			}
			s.Orphan()

			e := concurrency.NewElection(s, campaign.pfx)
			eerr := e.Campaign(context.Background(), "whatever")
			if eerr != nil {
				errC <- fmt.Errorf("[Campaign] %s: %w", campaign.name, eerr)
			}
		}()
	}

	go func() {
		t.Log("Waiting for all goroutines to finish.")
		defer close(doneC)
		wg.Wait()
	}()

	select {
	case err := <-errC:
		t.Fatalf("Error: %v", err)
	case <-doneC:
		t.Log("All goroutine done!")
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out")
	}
}
