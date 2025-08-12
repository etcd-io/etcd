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

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestStatus(t *testing.T) {
	testRunner.BeforeTest(t)

	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				rs, err := cc.Status(ctx)
				require.NoErrorf(t, err, "could not get status")
				require.Lenf(t, rs, tc.config.ClusterSize, "wrong number of status responses. expected:%d, got:%d ", tc.config.ClusterSize, len(rs))
				memberIDs := make(map[uint64]struct{})
				for _, r := range rs {
					require.NotNilf(t, r, "status response is nil")
					memberIDs[r.Header.MemberId] = struct{}{}
				}
				require.Lenf(t, rs, len(memberIDs), "found duplicated members")
			})
		})
	}
}
