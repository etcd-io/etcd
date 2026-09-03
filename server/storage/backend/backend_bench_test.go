// Copyright 2015 The etcd Authors
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

package backend_test

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/storage/backend"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

// snapshotCompactTxMaxBytesFlag is a comma-separated list of bbolt.Compact
// txMaxSize values (in bytes) to sweep. Each value becomes a sub-benchmark.
// The empty string (default) uses the package default.
//
// Test-binary flags must appear after `-args`, otherwise `go test` treats
// them as unknown and stops parsing the command line:
//
//	go test -run=^$ -bench=BenchmarkBackendSnapshotDefragmented \
//	    ./server/storage/backend/... -args \
//	    -txmax=4194304,8388608,16777216,67108864
var snapshotCompactTxMaxBytesFlag = flag.String(
	"txmax", "",
	"comma-separated list of bbolt.Compact txMaxSize values (bytes) to sweep; empty = use package default",
)

var snapshotDefragmentedBenchKeys = flag.Int(
	"snapshot-defrag-bench-keys", 100_000,
	"number of keys to populate before each BenchmarkBackendSnapshotDefragmented iteration",
)

var snapshotDefragmentedBenchValueSize = flag.Int(
	"snapshot-defrag-bench-value-size", 256,
	"value size (bytes) used to populate keys before each BenchmarkBackendSnapshotDefragmented iteration",
)

var snapshotDefragmentedBenchDeleteRatio = flag.Float64(
	"snapshot-defrag-bench-delete-ratio", 0.9,
	"fraction of keys to delete after populate; controls how many freed pages the source DB has",
)

func BenchmarkBackendPut(b *testing.B) {
	backend, _ := betesting.NewTmpBackend(b, 100*time.Millisecond, 10000)
	defer betesting.Close(b, backend)

	// prepare keys
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = make([]byte, 64)
		_, err := rand.Read(keys[i])
		require.NoError(b, err)
	}
	value := make([]byte, 128)
	_, err := rand.Read(value)
	require.NoError(b, err)

	batchTx := backend.BatchTx()

	batchTx.Lock()
	batchTx.UnsafeCreateBucket(schema.Test)
	batchTx.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batchTx.Lock()
		batchTx.UnsafePut(schema.Test, keys[i], value)
		batchTx.Unlock()
	}
}

// BenchmarkBackendSnapshotDefragmented measures the end-to-end cost of
// producing a defragmented snapshot. The benchmark populates the backend
// once, deletes a configurable fraction of its keys (so bbolt.Compact has
// work to do), then runs a sub-benchmark for each txMax value in the
// -txmax flag — each sub-benchmark invokes SnapshotDefragmented and drains
// it to io.Discard.
//
// Use -txmax to specify one or more comma-separated bbolt.Compact txMaxSize
// values. Use -snapshot-defrag-bench-keys, -snapshot-defrag-bench-value-size,
// and -snapshot-defrag-bench-delete-ratio to vary the workload.
func BenchmarkBackendSnapshotDefragmented(b *testing.B) {
	txMaxValues, err := parseTxMaxList(*snapshotCompactTxMaxBytesFlag)
	require.NoError(b, err)
	numKeys := *snapshotDefragmentedBenchKeys
	valueSize := *snapshotDefragmentedBenchValueSize
	deleteRatio := *snapshotDefragmentedBenchDeleteRatio
	require.GreaterOrEqual(b, deleteRatio, 0.0, "delete ratio must be in [0, 1]")
	require.LessOrEqual(b, deleteRatio, 1.0, "delete ratio must be in [0, 1]")

	be, _ := betesting.NewTmpBackend(b, time.Hour, 10000)
	defer betesting.Close(b, be)

	value := make([]byte, valueSize)
	_, err = rand.Read(value)
	require.NoError(b, err)

	b.Logf("populating: keys=%d value=%dB ...", numKeys, valueSize)
	popStart := time.Now()
	tx := be.BatchTx()
	tx.Lock()
	tx.UnsafeCreateBucket(schema.Test)
	for i := 0; i < numKeys; i++ {
		tx.UnsafePut(schema.Test, []byte(fmt.Sprintf("k-%010d", i)), value)
	}
	tx.Unlock()
	be.ForceCommit()
	b.Logf("populate took %s (live size=%dMB)", time.Since(popStart), be.Size()/1e6)

	numDeleted := int(float64(numKeys) * deleteRatio)
	b.Logf("deleting %d/%d keys (ratio=%.2f) ...", numDeleted, numKeys, deleteRatio)
	delStart := time.Now()
	tx = be.BatchTx()
	tx.Lock()
	for i := 0; i < numDeleted; i++ {
		tx.UnsafeDelete(schema.Test, []byte(fmt.Sprintf("k-%010d", i)))
	}
	tx.Unlock()
	be.ForceCommit()
	b.Logf("delete took %s (live size=%dMB, in-use=%dMB)",
		time.Since(delStart), be.Size()/1e6, be.SizeInUse()/1e6)

	for _, txMax := range txMaxValues {
		txMax := txMax
		b.Run(fmt.Sprintf("txmax=%d", txMax), func(b *testing.B) {
			b.ReportMetric(float64(txMax), "txmax-bytes")
			b.ReportMetric(float64(numKeys), "populated-keys")
			b.ReportMetric(float64(valueSize), "value-bytes")
			b.ReportMetric(deleteRatio, "delete-ratio")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				snap, err := backend.SnapshotDefragmentedWithTxMaxBytesForTest(be, txMax)
				require.NoError(b, err)
				n, err := snap.WriteTo(io.Discard)
				require.NoError(b, err)
				require.NoError(b, snap.Close())
				if i == 0 {
					b.ReportMetric(float64(n), "snapshot-bytes")
				}
			}
		})
	}
}

func parseTxMaxList(s string) ([]int64, error) {
	if s == "" {
		return []int64{backend.DefaultSnapshotCompactTxMaxBytesForTest()}, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid txmax value %q: %w", p, err)
		}
		out = append(out, v)
	}
	return out, nil
}
