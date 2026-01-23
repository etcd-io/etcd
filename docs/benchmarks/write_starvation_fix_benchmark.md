# Write Starvation Fix Benchmark Results

## Summary

This document provides benchmark methodology and analysis for the fix in PR that changes `rangeEvents()` to use `ConcurrentReadTx` instead of `ReadTx` in `server/storage/mvcc/watchable_store.go`.

## Problem Description

**Issue**: Write starvation occurs when `syncWatchers` calls `rangeEvents()` to read large historical event ranges. The original implementation used `ReadTx()` which holds `readTx.mu.RLock()`, blocking `batchTxBuffered.Unlock()` that needs `readTx.mu.Lock()` for writeback.

**Impact**: Under heavy historical watch load, writes can be starved causing:
- Increased write latencies (p99 spikes)
- Cluster timeouts
- Leader election issues due to delayed heartbeats

**Fix**: Replace `b.ReadTx()` with `b.ConcurrentReadTx()` in `rangeEvents()`. `ConcurrentReadTx` operates on a read-only snapshot and doesn't block write commits.

## Benchmark Methodology

### Environment Setup

```bash
# Build etcd and benchmark tool
cd /path/to/etcd
go build -o bin/etcd ./server
go build -o bin/benchmark ./tools/benchmark

# Start single-node etcd cluster
./bin/etcd --data-dir /tmp/etcd-data \
  --listen-client-urls http://localhost:2379 \
  --advertise-client-urls http://localhost:2379
```

### Test Scenario: Heavy Watch Load with Concurrent Writes

This scenario simulates the write starvation condition:
1. Create many watchers across multiple streams
2. Generate historical events for watchers to sync
3. Issue concurrent put operations
4. Measure put latency during watch sync

```bash
# Benchmark command
./bin/benchmark --endpoints=localhost:2379 \
  watch \
  --streams=50 \
  --watch-per-stream=100 \
  --watched-key-total=100 \
  --put-total=5000 \
  --put-rate=500 \
  --key-space-size=100
```

### Metrics to Compare

| Metric | Description |
|--------|-------------|
| Put Avg Latency | Average time for put operations |
| Put P99 Latency | 99th percentile put latency |
| Put Throughput | Puts per second achieved |
| Watch Event Latency | Time from put to watch notification |

## Expected Results

### Before Fix (using `ReadTx`)

Under heavy historical watch load:
- Put P99 latency: **High spikes** (100ms - seconds)
- Put throughput: **Degraded** (may drop significantly)
- Potential timeouts when reading large event ranges

### After Fix (using `ConcurrentReadTx`)

With the fix applied:
- Put P99 latency: **Stable** (sub-10ms typical)
- Put throughput: **Sustained** at target rate
- No write starvation even with concurrent large range reads

## Technical Analysis

The fix works because:

1. **`ReadTx`**: Acquires `readTx.mu.RLock()` which blocks `batchTx` commits that need `readTx.mu.Lock()` for buffer writeback

2. **`ConcurrentReadTx`**: 
   - Operates on a copy/snapshot of the read buffer
   - Uses `txWg` for coordination instead of blocking the write path
   - Allows commits to proceed while reads are in progress

```go
// Before (causes starvation)
tx := b.ReadTx()  // Holds readTx.mu.RLock()

// After (no starvation)
tx := b.ConcurrentReadTx()  // Uses snapshot, doesn't block writes
```

## Reproduction Commands

To reproduce and verify the fix:

```bash
# 1. Checkout main branch (without fix)
git checkout main
go build -o bin/etcd-main ./server

# 2. Run benchmark against main
./bin/etcd-main --data-dir /tmp/etcd-main &
./bin/benchmark --endpoints=localhost:2379 watch \
  --streams=50 --watch-per-stream=100 --put-total=5000
# Record results, then stop etcd

# 3. Checkout fix branch
git checkout fix/syncwatchers-write-starvation
go build -o bin/etcd-fix ./server

# 4. Run benchmark against fix
./bin/etcd-fix --data-dir /tmp/etcd-fix &
./bin/benchmark --endpoints=localhost:2379 watch \
  --streams=50 --watch-per-stream=100 --put-total=5000
# Compare results
```

## Conclusion

The change from `ReadTx` to `ConcurrentReadTx` in `rangeEvents()` eliminates write starvation under heavy historical watch load by allowing write commits to proceed without being blocked by ongoing large range reads.
