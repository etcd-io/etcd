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
	"os"
	"strings"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestAlarm(t *testing.T) {
	testRunner.BeforeTest(t)
	clus := testRunner.NewCluster(t, config.ClusterConfig{ClusterSize: 1, QuotaBackendBytes: int64(13 * os.Getpagesize())})
	defer clus.Close()
	testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
		// test small put still works
		smallbuf := strings.Repeat("a", 64)
		if err := clus.Client().Put("1st_test", smallbuf, config.PutOptions{}); err != nil {
			t.Fatalf("alarmTest: put kv error (%v)", err)
		}

		// write some chunks to fill up the database
		buf := strings.Repeat("b", os.Getpagesize())
		for {
			if err := clus.Client().Put("2nd_test", buf, config.PutOptions{}); err != nil {
				if !strings.Contains(err.Error(), "etcdserver: mvcc: database space exceeded") {
					t.Fatal(err)
				}
				break
			}
		}

		// quota alarm should now be on
		alarmResp, err := clus.Client().AlarmList()
		if err != nil {
			t.Fatalf("alarmTest: Alarm error (%v)", err)
		}

		// check that Put is rejected when alarm is on
		if err := clus.Client().Put("3rd_test", smallbuf, config.PutOptions{}); err != nil {
			if !strings.Contains(err.Error(), "etcdserver: mvcc: database space exceeded") {
				t.Fatal(err)
			}
		}

		// get latest revision to compact
		sresp, err := clus.Client().Status()
		if err != nil {
			t.Fatalf("get endpoint status error: %v", err)
		}
		var rvs int64
		for _, resp := range sresp {
			if resp != nil && resp.Header != nil {
				rvs = resp.Header.Revision
				break
			}
		}

		// make some space
		_, err = clus.Client().Compact(rvs, config.CompactOption{Physical: true, Timeout: 10 * time.Second})
		if err != nil {
			t.Fatalf("alarmTest: Compact error (%v)", err)
		}

		if err = clus.Client().Defragment(config.DefragOption{Timeout: 10 * time.Second}); err != nil {
			t.Fatalf("alarmTest: defrag error (%v)", err)
		}

		// turn off alarm
		for _, alarm := range alarmResp.Alarms {
			alarmMember := &clientv3.AlarmMember{
				MemberID: alarm.MemberID,
				Alarm:    alarm.Alarm,
			}
			_, err = clus.Client().AlarmDisarm(alarmMember)
			if err != nil {
				t.Fatalf("alarmTest: Alarm error (%v)", err)
			}
		}

		// put one more key below quota
		if err := clus.Client().Put("4th_test", smallbuf, config.PutOptions{}); err != nil {
			t.Fatal(err)
		}
	})
}
