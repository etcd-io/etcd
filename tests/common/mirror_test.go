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

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// func TestMirror_SyncBaseAndUpdates(t *testing.T) {
// 	testRunner.BeforeTest(t)
// 	for _, tc := range clusterTestCases() {
// 		t.Run(tc.name, func(t *testing.T) {
// 			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
// 			defer cancel()

// 			// Source cluster
// 			src := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
// 			defer src.Close()
// 			srcCli := testutils.MustClient(src.Client())

// 			// Destination cluster
// 			dst := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, ClientTLS: tc.config.ClientTLS, PeerTLS: tc.config.PeerTLS}))
// 			defer dst.Close()
// 			dstCli := testutils.MustClient(dst.Client())

// 			testutils.ExecuteUntil(ctx, t, func() {
// 				// Pre-existing key on source to validate base sync
// 				require.NoError(t, srcCli.Put(ctx, "foo", "bar", config.PutOptions{}))
// 				// Equivalent to ctlV3Get <srcprefix> --prefix on source
// 				getResp, err := srcCli.Get(ctx, "foo", config.GetOptions{})
// 				require.NoError(t, err)
// 				require.Equal(t, []testutils.KV{{Key: "foo", Val: "bar"}}, testutils.KeyValuesFromGetResponse(getResp))

// 				// Start watching destination before mirror starts to capture base copy and updates
// 				wch := dstCli.Watch(ctx, "foo", config.WatchOptions{Revision: 1})

// 				// Start mirror from source to destination
// 				go func() { _ = srcCli.MakeMirror(ctx, dst.Endpoints(), config.MakeMirrorOptions{Prefix: "foo"}) }()

// 				// Expect base copy first
// 				got, err := testutils.KeyValuesFromWatchChan(wch, 1, 5*time.Second)
// 				require.NoError(t, err)
// 				require.Equal(t, []testutils.KV{{Key: "foo", Val: "bar"}}, got)

// 				// Then an update
// 				require.NoError(t, srcCli.Put(ctx, "foo", "bar2", config.PutOptions{}))
// 				got, err = testutils.KeyValuesFromWatchChan(wch, 1, 5*time.Second)
// 				require.NoError(t, err)
// 				require.Equal(t, []testutils.KV{{Key: "foo", Val: "bar2"}}, got)
// 			})
// 		})
// 	}
// }

// func TestMirror_SyncBasePaging(t *testing.T) {
// 	testRunner.BeforeTest(t)
// 	for _, tc := range clusterTestCases() {
// 		t.Run(tc.name, func(t *testing.T) {
// 			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
// 			defer cancel()

// 			// Source cluster
// 			src := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
// 			defer src.Close()
// 			srcCli := testutils.MustClient(src.Client())

// 			// Destination cluster
// 			dst := testRunner.NewCluster(ctx, t, config.WithClusterConfig(config.ClusterConfig{ClusterSize: 1, ClientTLS: tc.config.ClientTLS, PeerTLS: tc.config.PeerTLS}))
// 			defer dst.Close()
// 			dstCli := testutils.MustClient(dst.Client())

// 			testutils.ExecuteUntil(ctx, t, func() {
// 				// Pre-populate many keys on source
// 				for i := 0; i < 2000; i++ {
// 					require.NoError(t, srcCli.Put(ctx, fmt.Sprintf("test%d", i), "v", config.PutOptions{}))
// 				}

// 				// Start watching destination for the base copy events
// 				wch := dstCli.Watch(ctx, "test", config.WatchOptions{Revision: 1, Prefix: true})

// 				// Start the mirror (will perform base sync which may paginate)
// 				go func() { _ = srcCli.MakeMirror(ctx, dst.Endpoints(), config.MakeMirrorOptions{Prefix: "test"}) }()

// 				// Collect exactly 2000 events from destination
// 				got, err := testutils.KeyValuesFromWatchChan(wch, 2000,
//  30*time.Second)
// 				require.NoError(t, err)
// 				require.Len(t, got, 2000)
// 			})
// 		})
// 	}
// }

// func TestMakeMirrorBasic(t *testing.T) {
// 	testRunner.BeforeTest(t)
// 	for _, srcTC := range clusterTestCases() {
// 		for _, destTC := range clusterTestCases() {
// 			t.Run(fmt.Sprintf("Source=%s/Destination=%s", srcTC.name, destTC.name), func(t *testing.T) {
// 				if destTC.config.ClientTLS == config.AutoTLS {
// 					t.Skip("Skipping test for destination cluster with AutoTLS")
// 				}

// 				var (
// 					mmOpts     = config.MakeMirrorOptions{Prefix: "key"}
// 					sourcekvs  = []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key2", Val: "val2"}, {Key: "key3", Val: "val3"}}
// 					destkvs    = []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key2", Val: "val2"}, {Key: "key3", Val: "val3"}}
// 					srcprefix  = "key"
// 					destprefix = "key"
// 				)

// 				testMirror(t, srcTC, destTC, mmOpts, sourcekvs, destkvs, srcprefix, destprefix)
// 			})
// 		}
// 	}
// }

