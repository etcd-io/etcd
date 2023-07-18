// Copyright 2022 The etcd Authors
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

package concurrency_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestSessionOptions(t *testing.T) {
	cli, err := integration2.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	lease, err := cli.Grant(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	s, err := concurrency.NewSession(cli, concurrency.WithLease(lease.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assert.Equal(t, s.Lease(), lease.ID)

	go s.Orphan()
	select {
	case <-s.Done():
	case <-time.After(time.Millisecond * 100):
		t.Fatal("session did not get orphaned as expected")
	}

}
func TestSessionTTLOptions(t *testing.T) {
	cli, err := integration2.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	var setTTL int = 90
	s, err := concurrency.NewSession(cli, concurrency.WithTTL(setTTL))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	leaseId := s.Lease()
	// TTL retrieved should be less than the set TTL, but not equal to default:60 or exprired:-1
	resp, err := cli.Lease.TimeToLive(context.Background(), leaseId)
	if err != nil {
		t.Log(err)
	}
	if resp.TTL == -1 {
		t.Errorf("client lease should not be expired: %d", resp.TTL)

	}
	if resp.TTL == 60 {
		t.Errorf("default TTL value is used in the session, instead of set TTL: %d", setTTL)
	}
	if resp.TTL >= int64(setTTL) || resp.TTL < int64(setTTL)-20 {
		t.Errorf("Session TTL from lease should be less, but close to set TTL %d, have: %d", setTTL, resp.TTL)
	}

}

func TestSessionCtx(t *testing.T) {
	cli, err := integration2.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	lease, err := cli.Grant(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	s, err := concurrency.NewSession(cli, concurrency.WithLease(lease.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assert.Equal(t, s.Lease(), lease.ID)

	childCtx, cancel := context.WithCancel(s.Ctx())
	defer cancel()

	go s.Orphan()
	select {
	case <-childCtx.Done():
	case <-time.After(time.Millisecond * 100):
		t.Fatal("child context of session context is not canceled")
	}
	assert.Equal(t, childCtx.Err(), context.Canceled)
}
