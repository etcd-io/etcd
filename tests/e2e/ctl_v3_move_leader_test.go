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

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
)

func TestCtlV3MoveLeader(t *testing.T) {
	defer testutil.AfterTest(t)

	epc := setupEtcdctlTest(t, &configNoTLS, true)
	defer func() {
		if errC := epc.Close(); errC != nil {
			t.Fatalf("error closing etcd processes (%v)", errC)
		}
	}()

	var leadIdx int
	var leaderID uint64
	var transferee uint64
	for i, ep := range epc.EndpointsV3() {
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   []string{ep},
			DialTimeout: 3 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		resp, err := cli.Status(context.Background(), ep)
		if err != nil {
			t.Fatal(err)
		}
		cli.Close()

		if resp.Header.GetMemberId() == resp.Leader {
			leadIdx = i
			leaderID = resp.Leader
		} else {
			transferee = resp.Header.GetMemberId()
		}
	}

	os.Setenv("ETCDCTL_API", "3")
	defer os.Unsetenv("ETCDCTL_API")
	cx := ctlCtx{
		t:           t,
		cfg:         configNoTLS,
		dialTimeout: 7 * time.Second,
		epc:         epc,
	}

	tests := []struct {
		prefixes []string
		expect   string
	}{
		{ // request to non-leader
			cx.prefixArgs([]string{cx.epc.EndpointsV3()[(leadIdx+1)%3]}),
			"no leader endpoint given at ",
		},
		{ // request to leader
			cx.prefixArgs([]string{cx.epc.EndpointsV3()[leadIdx]}),
			fmt.Sprintf("Leadership transferred from %s to %s", types.ID(leaderID), types.ID(transferee)),
		},
	}
	for i, tc := range tests {
		cmdArgs := append(tc.prefixes, "move-leader", types.ID(transferee).String())
		if err := spawnWithExpect(cmdArgs, tc.expect); err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
	}
}
