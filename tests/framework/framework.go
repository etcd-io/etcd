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

package framework

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/config"
)

var (
	// UnitTestRunner only runs in `--short` mode, will fail otherwise. Attempts in cluster creation will result in tests being skipped.
	UnitTestRunner testRunner = unitRunner{}
	// E2eTestRunner runs etcd and etcdctl binaries in a separate process.
	E2eTestRunner = e2eRunner{}
	// IntegrationTestRunner runs etcdserver.EtcdServer in separate goroutine and uses client libraries to communicate.
	IntegrationTestRunner = integrationRunner{}
)

const TickDuration = 10 * time.Millisecond

// WaitLeader returns index of the member in c.Members() that is leader
// or fails the test (if members don't agree on a legal leader in 30s).
func WaitLeader(t testing.TB, c Cluster) int {
	return WaitMembersForLeader(t, c, c.Members())
}

// WaitMembersForLeader waits until given members agree on the same leader,
// and returns its 'index' in the 'membs' list
// or fails the test (if members don't agree on a legal leader in 30s).
func WaitMembersForLeader(t testing.TB, c Cluster, membs []Member) int {
	t.Logf("WaitMembersForLeader")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	l := 0
	for l = waitMembersForLeader(ctx, t, c, membs); l < 0; {
		if ctx.Err() != nil {
			t.Fatalf("waitMembersForLeader failed: %v", ctx.Err())
		}
	}
	t.Logf("waitMembersForLeader succeeded. Cluster leader index: %v", l)
	return l
}

// WaitMembersForLeader waits until given members agree on the same leader,
// and returns its 'index' in the 'membs' list.
// or fails the test (if members don't agree on a legal leader before deadline).
func waitMembersForLeader(ctx context.Context, t testing.TB, c Cluster, membs []Member) int {
	cc := c.Client()

	// ensure leader is up via linearizable get
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("waitMembersForLeader timeout")
		default:
		}
		_, err := cc.Get("0", config.GetOptions{Timeout: 10*TickDuration + time.Second})
		if err == nil || strings.Contains(err.Error(), "Key not found") {
			break
		}
	}

	leaders := make(map[uint64]struct{})
	members := make(map[uint64]int)
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("waitMembersForLeader timeout")
		default:
		}
		for i := range membs {
			resp, err := membs[i].Client().Status()
			if err != nil {
				// UNIX socket: failed to dial   IP socket: Error while dialing
				if strings.Contains(err.Error(), "failed to dial") ||
					strings.Contains(err.Error(), "Error while dialing") {
					// if member[i] has stopped
					continue
				} else {
					t.Fatal(err)
				}
			}
			members[resp[0].Header.MemberId] = i
			leaders[resp[0].Leader] = struct{}{}
		}
		// members agree on the same leader
		if len(leaders) == 1 {
			break
		}
		leaders = make(map[uint64]struct{})
		members = make(map[uint64]int)
		time.Sleep(10 * TickDuration)
	}
	for l := range leaders {
		if index, ok := members[l]; ok {
			t.Logf("members agree on a leader, members:%v , leader:%v", members, l)
			return index
		}
		t.Fatalf("members agree on a leader which is not one of members, members:%v , leader:%v", members, l)
	}
	t.Fatalf("impossible path of execution")
	return -1
}
