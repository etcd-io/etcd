// Copyright 2021 The etcd Authors
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

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestEtcdVersionFromWAL(t *testing.T) {
	testCases := []struct {
		name               string
		operation          func(ctx context.Context, c *clientv3.Client) error
		expectedWALVersion *semver.Version
	}{
		{
			name: "normal operation",
			operation: func(ctx context.Context, c *clientv3.Client) error {
				_, err := c.AuthStatus(ctx)
				return err
			},
			expectedWALVersion: &semver.Version{Major: 3, Minor: 5},
		},
		{
			name: "downgrade version 3.6 test operation",
			operation: func(ctx context.Context, c *clientv3.Client) error {
				_, err := c.DowngradeVersionTest(ctx, "3.6.0")
				return err
			},
			expectedWALVersion: &semver.Version{Major: 3, Minor: 6},
		},
		{
			name: "downgrade version 3.7 test operation",
			operation: func(ctx context.Context, c *clientv3.Client) error {
				_, err := c.DowngradeVersionTest(ctx, "3.7.0")
				return err
			},
			expectedWALVersion: &semver.Version{Major: 3, Minor: 7},
		},
	}

	for _, tc := range testCases {
		testutil.SkipTestIfShortMode(t,
			"Wal creation tests are depending on embedded etcd server so are integration-level tests.")
		cfg := integration.NewEmbedConfig(t, "default")
		srv, err := embed.StartEtcd(cfg)
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-srv.Server.ReadyNotify():
		case <-time.After(3 * time.Second):
			t.Fatalf("failed to start embed.Etcd for test")
		}

		ccfg := clientv3.Config{Endpoints: []string{cfg.AdvertiseClientUrls[0].String()}}
		cli, err := integration.NewClient(t, ccfg)
		if err != nil {
			srv.Close()
			t.Fatal(err)
		}

		// Once the cluster version has been updated, any entity's storage
		// version should be align with cluster version.
		ctx, cancel := context.WithTimeout(context.Background(), testutil.RequestTimeout)
		err = tc.operation(ctx, cli)
		cancel()
		if err != nil {
			srv.Close()
			t.Fatalf("failed to get auth status: %v", err)
		}

		cli.Close()
		srv.Close()

		w, err := wal.Open(zap.NewNop(), cfg.Dir+"/member/wal", walpb.Snapshot{})
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()

		walVersion, err := wal.ReadWALVersion(w)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, tc.expectedWALVersion, walVersion.MinimalEtcdVersion())
	}
}