// func TestMakeMirrorModifyDestPrefix(t *testing.T) {
// 	testRunner.BeforeTest(t)
// 	for _, srcTC := range clusterTestCases() {
// 		for _, destTC := range clusterTestCases() {
// 			t.Run(fmt.Sprintf("Source=%s/Destination=%s", srcTC.name, destTC.name), func(t *testing.T) {
// 				if destTC.config.ClientTLS == config.AutoTLS {
// 					t.Skip("Skipping test for destination cluster with AutoTLS")
// 				}

// 				var (
// 					mmOpts     = config.MakeMirrorOptions{Prefix: "o_", DestPrefix: "d_"}
// 					sourcekvs  = []testutils.KV{{Key: "o_key1", Val: "val1"}, {Key: "o_key2", Val: "val2"}, {Key: "o_key3", Val: "val3"}}
// 					destkvs    = []testutils.KV{{Key: "d_key1", Val: "val1"}, {Key: "d_key2", Val: "val2"}, {Key: "d_key3", Val: "val3"}}
// 					srcprefix  = "o_"
// 					destprefix = "d_"
// 				)

// 				testMirror(t, srcTC, destTC, mmOpts, sourcekvs, destkvs, srcprefix, destprefix)
// 			})
// 		}
// 	}
// }

func TestMakeMirrorNoDestPrefix(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, srcTC := range clusterTestCases() {
		for _, destTC := range clusterTestCases() {
			t.Run(fmt.Sprintf("Source=%s/Destination=%s", srcTC.name, destTC.name), func(t *testing.T) {
				if destTC.config.ClientTLS == config.AutoTLS {
					t.Skip("Skipping test for destination cluster with AutoTLS")
				}

				var (
					mmOpts     = config.MakeMirrorOptions{Prefix: "o_", NoDestPrefix: true}
					sourcekvs  = []testutils.KV{{Key: "o_key1", Val: "val1"}, {Key: "o_key2", Val: "val2"}, {Key: "o_key3", Val: "val3"}}
					destkvs    = []testutils.KV{{Key: "key1", Val: "val1"}, {Key: "key2", Val: "val2"}, {Key: "key3", Val: "val3"}}
					srcprefix  = "o_"
					destprefix = "key"
				)

				testMirror(t, srcTC, destTC, mmOpts, sourcekvs, destkvs, srcprefix, destprefix)
			})
		}
	}
}

func TestMakeMirrorWithWatchRev(t *testing.T) {
	testRunner.BeforeTest(t)
	for _, srcTC := range clusterTestCases() {
		for _, destTC := range clusterTestCases() {
			t.Run(fmt.Sprintf("Source=%s/Destination=%s", srcTC.name, destTC.name), func(t *testing.T) {
				if destTC.config.ClientTLS == config.AutoTLS {
					t.Skip("Skipping test for destination cluster with AutoTLS")
				}

				var (
					mmOpts     = config.MakeMirrorOptions{Prefix: "o_", NoDestPrefix: true, Rev: 4}
					sourcekvs  = []testutils.KV{{Key: "o_key1", Val: "val1"}, {Key: "o_key2", Val: "val2"}, {Key: "o_key3", Val: "val3"}, {Key: "o_key4", Val: "val4"}}
					destkvs    = []testutils.KV{{Key: "key3", Val: "val3"}, {Key: "key4", Val: "val4"}}
					srcprefix  = "o_"
					destprefix = "key"
				)

				testMirror(t, srcTC, destTC, mmOpts, sourcekvs, destkvs, srcprefix, destprefix)
			})
		}
	}
}

func testMirror(t *testing.T, srcTC, destTC testCase, mmOpts config.MakeMirrorOptions, sourcekvs, destkvs []testutils.KV, srcprefix, destprefix string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)

	// Source cluster
	src := testRunner.NewCluster(ctx, t, config.WithClusterConfig(srcTC.config))
	defer src.Close()
	srcCli := testutils.MustClient(src.Client())

	// Destination cluster (use different BasePort to avoid collisions)
	destCfg := destTC.config
	dest := testRunner.NewCluster(ctx, t, config.WithClusterConfig(destCfg), WithBasePort(10000))
	defer dest.Close()
	destCli := testutils.MustClient(dest.Client())

	// Start make mirror
	errCh := make(chan error, 1)
	go func() {
		configureMirrorDestTLS(&mmOpts, destCfg.ClientTLS)
		errCh <- srcCli.MakeMirror(ctx, dest.Endpoints(), mmOpts)
	}()
	defer func() {
		// Need to cancel the context to ensure the MakeMirror goroutine is cancelled before catching the error.
		cancel()
		require.NoError(t, <-errCh)
	}()

	// Write to source
	for _, kv := range sourcekvs {
		require.NoError(t, srcCli.Put(ctx, kv.Key, kv.Val, config.PutOptions{}))
	}

	// Source assert
	srcResp, err := srcCli.Get(ctx, srcprefix, config.GetOptions{Prefix: true})
	require.NoError(t, err)
	require.Equal(t, sourcekvs, testutils.KeyValuesFromGetResponse(srcResp))

	// Destination assert
	wCtx, wCancel := context.WithCancel(ctx)
	wch := destCli.Watch(wCtx, destprefix, config.WatchOptions{Revision: 1, Prefix: true})
	defer wCancel()
	destResp, err := testutils.KeyValuesFromWatchChan(wch, len(destkvs), 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, destkvs, destResp)
}
