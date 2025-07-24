// Copyright 2025 The etcd Authors
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

//go:build !cluster_proxy

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestReproduce20340 reproduces the issue https://github.com/etcd-io/etcd/issues/20340.
// Refer to https://github.com/etcd-io/etcd/issues/20340#issuecomment-3105037914.
func TestReproduce20340(t *testing.T) {
	e2e.BeforeTest(t)

	ctx := context.Background()

	epc, members := mustCreateNewClusterByPromotingMembers(t, e2e.CurrentVersion, 3)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	t.Logf("Removing the second member (%x)", members[0].ID)
	etcdctl := epc.Procs[0].Etcdctl()
	testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
		for {
			_, merr := etcdctl.MemberRemove(ctx, members[0].ID)
			if merr != nil {
				if strings.Contains(merr.Error(), "etcdserver: unhealthy cluster") {
					time.Sleep(1 * time.Second)
					continue
				}
				t.Fatalf("failed to remove member: %s", merr.Error())
			}
			break
		}
	})

	epc.Procs = append(epc.Procs[:1], epc.Procs[2:]...)

	t.Logf("Restarting member: %s", epc.Procs[0].Config().Name)
	require.NoError(t, epc.Procs[0].Restart(ctx))
}
