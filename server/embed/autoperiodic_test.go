// Copyright 2026 The etcd Authors
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

package embed

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3compactor"
)

// TestAutoPeriodicCompactionMode drives ~4x the quota through a single key with
// the workload-adaptive auto-periodic mode at a realistic write rate. The mode
// only compacts (never defrags, matching the periodic and revision modes), yet
// the backend stays under quota: compaction frees pages to the freelist that
// subsequent writes reuse, so under continuous write+compaction the file
// stabilizes at the working-set high-water mark instead of growing to the quota.
// (A defrag would additionally shrink the file to the live-data size, but is not
// needed to stay under quota.)
func TestAutoPeriodicCompactionMode(t *testing.T) {
	if testing.Short() {
		t.Skip("writes ~1 GiB")
	}

	// Short, aggressive tuning for the test: sample often, project a short
	// distance, steer to 50% of quota so the reclaim has ample headroom.
	defer restore(&v3compactor.AutoPeriodicSampleInterval, 100*time.Millisecond)()
	defer restore(&v3compactor.AutoPeriodicLeadTime, 1500*time.Millisecond)()
	defer restore(&v3compactor.AutoPeriodicCooldown, 700*time.Millisecond)()
	defer restore(&v3compactor.AutoPeriodicHighWater, 0.5)()

	const (
		quota      = 256 << 20
		valueSize  = 256 << 10
		writeTotal = 4000
		maxReq     = 4 << 20
	)

	cfg := NewConfig()
	applyTestURLConfig(cfg, newConfigTestURLs())
	cfg.Dir = t.TempDir()
	cfg.QuotaBackendBytes = quota
	cfg.MaxRequestBytes = maxReq
	cfg.AutoCompactionMode = CompactorModeAutoPeriodic
	cfg.AutoCompactionRetention = "5"
	cfg.LogLevel = "error"

	e, err := StartEtcd(cfg)
	require.NoError(t, err)
	defer e.Close()
	select {
	case <-e.Server.ReadyNotify():
	case <-time.After(30 * time.Second):
		t.Fatal("etcd not ready")
	}

	cli := v3client.New(e.Server)
	defer cli.Close()
	ctx := t.Context()

	rng := rand.New(rand.NewSource(1))
	val := make([]byte, valueSize)

	var (
		done       int
		tripErr    error
		maxSize    int64
		maxLatency time.Duration
	)
	for i := 1; i <= writeTotal; i++ {
		rng.Read(val)
		time.Sleep(5 * time.Millisecond)
		start := time.Now()
		_, perr := cli.Put(ctx, "k", string(val))
		lat := time.Since(start)
		if perr != nil {
			tripErr = perr
			break
		}
		done = i
		if lat > maxLatency {
			maxLatency = lat
		}
		if s := e.Server.Backend().Size(); s > maxSize {
			maxSize = s
		}
	}

	t.Logf("\n===== bbolt (quota=%s, wrote %d/%d, realistic rate) =====", mib256(quota), done, writeTotal)
	t.Logf("  data pushed through key : %s", mib256(int64(done)*valueSize))
	t.Logf("  max Size() observed     : %s (quota %s)", mib256(maxSize), mib256(quota))
	t.Logf("  final Size()            : %s", mib256(e.Server.Backend().Size()))
	t.Logf("  max single-write latency: %s", maxLatency)
	if tripErr != nil {
		t.Logf("  RESULT: TRIPPED after %d writes: %v", done, tripErr)
	} else {
		t.Logf("  RESULT: survived all %d writes (no NOSPACE)", writeTotal)
	}

	// Compaction only (no defrag) keeps the backend under quota: freed pages go
	// to the freelist and are reused by subsequent writes.
	require.NoError(t, tripErr, "auto-periodic should keep the backend below quota under realistic load")
	require.Equal(t, writeTotal, done)
	require.Less(t, maxSize, int64(quota), "size should stay under quota")
}

func mib256(n int64) string { return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20)) }

// restore sets *p to v and returns a func that restores the old value (for
// overriding package tuning vars in a test).
func restore[T any](p *T, v T) func() {
	old := *p
	*p = v
	return func() { *p = old }
}
