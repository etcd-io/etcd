# Range Benchmark Results

## Setup

- etcd single node on localhost
- All modes accumulate full results in client memory for fair comparison
- Client peak memory: max `HeapInuse` sampled at 1ms via `runtime.ReadMemStats`
- Server peak memory: max `go_memstats_heap_inuse_bytes` from `/metrics` sampled at 50ms

## Small values (100-byte values, 1 client, 10 iterations)

All comparisons relative to **Stream (count once)**.

### 100k keys (~10 MB total data)

| Mode | Latency | Client Mem | Server Mem | Throughput |
|---|---|---|---|---|
| Stream (count once) | 0.10s | 38 MB | 52 MB | 117 MB/s |
| Single-shot unary | 1.0x | 1.4x more | 2.1x more | 1.0x |
| Paginated unary (10k) | 1.5x slower | 1.0x | 1.0x | 1.3x lower |
| Stream (count always) | 1.3x slower | 1.0x | 1.0x | 1.3x lower |

### 500k keys (~50 MB total data)

| Mode | Latency | Client Mem | Server Mem | Throughput |
|---|---|---|---|---|
| Stream (count once) | 0.64s | 141 MB | 190 MB | 98 MB/s |
| Single-shot unary | 0.9x | 1.8x more | 2.6x more | 1.0x |
| Paginated unary (10k) | 2.6x slower | 1.0x | 0.9x | 2.9x lower |
| Stream (count always) | 3.0x slower | 1.0x | 0.9x | 2.9x lower |

### 1M keys (~100 MB total data)

| Mode | Latency | Client Mem | Server Mem | Throughput |
|---|---|---|---|---|
| Stream (count once) | 1.37s | 271 MB | 348 MB | 90 MB/s |
| Single-shot unary | 1.0x | 1.8x more | 2.9x more | 1.0x |
| Paginated unary (10k) | 4.0x slower | 1.0x | 1.0x | 4.2x lower |
| Stream (count always) | 4.9x slower | 1.0x | 0.9x | 4.8x lower |

### 2M keys (~200 MB total data)

| Mode | Latency | Throughput |
|---|---|---|
| Stream (count once) | 2.76s | 86 MB/s |
| Single-shot unary | 1.0x | 1.0x |
| Paginated unary (10k) | 7.0x slower | 6.9x lower |
| Stream (count always) | 8.4x slower | 8.4x lower |

## Large values (10k keys x 100KB values, ~1 GB total data)

### 1 client, 1 iteration

| Mode | Latency | Client Mem | Server Mem | Throughput |
|---|---|---|---|---|
| Stream (count once) | 0.48s | 1.01 GB | 1.03 GB | 1.99 GB/s |
| Single-shot unary | 2.9x slower | 2.9x more | 2.4x more | 2.8x lower |

### 5 clients, 50 iterations

| Mode | P50 Latency | P90 Latency | Client Mem | Server Mem | Throughput |
|---|---|---|---|---|---|
| Stream | 1.81s | 8.33s | 5.01 GB | 1.06 GB | 383 MB/s |
| Single-shot | 15.6x slower | 5.1x slower | 2.8x more | 15.8x more | 10.5x lower |

With concurrent clients and large values, single-shot collapses: 5 in-flight ~1 GB responses overwhelm both client and server memory, causing GC thrashing. Stream stays comfortable at ~1 GB server memory since chunks are sent and freed incrementally.

## Key observations

1. **Stream matches single-shot on latency and throughput for small values** while using 1.8x less client memory and 2.9x less server memory at 1M keys.
2. **With large values, stream wins on every metric** — 2.9x faster, 2.9x less client memory, 2.4x less server memory.
3. **Under concurrency with large values, stream's advantage explodes** — 15.6x faster latency, 15.8x less server memory. Single-shot's memory footprint causes GC thrashing and swapping.
4. **Paginated unary has no advantages** — slower than stream at every size (1.5x at 100k, 7x at 2M), with the gap growing superlinearly due to per-RPC overhead compounding across hundreds of pages.
5. **Count-always is expensive** — computing total count on every chunk via O(n) b-tree scan makes stream count-always slower than paginated at 1M+ keys. Counting once at the start is critical.

## Reproducing

```bash
# Start etcd
bin/etcd --listen-client-urls http://127.0.0.1:12379 \
  --advertise-client-urls http://127.0.0.1:12379 \
  --listen-peer-urls http://127.0.0.1:12380 \
  --data-dir /tmp/etcd-bench-data --name bench

# Load small values (adjust --total for key count)
go run ./tools/benchmark/ put --endpoints 127.0.0.1:12379 \
  --total 100000 --val-size 100 --conns 10 --clients 50

# Stream
go run ./tools/benchmark/ range 00000000 zzzzzzzz \
  --endpoints 127.0.0.1:12379 --total 10 --stream

# Single-shot unary
go run ./tools/benchmark/ range 00000000 zzzzzzzz \
  --endpoints 127.0.0.1:12379 --total 10

# Paginated unary
go run ./tools/benchmark/ range 00000000 zzzzzzzz \
  --endpoints 127.0.0.1:12379 --total 10 --page-size 10000

# Load large values (wipe data first: rm -rf /tmp/etcd-bench-data, restart etcd)
go run ./tools/benchmark/ put --endpoints 127.0.0.1:12379 \
  --total 10000 --val-size 102400 --conns 10 --clients 50

# Large values, concurrent
go run ./tools/benchmark/ range 00000000 zzzzzzzz \
  --endpoints 127.0.0.1:12379 --total 50 --clients 5 --stream
go run ./tools/benchmark/ range 00000000 zzzzzzzz \
  --endpoints 127.0.0.1:12379 --total 50 --clients 5
```
