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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

// TestCurlV3_MaxStreams_BelowLimit_NoTLS_Small tests no TLS
func TestCurlV3_MaxStreams_BelowLimit_NoTLS_Small(t *testing.T) {
	testCurlV3MaxStream(t, false, withCfg(*e2e.NewConfigNoTLS()), withMaxConcurrentStreams(3))
}

func TestCurlV3_MaxStreams_BelowLimit_NoTLS_Medium(t *testing.T) {
	testCurlV3MaxStream(t, false, withCfg(*e2e.NewConfigNoTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

func TestCurlV3_MaxStreamsNoTLS_BelowLimit_Large(t *testing.T) {
	f, err := setRLimit(10240)
	require.NoError(t, err)
	defer f()
	testCurlV3MaxStream(t, false, withCfg(*e2e.NewConfigNoTLS()), withMaxConcurrentStreams(1000), withTestTimeout(200*time.Second))
}

func TestCurlV3_MaxStreams_ReachLimit_NoTLS_Small(t *testing.T) {
	testCurlV3MaxStream(t, true, withCfg(*e2e.NewConfigNoTLS()), withMaxConcurrentStreams(3))
}

func TestCurlV3_MaxStreams_ReachLimit_NoTLS_Medium(t *testing.T) {
	testCurlV3MaxStream(t, true, withCfg(*e2e.NewConfigNoTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

// TestCurlV3_MaxStreams_BelowLimit_TLS_Small tests with TLS
func TestCurlV3_MaxStreams_BelowLimit_TLS_Small(t *testing.T) {
	testCurlV3MaxStream(t, false, withCfg(*e2e.NewConfigTLS()), withMaxConcurrentStreams(3))
}

func TestCurlV3_MaxStreams_BelowLimit_TLS_Medium(t *testing.T) {
	testCurlV3MaxStream(t, false, withCfg(*e2e.NewConfigTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

func TestCurlV3_MaxStreams_ReachLimit_TLS_Small(t *testing.T) {
	testCurlV3MaxStream(t, true, withCfg(*e2e.NewConfigTLS()), withMaxConcurrentStreams(3))
}

func TestCurlV3_MaxStreams_ReachLimit_TLS_Medium(t *testing.T) {
	testCurlV3MaxStream(t, true, withCfg(*e2e.NewConfigTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

func testCurlV3MaxStream(t *testing.T, reachLimit bool, opts ...ctlOption) {
	e2e.BeforeTest(t)

	// Step 1: generate configuration for creating cluster
	t.Log("Generating configuration for creating cluster.")
	cx := getDefaultCtlCtx(t)
	cx.applyOpts(opts)
	// We must set the `ClusterSize` to 1, otherwise different streams may
	// connect to different members, accordingly it's difficult to test the
	// behavior.
	cx.cfg.ClusterSize = 1

	// Step 2: create the cluster
	t.Log("Creating an etcd cluster")
	epc, err := e2e.NewEtcdProcessCluster(t.Context(), t, e2e.WithConfig(&cx.cfg))
	require.NoErrorf(t, err, "Failed to start etcd cluster")
	cx.epc = epc
	cx.dataDir = epc.Procs[0].Config().DataDirPath

	// Step 3: run test
	//	(a) generate ${concurrentNumber} concurrent watch streams;
	//	(b) submit a range request.
	var wg sync.WaitGroup
	concurrentNumber := cx.cfg.ServerConfig.MaxConcurrentStreams - 1
	expectedResponse := `"revision":"`
	if reachLimit {
		concurrentNumber = cx.cfg.ServerConfig.MaxConcurrentStreams
		expectedResponse = "Operation timed out"
	}
	wg.Add(int(concurrentNumber))
	t.Logf("Running the test, MaxConcurrentStreams: %d, concurrentNumber: %d, expected range's response: %s\n",
		cx.cfg.ServerConfig.MaxConcurrentStreams, concurrentNumber, expectedResponse)

	closeServerCh := make(chan struct{})
	submitConcurrentWatch(cx, int(concurrentNumber), &wg, closeServerCh)
	submitRangeAfterConcurrentWatch(cx, expectedResponse)

	// Step 4: Close the cluster
	t.Log("Closing test cluster...")
	close(closeServerCh)
	require.NoError(t, epc.Close())
	t.Log("Closed test cluster")

	// Step 5: Waiting all watch goroutines to exit.
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		wg.Wait()
	}()

	timeout := cx.getTestTimeout()
	t.Logf("Waiting test case to finish, timeout: %s", timeout)
	select {
	case <-time.After(timeout):
		testutil.FatalStack(t, fmt.Sprintf("test timed out after %v", timeout))
	case <-doneCh:
		t.Log("All watch goroutines exited.")
	}

	t.Log("testCurlV3MaxStream done!")
}

func submitConcurrentWatch(cx ctlCtx, number int, wgDone *sync.WaitGroup, closeCh chan struct{}) {
	watchData, err := json.Marshal(&pb.WatchRequest_CreateRequest{
		CreateRequest: &pb.WatchCreateRequest{
			Key: []byte("foo"),
		},
	})
	require.NoError(cx.t, err)

	var wgSchedule sync.WaitGroup

	createWatchConnection := func() error {
		cluster := cx.epc
		member := cluster.Procs[rand.Intn(cluster.Cfg.ClusterSize)]
		curlReq := e2e.CURLReq{Endpoint: "/v3/watch", Value: string(watchData)}

		args := e2e.CURLPrefixArgsCluster(cluster.Cfg, member, "POST", curlReq)
		proc, err := e2e.SpawnCmd(args, nil)
		if err != nil {
			return fmt.Errorf("failed to spawn: %w", err)
		}
		defer proc.Stop()

		// make sure that watch request has been created
		expectedLine := `"created":true}}`
		_, lerr := proc.ExpectWithContext(context.TODO(), expect.ExpectedResponse{Value: expectedLine})
		if lerr != nil {
			return fmt.Errorf("%v %w (expected %q). Try EXPECT_DEBUG=TRUE", args, lerr, expectedLine)
		}

		wgSchedule.Done()

		// hold the connection and wait for server shutdown
		perr := proc.Close()

		// curl process will return
		select {
		case <-closeCh:
		default:
			// perr could be nil.
			return fmt.Errorf("unexpected connection close before server closes: %w", perr)
		}
		return nil
	}

	testutils.ExecuteWithTimeout(cx.t, cx.getTestTimeout(), func() {
		wgSchedule.Add(number)

		for i := 0; i < number; i++ {
			go func(i int) {
				defer wgDone.Done()

				require.NoErrorf(cx.t, createWatchConnection(), "testCurlV3MaxStream watch failed: %d", i)
			}(i)
		}

		// make sure all goroutines have already been scheduled.
		wgSchedule.Wait()
	})
}

func submitRangeAfterConcurrentWatch(cx ctlCtx, expectedValue string) {
	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: []byte("foo"),
	})
	require.NoError(cx.t, err)

	cx.t.Log("Submitting range request...")
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/kv/range", Value: string(rangeData), Expected: expect.ExpectedResponse{Value: expectedValue}, Timeout: 5}); err != nil {
		require.ErrorContains(cx.t, err, expectedValue)
	}
	cx.t.Log("range request done")
}

// setRLimit sets the open file limitation, and return a function which
// is used to reset the limitation.
func setRLimit(nofile uint64) (func() error, error) {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return nil, fmt.Errorf("failed to get open file limit, error: %w", err)
	}

	var wLimit syscall.Rlimit
	wLimit.Max = nofile
	wLimit.Cur = nofile
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &wLimit); err != nil {
		return nil, fmt.Errorf("failed to set max open file limit, %w", err)
	}

	return func() error {
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
			return fmt.Errorf("failed reset max open file limit, %w", err)
		}
		return nil
	}, nil
}
