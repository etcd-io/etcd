// Copyright 2023 The etcd Authors
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

package failpoint

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	integration2 "go.etcd.io/etcd/tests/v3/framework/integration"
)

func MemberPromoteMemberNotLearnerTest(t *testing.T, f Failpoint) {
	integration2.BeforeTest(t)

	require.NoError(t, f.Enable())
	defer func() {
		require.NoError(t, f.Disable())
	}()

	clus := integration2.NewCluster(t, &integration2.ClusterConfig{Size: 3})
	defer clus.Terminate(t)
	// member promote request can be sent to any server in cluster,
	// the request will be auto-forwarded to leader on server-side.
	// This test explicitly includes the server-side forwarding by
	// sending the request to follower.
	leaderIdx := clus.WaitLeader(t)
	followerIdx := (leaderIdx + 1) % 3
	cli := clus.Client(followerIdx)

	resp, err := cli.MemberList(context.Background())
	if err != nil {
		t.Fatalf("failed to list member %v", err)
	}
	if len(resp.Members) != 3 {
		t.Fatalf("number of members = %d, want %d", len(resp.Members), 3)
	}

	// promoting any of the voting members in cluster should fail
	expectedErrKeywords := "can only promote a learner member"
	for _, m := range resp.Members {
		_, err = cli.MemberPromote(context.Background(), m.ID)
		if err == nil {
			t.Fatalf("expect promoting voting member to fail, got no error")
		}
		if !strings.Contains(err.Error(), expectedErrKeywords) {
			t.Fatalf("expect error to contain %s, got %s", expectedErrKeywords, err.Error())
		}
	}
}
