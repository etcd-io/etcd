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

package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/stringutil"
	"go.etcd.io/etcd/tests/v3/framework/e2e"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReproduce19406 reproduces the issue: https://github.com/etcd-io/etcd/issues/19406
func TestReproduce19406(t *testing.T) {
	e2e.BeforeTest(t)

	compactionBatchLimit := 1
	compactionSleepInterval := 100 * time.Millisecond
	ctx := context.TODO()

	clus, cerr := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithGoFailEnabled(true),
		e2e.WithCompactionBatchLimit(compactionBatchLimit),
		e2e.WithCompactionSleepInterval(compactionSleepInterval),
	)
	require.NoError(t, cerr)
	t.Cleanup(func() { require.NoError(t, clus.Stop()) })

	// produce some data
	cli := newClient(t, clus.EndpointsGRPC(), e2e.ClientConfig{})
	valueSize := 10
	var latestRevision int64

	for i := 0; i <= 50; i++ {
		resp, err := cli.Put(ctx, fmt.Sprintf("%d", i), stringutil.RandString(uint(valueSize)))
		require.NoError(t, err)
		latestRevision = resp.Header.Revision
	}

	// Sleep for compactionSleepInterval to simulate a single iteration of compaction lasting at least this duration.
	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "compactAfterAcquiredBatchTxLock",
		fmt.Sprintf(`sleep("%s")`, compactionSleepInterval)))

	// start compaction
	compactionFinished := make(chan struct{}) // Channel to signal completion of compaction
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done() // Decrement counter when done
		defer close(compactionFinished)

		t.Log("start compaction...")
		_, err := cli.Compact(ctx, latestRevision, clientv3.WithCompactPhysical())
		assert.NoError(t, err)
		t.Log("finished compaction...")
	}()

	// Compaction runs in batches. During each batch, it acquires a lock, releases it at the end,
	// and then waits for a compactionSleepInterval before starting the next batch. This pause
	// allows PUT requests to be processed.
	//
	// Therefore, the expected PUT request latency should fall between
	// [each batch execution time, each batch execution time + compactionSleepInterval],
	// which in this case is [compactionSleepInterval, compactionSleepInterval × 2].
	numOfWriters := 10
	for i := 1; i <= numOfWriters; i++ {
		wg.Add(1) // Increment WaitGroup counter

		go func() {
			defer wg.Done() // Decrement counter when done
			checkPutLatencyDuringCompaction(ctx, t, cli, compactionFinished, compactionSleepInterval*2)
		}()
	}
	wg.Wait()
}

func checkPutLatencyDuringCompaction(ctx context.Context, t *testing.T, cli *clientv3.Client, compactionFinished <-chan struct{}, putLatencyThreshold time.Duration) {
	for {
		select {
		case <-compactionFinished:
			return // Exit once compaction is complete
		default:
			start := time.Now()
			_, err := cli.Put(ctx, "foo", "far")
			if err != nil {
				t.Fatal(err)
			}

			if latency := time.Since(start); latency > putLatencyThreshold {
				t.Fatalf("Test failed: PUT latency exceeded %s (actual: %s)", putLatencyThreshold, latency)
			}
		}
	}
}
