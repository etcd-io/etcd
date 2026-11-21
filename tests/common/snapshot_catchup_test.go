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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestEtcdHealthyWithTinySnapshotCatchupEntries ensures multi-node etcd cluster remains healthy with 10 snapshot catch up entry
func TestEtcdHealthyWithTinySnapshotCatchupEntries(t *testing.T) {
	testRunner.BeforeTest(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{
		ClusterSize:            3,
		SnapshotCount:          10,
		SnapshotCatchUpEntries: 10,
	}))
	defer clus.Close()
	cc := testutils.MustClient(clus.Client())

	// simulate 10 clients keep writing to etcd in parallel with no error
	testutils.ExecuteUntil(ctx, t, func() {
		g, ctx := errgroup.WithContext(ctx)
		for i := 0; i < 10; i++ {
			clientID := i
			g.Go(func() error {
				for j := 0; j < 100; j++ {
					if _, err := cc.Put(ctx, "foo", fmt.Sprintf("bar%d", clientID), config.PutOptions{}); err != nil {
						return err
					}
				}
				return nil
			})
		}
		require.NoError(t, g.Wait())
	})
}