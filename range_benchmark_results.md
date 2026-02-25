# Range Benchmark Results

## Setup

- etcd single node on localhost (port 12379)
- 100-byte values per key
- 10 iterations per mode
- Paginated unary uses page-size=10000
- Default `MaxRequestBytes` (1.5 MB) for stream chunk sizing
- All modes accumulate full results in client memory for fair comparison
- Client peak memory: max of `runtime.ReadMemStats` HeapInuse sampled at 1ms intervals
- Server peak memory: max of `go_memstats_heap_inuse_bytes` from /metrics sampled at 50ms intervals

## Raw Data

| Keys | Mode | Avg Latency | Client Mem Max | Server Mem Max | Throughput |
|---|---|---|---|---|---|
| **100k** | Stream (count once) | 0.10s | 38.06 MB | 52.08 MB | 117.19 MB/s |
| **100k** | Single-shot unary | 0.10s | 53.99 MB | 109.99 MB | 113.86 MB/s |
| **100k** | Paginated unary (10k) | 0.15s | 38.06 MB | 52.08 MB | 90.03 MB/s |
| **100k** | Stream (count always) | 0.13s | 38.53 MB | 51.05 MB | 87.72 MB/s |
| | | | | | |
| **500k** | Stream (count once) | 0.64s | 140.64 MB | 190.14 MB | 97.93 MB/s |
| **500k** | Single-shot unary | 0.60s | 252.97 MB | 498.27 MB | 98.27 MB/s |
| **500k** | Paginated unary (10k) | 1.68s | 139.53 MB | 179.40 MB | 33.24 MB/s |
| **500k** | Stream (count always) | 1.95s | 140.91 MB | 175.16 MB | 33.92 MB/s |
| | | | | | |
| **1M** | Stream (count once) | 1.37s | 271.26 MB | 347.91 MB | 90.08 MB/s |
| **1M** | Single-shot unary | 1.39s | 493.61 MB | 999.84 MB | 88.95 MB/s |
| **1M** | Paginated unary (10k) | 5.44s | 269.40 MB | 338.61 MB | 21.70 MB/s |
| **1M** | Stream (count always) | 6.78s | 269.77 MB | 328.54 MB | 18.63 MB/s |
| | | | | | |
| **2M** | Stream (count once) | 2.76s | — | — | 85.54 MB/s |
| **2M** | Single-shot unary | 2.79s | — | — | 84.69 MB/s |
| **2M** | Paginated unary (10k) | 19.21s | — | — | 12.31 MB/s |
| **2M** | Stream (count always) | 23.27s | — | — | 10.14 MB/s |

*2M memory data omitted (from earlier run without accumulation fix).*

## Latency scaling (vs Stream count-once)

| Keys | Single-shot | Paginated | Stream count-always |
|---|---|---|---|
| 100k | 1.0x | 1.5x slower | 1.3x slower |
| 500k | 0.9x | 2.6x slower | 3.0x slower |
| 1M | 1.0x | 4.0x slower | 4.9x slower |
| 2M | 1.0x | 7.0x slower | 8.4x slower |

The latency gap widens superlinearly with key count. Paginated pays per-RPC overhead on every page (100 RPCs at 1M keys, 200 at 2M). Stream count-always pays an O(n) b-tree scan on every chunk.

## Client memory scaling

When all modes accumulate full results in client memory:

| Keys | Stream (count once) | Single-shot unary | Reduction |
|---|---|---|---|
| 100k | 38.06 MB | 53.99 MB | 1.4x less |
| 500k | 140.64 MB | 252.97 MB | 1.8x less |
| 1M | 271.26 MB | 493.61 MB | 1.8x less |

Stream and paginated have nearly identical client memory (both accumulate the same total data). The ~1.4-1.8x reduction vs single-shot comes from gRPC message buffering overhead — single-shot materializes one massive proto message, while stream/paginated process smaller chunks.

Streaming clients that process-and-discard chunks (instead of accumulating) would see flat ~20 MB regardless of dataset size.

## Server memory scaling

| Keys | Stream (count once) | Single-shot unary | Reduction |
|---|---|---|---|
| 100k | 52.08 MB | 109.99 MB | 2.1x less |
| 500k | 190.14 MB | 498.27 MB | 2.6x less |
| 1M | 347.91 MB | 999.84 MB | 2.9x less |

Server-side memory savings are more significant. Single-shot must materialize the entire response in server memory before sending (~1 GB at 1M keys), while stream sends chunks incrementally. The reduction factor grows with dataset size.

## Throughput scaling

| Keys | Stream (count once) | Single-shot | Paginated | Stream (count always) |
|---|---|---|---|---|
| 100k | 117.19 MB/s | 113.86 MB/s | 90.03 MB/s | 87.72 MB/s |
| 500k | 97.93 MB/s | 98.27 MB/s | 33.24 MB/s | 33.92 MB/s |
| 1M | 90.08 MB/s | 88.95 MB/s | 21.70 MB/s | 18.63 MB/s |
| 2M | 85.54 MB/s | 84.69 MB/s | 12.31 MB/s | 10.14 MB/s |

Stream count-once and single-shot maintain ~85-117 MB/s across all sizes. Paginated and stream count-always degrade sharply as key count grows.

## Key takeaways

1. **Stream count-once matches single-shot on latency and throughput** while using ~1.8x less client memory and ~2.9x less server memory at 1M keys. The server memory advantage grows with scale.
2. **Server memory is the biggest win**: at 1M keys, single-shot uses ~1 GB server-side vs ~348 MB for stream. This matters for multi-tenant clusters serving concurrent large ranges.
3. **Paginated unary has no advantages**: slower than stream at every size (1.5x at 100k, 7x at 2M), and the gap grows with scale. Per-RPC overhead compounds across hundreds of pages.
4. **Count-always is expensive**: computing total count on every chunk is an O(n) b-tree scan. At 1M+ keys this makes stream count-always even slower than paginated. Counting once at the start is critical.
5. **Process-and-discard streaming** would show even larger memory savings (flat ~20 MB vs linearly scaling), but this benchmark forces accumulation for fair comparison with unary.
