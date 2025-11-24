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

package common

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	"go.etcd.io/etcd/tests/v3/framework/config"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestPeriodicCheckDetectsCorruption(t *testing.T) {
	testRunner.BeforeTest(t)
	const checkTime = time.Second
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			if tc.config.ClusterSize < 2 {
				t.Skip("single-member clusters lack peer hash comparisons, so CORRUPT alarm rarely occurs in this test case.")
			}

			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config), config.WithCorruptCheckTime(checkTime))
			defer clus.Close()

			cc := testutils.MustClient(clus.Client())
			for i := 0; i < 10; i++ {
				err := cc.Put(ctx, testutil.PickKey(int64(i)), fmt.Sprint(i), config.PutOptions{})
				require.NoErrorf(t, err, "error on put: %v", err)
			}

			member := clus.Members()[0]
			memberID := memberIDByName(ctx, t, cc, member.Name())

			member.Stop()
			err := testutil.CorruptBBolt(datadir.ToBackendFileName(member.DataDirPath()))
			require.NoError(t, err)

			err = member.Start(ctx)
			require.NoError(t, err)

			assert.Eventually(t, func() bool {
				alarmResp, err := cc.AlarmList(ctx)
				if err != nil {
					t.Logf("alarm list failed: %v", err)
					return false
				}

				return slices.EqualFunc(alarmResp.Alarms, []*etcdserverpb.AlarmMember{{Alarm: etcdserverpb.AlarmType_CORRUPT, MemberID: memberID}}, func(a, b *etcdserverpb.AlarmMember) bool {
					return a.Alarm == b.Alarm && a.MemberID == b.MemberID
				})

			}, 2*checkTime, 100*time.Millisecond, "expected corruption alarm for member %q", member.Name())
		})
	}
}

func memberIDByName(ctx context.Context, t *testing.T, client intf.Client, name string) uint64 {
	t.Helper()
	resp, err := client.MemberList(ctx, false)
	require.NoError(t, err)
	for _, member := range resp.Members {
		if member.Name == name {
			return member.ID
		}
	}
	t.Fatalf("member %q not found in MemberList response", name)
	return 0
}
