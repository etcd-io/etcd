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

package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// TestForceNewCluster verified that etcd works as expected when --force-new-cluster.
// Refer to discussion in https://github.com/etcd-io/etcd/issues/20009.
func TestForceNewCluster(t *testing.T) {
	e2e.BeforeTest(t)

	testCases := []struct {
		name      string
		snapcount int
	}{
		{
			name:      "create a snapshot after promotion",
			snapcount: 10,
		},
		{
			name:      "no snapshot after promotion",
			snapcount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			epc, promotedMembers := mustCreateNewClusterByPromotingMembers(t, e2e.CurrentVersion, 5,
				e2e.WithKeepDataDir(true), e2e.WithSnapshotCount(uint64(tc.snapcount)))
			require.Len(t, promotedMembers, 4)

			for i := 0; i < tc.snapcount; i++ {
				err := epc.Etcdctl().Put(t.Context(), "foo", "bar", config.PutOptions{})
				require.NoError(t, err)
			}

			require.NoError(t, epc.Close())

			m := epc.Procs[0]
			t.Logf("Forcibly create a one-member cluster with member: %s", m.Config().Name)
			m.Config().Args = append(m.Config().Args, "--force-new-cluster")
			require.NoError(t, m.Start(t.Context()))

			t.Log("Restarting the member")
			require.NoError(t, m.Restart(t.Context()))

			t.Log("Closing the member")
			require.NoError(t, m.Close())
		})
	}
}
