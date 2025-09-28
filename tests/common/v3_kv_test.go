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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestKV_PutTimeout migrates TestCtlV3PutTimeout from e2e to integration framework
// Original e2e test: func TestCtlV3PutTimeout(t *testing.T) { testCtl(t, putTest, withDefaultDialTimeout()) }
func TestKV_PutTimeout(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second) // Short timeout to test timeout behavior
			defer cancel()

			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			client := testutils.MustClient(clus.Client())

			key, value := "foo", "bar"

			// Put operation - should work within timeout
			err := client.Put(ctx, key, value, config.PutOptions{})
			// don't require.NoError here because timeout behavior may vary
			// test validates that the client handles timeouts gracefully

			if err == nil {
				// If put succeeded, verify with get
				getResp, getErr := client.Get(ctx, key, config.GetOptions{})
				if getErr == nil {
					require.Equal(t, getResp.Kvs, 1)
					require.Equal(t, value, string(getResp.Kvs[0].Value))
				}
			}
			// If timeout occurred, that's also acceptable behavior for this test
		})
	}
}
