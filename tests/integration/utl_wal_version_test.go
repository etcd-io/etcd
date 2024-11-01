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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
	framecfg "go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestEtcdVersionFromWAL(t *testing.T) {
	testutil.SkipTestIfShortMode(t,
		"Wal creation tests are depending on embedded etcd server so are integration-level tests.")
	cfg := integration.NewEmbedConfig(t, "default")
	srv, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	select {
	case <-srv.Server.ReadyNotify():
	case <-time.After(3 * time.Second):
		t.Fatalf("failed to start embed.Etcd for test")
	}

	// When the member becomes leader, it will update the cluster version
	// with the cluster's minimum version. As it's updated asynchronously,
	// it could not be updated in time before close. Wait for it to become
	// ready.
	if err = waitForClusterVersionReady(srv); err != nil {
		srv.Close()
		t.Fatalf("failed to wait for cluster version to become ready: %v", err)
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
	_, err = cli.AuthStatus(ctx)
	cancel()
	if err != nil {
		srv.Close()
		t.Fatalf("failed to get auth status: %v", err)
	}

	cli.Close()
	srv.Close()

	w, err := wal.Open(zap.NewNop(), cfg.Dir+"/member/wal", walpb.Snapshot{})
	require.NoError(t, err)
	defer w.Close()

	walVersion, err := wal.ReadWALVersion(w)
	require.NoError(t, err)
	assert.Equal(t, &semver.Version{Major: 3, Minor: 5}, walVersion.MinimalEtcdVersion())
}

func waitForClusterVersionReady(srv *embed.Etcd) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if srv.Server.ClusterVersion() != nil {
			return nil
		}
		time.Sleep(framecfg.TickDuration)
	}
}
