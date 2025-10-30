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

package common

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestAlarm(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t,
		config.WithClusterSize(1),
		config.WithQuotaBackendBytes(int64(13*os.Getpagesize())),
	)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())
	testutils.ExecuteUntil(ctx, t, func() {
		// test small put still works
		smallbuf := strings.Repeat("a", 64)
		require.NoErrorf(t, cc.Put(ctx, "1st_test", smallbuf, config.PutOptions{}), "alarmTest: put kv error")

		// write some chunks to fill up the database
		buf := strings.Repeat("b", os.Getpagesize())
		for {
			if err := cc.Put(ctx, "2nd_test", buf, config.PutOptions{}); err != nil {
				require.ErrorContains(t, err, "etcdserver: mvcc: database space exceeded")
				break
			}
		}

		// quota alarm should now be on
		alarmResp, err := cc.AlarmList(ctx)
		require.NoErrorf(t, err, "alarmTest: Alarm error")

		// check that Put is rejected when alarm is on
		if err = cc.Put(ctx, "3rd_test", smallbuf, config.PutOptions{}); err != nil {
			require.ErrorContains(t, err, "etcdserver: mvcc: database space exceeded")
		}

		// get latest revision to compact
		sresp, err := cc.Status(ctx)
		require.NoErrorf(t, err, "get endpoint status error")
		var rvs int64
		for _, resp := range sresp {
			if resp != nil && resp.Header != nil {
				rvs = resp.Header.Revision
				break
			}
		}

		// make some space
		_, err = cc.Compact(ctx, rvs, config.CompactOption{Physical: true, Timeout: 10 * time.Second})
		require.NoErrorf(t, err, "alarmTest: Compact error")

		err = cc.Defragment(ctx, config.DefragOption{Timeout: 10 * time.Second})
		require.NoErrorf(t, err, "alarmTest: defrag error")

		// turn off alarm
		for _, alarm := range alarmResp.Alarms {
			alarmMember := &clientv3.AlarmMember{
				MemberID: alarm.MemberID,
				Alarm:    alarm.Alarm,
			}
			_, err = cc.AlarmDisarm(ctx, alarmMember)
			require.NoErrorf(t, err, "alarmTest: Alarm error")
		}

		// put one more key below quota
		err = cc.Put(ctx, "4th_test", smallbuf, config.PutOptions{})
		require.NoError(t, err)
	})
}

func TestAlarmlistOnMemberRestart(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t,
		config.WithClusterSize(1),
		config.WithQuotaBackendBytes(int64(13*os.Getpagesize())),
		config.WithSnapshotCount(5),
	)
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())

	testutils.ExecuteUntil(ctx, t, func() {
		for i := 0; i < 6; i++ {
			_, err := cc.AlarmList(ctx)
			require.NoError(t, err)
		}

		clus.Members()[0].Stop()
		err := clus.Members()[0].Start(ctx)
		require.NoErrorf(t, err, "failed to start etcdserver")
	})
}
