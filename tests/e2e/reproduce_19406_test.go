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

	"github.com/stretchr/testify/require"
)

// TestReproduce19406 reproduces the issue: https://github.com/etcd-io/etcd/issues/19406
func TestReproduce19406(t *testing.T) {
	e2e.BeforeTest(t)

	compactionBatchLimit := 1

	ctx := context.TODO()
	CompactionSleepInterval := 100 * time.Millisecond

	clus, cerr := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithGoFailEnabled(true),
		e2e.WithCompactionBatchLimit(compactionBatchLimit),
		e2e.WithCompactionSleepInterval(CompactionSleepInterval),
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

	// sleep for CompactionSleepInterval
	require.NoError(t, clus.Procs[0].Failpoints().SetupHTTP(ctx, "compactAfterAcquiredBatchTxLock",
		fmt.Sprintf(`sleep("%s")`, CompactionSleepInterval)))

	// start compaction
	compactionFinished := make(chan struct{}) // Channel to signal completion of compaction
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done() // Decrement counter when done
		defer close(compactionFinished)
		t.Log("start compaction...")
		_, err := cli.Compact(context.Background(), latestRevision, clientv3.WithCompactPhysical())
		require.NoError(t, err)
		t.Log("finished compaction...")
	}()

	// Compaction runs in batches. During each batch, it acquires a lock, releases it at the end,
	// and then waits for a CompactionSleepInterval before starting the next batch. This pause
	// allows PUT requests to be processed.
	//
	// Therefore, the expected PUT request latency should fall between
	// [each batch execution time, each batch execution time + CompactionSleepInterval],
	// which in this case is [CompactionSleepInterval, CompactionSleepInterval × 2].
	numOfWriters := 10
	putLatencyLimit := CompactionSleepInterval * 2
	for i := 1; i <= numOfWriters; i++ {
		wg.Add(1) // Increment WaitGroup counter

		go func() {
			defer wg.Done() // Decrement counter when done
			for {
				select {
				case <-compactionFinished: // compaction finished
					return
				default:
					start := time.Now()
					_, err := cli.Put(ctx, "foo", "far")
					require.NoError(t, err)

					if time.Since(start) > putLatencyLimit {
						t.Fatalf("Test failed: put latency is larger than %s", putLatencyLimit)
					}
				}
			}
		}()
	}
	wg.Wait()
}
