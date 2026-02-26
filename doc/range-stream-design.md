# RangeStream Design

## Problem

The unary `Range` RPC builds the entire response in memory before sending. For large result sets this means:

- Server-side memory spikes — the KV slice, serialized protobuf, and gRPC send buffer all coexist.
- No backpressure — the client cannot control consumption rate.
- Redundant work — client-side pagination (repeated `Range` calls with increasing keys) recomputes the total count on every page by walking the full B-tree index.

## Approach

Add a server-streaming `RangeStream` RPC that reuses `RangeRequest` and returns results in chunks. The server does the pagination internally with adaptive chunk sizing, pins to a single MVCC revision for consistency, and only computes the total count once.

```protobuf
service KV {
  rpc RangeStream(RangeRequest) returns (stream RangeStreamResponse) {}
}

message RangeStreamResponse {
  option (versionpb.etcd_version_msg) = "3.7";
  RangeResponse range_response = 1;
}
```

`RangeStreamResponse` wraps `RangeResponse` so that `proto.Merge()` across all chunks produces the same result as a single `Range()` call. The wrapper also leaves room for future streaming-specific fields (progress, mid-stream errors).

## Stream Message Layout

| Message | Contents |
|---------|----------|
| Header | ClusterId, MemberId, RaftTerm (sent immediately from v3rpc layer) |
| First chunk | Revision, Kvs |
| Intermediate chunks | Kvs only |
| Final chunk | Kvs, Count, More |

Count and More are deferred to the final message. Revision is only in the first data chunk. Clients reassemble by merging all messages.

## Key Design Decisions

### Adaptive chunk sizing

The chunk limit starts at 10 keys and adjusts based on response size relative to `MaxRequestBytes`:

- `resp.Size() < MaxRequestBytes/2` — double the limit
- `resp.Size() > MaxRequestBytes*2` — reduce by 2/3

This auto-tunes for both small-value and large-value workloads without requiring the client to pick a page size.

### Revision pinning

After the first internal `Range` call, `r.Revision` is pinned to the returned revision. All subsequent chunks read from the same MVCC snapshot, so the stream is consistent even if writes arrive during streaming.

### CountTotal optimization

A new `CountTotal bool` on `RangeOptions` controls whether `treeIndex.Revisions()` counts all matching keys or exits early at the limit:

```go
if countTotal {
    total++
} else if limit > 0 && len(revs) >= limit {
    return false  // early exit
}
```

The first `RangeStream` page uses `countTotal=true` to get the accurate total. All subsequent pages use `countTotal=false`, changing per-page cost from O(total_keys) to O(limit).

### Unsupported options fallback

Requests with non-default sort order or min/max create/mod revision filters fall back to a single buffered `Range` call, since these require server-side post-processing that defeats streaming.

## What Changed

### Proto (`api/etcdserverpb/rpc.proto`)
- New `RangeStream` RPC on the `KV` service.
- New `RangeStreamResponse` message wrapping `RangeResponse`.

### v3rpc (`server/etcdserver/api/v3rpc/key.go`)
- `kvServer.RangeStream` — validates the request, sends the header message immediately, delegates to `EtcdServer.RangeStream`.

### EtcdServer (`server/etcdserver/v3_server.go`)
- `RangeStream` — same auth and linearizability path as unary `Range`.
- `rangeStream` — the chunking loop: adaptive sizing, revision pinning, cursor advancement via `append(lastKey, '\x00')`.

### MVCC (`server/storage/mvcc/`)
- `RangeOptions.CountTotal` field (`kv.go`).
- `treeIndex.Revisions()` accepts `countTotal bool`, enabling early exit when count isn't needed (`index.go`).
- `kvstore_txn.go` passes `CountTotal` through to the index.

### Client (`client/v3/kv.go`)
- `RangeStreamToRangeResponse` — reassembles a stream into a single `RangeResponse` so callers can transparently switch between unary and streaming.

### gRPC Proxy (`server/proxy/grpcproxy/`)
- `kvs2kvc.RangeStream` adapter using channel-based `pipeStream` to bridge server/client stream interfaces.
- Fix for `SendMsg`/channel-close data race in `chanStream`.

### Tests
- Integration tests (`tests/integration/v3_grpc_test.go`) — every existing `Range` test case also calls `RangeStream` and diffs the reassembled response against the unary result.
- MVCC microbenchmarks (`server/storage/mvcc/kvstore_range_bench_test.go`) — `BenchmarkRangeUnary` vs `BenchmarkRangeStream` at 1K–500K keys.

## Limitations

- Sort/filter requests bypass chunking (sent as a single message).
- Adaptive sizing is heuristic — doesn't precisely cap per-message bytes.
- No gRPC-gateway (HTTP/JSON) mapping.
