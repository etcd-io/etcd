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
	"go.etcd.io/etcd/tests/v3/framework/testutils"
	"testing"
	"time"
)

func TestMemberList(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteUntil(ctx, t, func() {
				resp, err := cc.MemberList()
				if err != nil {
					t.Fatalf("could not get member list, err: %s", err)
				}
				expectNum := len(clus.Members())
				gotNum := len(resp.Members)
				if expectNum != gotNum {
					t.Fatalf("number of members not equal, expect: %d, got: %d", expectNum, gotNum)
				}
				for _, m := range resp.Members {
					if len(m.ClientURLs) == 0 {
						t.Fatalf("member is not started, memberId:%d, memberName:%s", m.ID, m.Name)
					}
				}
			})
		})
	}
}
