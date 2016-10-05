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

package e2e

import (
	"os"
	"strings"
	"testing"
)

func TestCtlV3Alarm(t *testing.T) {
	// The boltdb minimum working set is six pages.
	testCtl(t, alarmTest, withQuota(int64(13*os.Getpagesize())))
}

func alarmTest(cx ctlCtx) {
	// test small put still works
	smallbuf := strings.Repeat("a", 64)
	if err := ctlV3Put(cx, "1st_test", smallbuf, ""); err != nil {
		cx.t.Fatal(err)
	}

	// write some chunks to fill up the database
	buf := strings.Repeat("b", int(os.Getpagesize()))
	var rev int64
	for ; ; rev++ {
		if err := ctlV3Put(cx, "2nd_test", buf, ""); err != nil {
			if !strings.Contains(err.Error(), "etcdserver: mvcc: database space exceeded") {
				cx.t.Fatal(err)
			}
			break
		}
	}

	// quota alarm should now be on
	if err := ctlV3Alarm(cx, "list", "alarm:NOSPACE"); err != nil {
		cx.t.Fatal(err)
	}

	// check that Put is rejected when alarm is on
	if err := ctlV3Put(cx, "3rd_test", smallbuf, ""); err != nil {
		if !strings.Contains(err.Error(), "etcdserver: mvcc: database space exceeded") {
			cx.t.Fatal(err)
		}
	}

	// make some space
	if err := ctlV3Compact(cx, rev, true); err != nil {
		cx.t.Fatal(err)
	}
	if err := ctlV3Defrag(cx); err != nil {
		cx.t.Fatal(err)
	}

	// turn off alarm
	if err := ctlV3Alarm(cx, "disarm", "alarm:NOSPACE"); err != nil {
		cx.t.Fatal(err)
	}

	// put one more key below quota
	if err := ctlV3Put(cx, "4th_test", smallbuf, ""); err != nil {
		cx.t.Fatal(err)
	}
}

func ctlV3Alarm(cx ctlCtx, cmd string, as ...string) error {
	cmdArgs := append(cx.PrefixArgs(), "alarm", cmd)
	return spawnWithExpects(cmdArgs, as...)
}
