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
	"encoding/json"
	"fmt"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/pkg/v3/testutil"
)

// NO TLS
func TestV3Curl_MaxStreams_BelowLimit_NoTLS_Small(t *testing.T) {
	testV3CurlMaxStream(t, false, withCfg(*newConfigNoTLS()), withMaxConcurrentStreams(3))
}

func TestV3Curl_MaxStreams_BelowLimit_NoTLS_Medium(t *testing.T) {
	testV3CurlMaxStream(t, false, withCfg(*newConfigNoTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

/*
// There are lots of "device not configured" errors. I suspect it's an issue
// of the project `github.com/creack/pty`. I manually executed the test case
// with 1000 concurrent streams, and confirmed it's working as expected.
// TODO(ahrtr): investigate the test failure in the future.
func TestV3Curl_MaxStreamsNoTLS_BelowLimit_Large(t *testing.T) {
	f, err := setRLimit(10240)
	if err != nil {
		t.Fatal(err)
	}
	testV3CurlMaxStream(t, false, withCfg(*e2e.NewConfigNoTLS()), withMaxConcurrentStreams(1000), withTestTimeout(200*time.Second))
	f()
} */

func TestV3Curl_MaxStreams_ReachLimit_NoTLS_Small(t *testing.T) {
	testV3CurlMaxStream(t, true, withCfg(*newConfigNoTLS()), withMaxConcurrentStreams(3))
}

func TestV3Curl_MaxStreams_ReachLimit_NoTLS_Medium(t *testing.T) {
	testV3CurlMaxStream(t, true, withCfg(*newConfigNoTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

// TLS
func TestV3Curl_MaxStreams_BelowLimit_TLS_Small(t *testing.T) {
	testV3CurlMaxStream(t, false, withCfg(*newConfigTLS()), withMaxConcurrentStreams(3))
}

func TestV3Curl_MaxStreams_BelowLimit_TLS_Medium(t *testing.T) {
	testV3CurlMaxStream(t, false, withCfg(*newConfigTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

func TestV3Curl_MaxStreams_ReachLimit_TLS_Small(t *testing.T) {
	testV3CurlMaxStream(t, true, withCfg(*newConfigTLS()), withMaxConcurrentStreams(3))
}

func TestV3Curl_MaxStreams_ReachLimit_TLS_Medium(t *testing.T) {
	testV3CurlMaxStream(t, true, withCfg(*newConfigTLS()), withMaxConcurrentStreams(100), withTestTimeout(20*time.Second))
}

func testV3CurlMaxStream(t *testing.T, reachLimit bool, opts ...ctlOption) {
	BeforeTest(t)

	// Step 1: generate configuration for creating cluster
	t.Log("Generating configuration for creating cluster.")
	cx := getDefaultCtlCtx(t)
	cx.applyOpts(opts)
	// We must set the `ClusterSize` to 1, otherwise different streams may
	// connect to different members, accordingly it's difficult to test the
	// behavior.
	cx.cfg.clusterSize = 1

	// Step 2: create the cluster
	t.Log("Creating an etcd cluster")
	epc, err := newEtcdProcessCluster(t, &cx.cfg)
	if err != nil {
		t.Fatalf("Failed to start etcd cluster: %v", err)
	}
	cx.epc = epc
	cx.dataDir = epc.procs[0].Config().dataDirPath

	// Step 3: run test
	//	(a) generate ${concurrentNumber} concurrent watch streams;
	//	(b) submit a range request.
	var wg sync.WaitGroup
	concurrentNumber := cx.cfg.MaxConcurrentStreams - 1
	expectedResponse := `"revision":"`
	if reachLimit {
		concurrentNumber = cx.cfg.MaxConcurrentStreams
		expectedResponse = "Operation timed out"
	}
	wg.Add(int(concurrentNumber))
	t.Logf("Running the test, MaxConcurrentStreams: %d, concurrentNumber: %d, expectedResponse: %s\n",
		cx.cfg.MaxConcurrentStreams, concurrentNumber, expectedResponse)
	errCh := make(chan error, concurrentNumber)
	submitConcurrentWatch(cx, int(concurrentNumber), &wg, errCh)
	submitRangeAfterConcurrentWatch(cx, expectedResponse)

	// Step 4: check the watch errors. Note that we only check the watch error
	// before closing cluster. Once we close the cluster, the watch must run
	// into error, and we should ignore them by then.
	t.Log("Checking watch error.")
	select {
	case werr := <-errCh:
		t.Fatal(werr)
	default:
	}

	// Step 5: Close the cluster
	t.Log("Closing test cluster...")
	assert.NoError(t, epc.Close())
	t.Log("Closed test cluster")

	// Step 6: Waiting all watch goroutines to exit.
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		wg.Wait()
	}()

	timeout := cx.getTestTimeout()
	t.Logf("Waiting test case to finish, timeout: %s", timeout)
	select {
	case <-time.After(timeout):
		testutil.FatalStack(t, fmt.Sprintf("test timed out after %v", timeout))
	case <-donec:
		t.Log("All watch goroutines exited.")
	}

	t.Log("testV3CurlMaxStream done!")
}

func submitConcurrentWatch(cx ctlCtx, number int, wgDone *sync.WaitGroup, errCh chan<- error) {
	watchData, err := json.Marshal(&pb.WatchRequest_CreateRequest{
		CreateRequest: &pb.WatchCreateRequest{
			Key: []byte("foo"),
		},
	})
	if err != nil {
		cx.t.Fatal(err)
	}

	var wgSchedule sync.WaitGroup
	wgSchedule.Add(number)
	for i := 0; i < number; i++ {
		go func(i int) {
			wgSchedule.Done()
			defer wgDone.Done()
			if err := cURLPost(cx.epc, cURLReq{endpoint: "/v3/watch", value: string(watchData), expected: `"revision":"`}); err != nil {
				werr := fmt.Errorf("testV3CurlMaxStream watch failed: %d, error: %v", i, err)
				cx.t.Error(werr)
				errCh <- werr
			}
		}(i)
	}
	// make sure all goroutines have already been scheduled.
	wgSchedule.Wait()
	// We need to make sure all watch streams have already been created.
	// For simplicity, we just sleep 3 second. We may consider improving
	// it in the future.
	time.Sleep(3 * time.Second)
}

func submitRangeAfterConcurrentWatch(cx ctlCtx, expectedValue string) {
	rangeData, err := json.Marshal(&pb.RangeRequest{
		Key: []byte("foo"),
	})
	if err != nil {
		cx.t.Fatal(err)
	}

	cx.t.Log("Submitting range request...")
	if err := cURLPost(cx.epc, cURLReq{endpoint: "/v3/kv/range", value: string(rangeData), expected: expectedValue, timeout: 5}); err != nil {
		cx.t.Fatalf("testV3CurlMaxStream get failed, error: %v", err)
	}
	cx.t.Log("range request done")
}

// setRLimit sets the open file limitation, and return a function which
// is used to reset the limitation.
func setRLimit(nofile uint64) (func() error, error) {
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return nil, fmt.Errorf("failed to get open file limit, error: %v", err)
	}

	var wLimit syscall.Rlimit
	wLimit.Max = nofile
	wLimit.Cur = nofile
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &wLimit); err != nil {
		return nil, fmt.Errorf("failed to set max open file limit, %v", err)
	}

	return func() error {
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
			return fmt.Errorf("failed reset max open file limit, %v", err)
		}
		return nil
	}, nil
}
