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
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestStatus(t *testing.T) {

	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteUntil(ctx, t, func() {
				rs, err := cc.Status(ctx)
				if err != nil {
					t.Fatalf("could not get status, err: %s", err)
				}
				if len(rs) != tc.config.ClusterSize {
					t.Fatalf("wrong number of status responses. expected:%d, got:%d ", tc.config.ClusterSize, len(rs))
				}
				memberIds := make(map[uint64]struct{})
				for _, r := range rs {
					if r == nil {
						t.Fatalf("status response is nil")
					}
					memberIds[r.Header.MemberId] = struct{}{}
				}
				if len(rs) != len(memberIds) {
					t.Fatalf("found duplicated members")
				}
			})
		})
	}
}
