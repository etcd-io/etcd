

Previous change logs can be found at [CHANGELOG-3.5](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.5.md).

<hr>

## v3.6.0 (TBD)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0...v3.6.0).

### Breaking Changes

- `etcd` will no longer start on data dir created by newer versions (for example etcd v3.6 will not run on v3.7+ data dir). To downgrade data dir please check out `etcdutl migrate` command.
- `etcd` doesn't support serving client requests on the peer listen endpoints (--listen-peer-urls). See [pull/13565](https://github.com/etcd-io/etcd/pull/13565).
- `etcdctl` will sleep(2s) in case of range delete without `--range` flag. See [pull/13747](https://github.com/etcd-io/etcd/pull/13747)
- Applications which depend on etcd v3.6 packages must be built with go version >= v1.18.

### Deprecations

- Deprecated [V2 discovery](https://etcd.io/docs/v3.5/dev-internal/discovery_protocol/).
- Deprecated [SetKeepAlive and SetKeepAlivePeriod in limitListenerConn](https://github.com/etcd-io/etcd/pull/14356).
- Removed [etcdctl defrag --data-dir](https://github.com/etcd-io/etcd/pull/13793).
- Removed [etcdctl snapshot status](https://github.com/etcd-io/etcd/pull/13809).
- Removed [etcdctl snapshot restore](https://github.com/etcd-io/etcd/pull/13809).
- Removed [etcdutl snapshot save](https://github.com/etcd-io/etcd/pull/13809).


### etcdctl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13133).
- When print endpoint status, [show db size in use](https://github.com/etcd-io/etcd/pull/13639)
- [Always print the raft_term in decimal](https://github.com/etcd-io/etcd/pull/13711) when displaying member list in json.
- [Add one more field `storageVersion`](https://github.com/etcd-io/etcd/pull/13773) into the response of command `etcdctl endpoint status`.
- Add [`--max-txn-ops`](https://github.com/etcd-io/etcd/pull/14340) flag to make-mirror command.
- Add [`--consistency`](https://github.com/etcd-io/etcd/pull/15261) flag to member list command.
- Display [field `hash_revision`](https://github.com/etcd-io/etcd/pull/14812) for `etcdctl endpoint hash` command.

### etcdutl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13142).
- Add `migrate` command for downgrading/upgrading etcd data dir files.
- Add [optional --bump-revision and --mark-compacted flag to etcdutl snapshot restore operation](https://github.com/etcd-io/etcd/pull/16029).

### Package `clientv3`

- [Support serializable `MemberList` operation](https://github.com/etcd-io/etcd/pull/15261).

### Package `server`

- Package `mvcc` was moved to `storage/mvcc`
- Package `mvcc/backend` was moved to `storage/backend`
- Package `mvcc/buckets` was moved to `storage/schema`
- Package `wal` was moved to `storage/wal`
- Package `datadir` was moved to `storage/datadir`

### Package `raft`
- Send empty `MsgApp` when entry in-flight limits are exceeded. See [pull/14633](https://github.com/etcd-io/etcd/pull/14633).
- Add [MaxInflightBytes](https://github.com/etcd-io/etcd/pull/14624) setting in `raft.Config` for better flow control of entries.
- [Decouple raft from etcd](https://github.com/etcd-io/etcd/issues/14713). Migrated raft to a separate [repository](https://github.com/etcd-io/raft), and renamed raft module to `go.etcd.io/raft/v3`.

### etcd server

- Add [`etcd --log-format`](https://github.com/etcd-io/etcd/pull/13339) flag to support log format.
- Add [`etcd --experimental-max-learners`](https://github.com/etcd-io/etcd/pull/13377) flag to allow configuration of learner max membership.
- Add [`etcd --experimental-enable-lease-checkpoint-persist`](https://github.com/etcd-io/etcd/pull/13508) flag to handle upgrade from v3.5.2 clusters with this feature enabled.
- Add [`etcdctl make-mirror --rev`](https://github.com/etcd-io/etcd/pull/13519) flag to support incremental mirror.
- Add [`etcd --experimental-wait-cluster-ready-timeout`](https://github.com/etcd-io/etcd/pull/13525) flag to wait for cluster to be ready before serving client requests.
- Add [v3 discovery](https://github.com/etcd-io/etcd/pull/13635) to bootstrap a new etcd cluster.
- Add [field `storage`](https://github.com/etcd-io/etcd/pull/13772) into the response body of endpoint `/version`.
- Add [`etcd --max-concurrent-streams`](https://github.com/etcd-io/etcd/pull/14169) flag to configure the max concurrent streams each client can open at a time, and defaults to math.MaxUint32.
- Add [`etcd grpc-proxy --experimental-enable-grpc-logging`](https://github.com/etcd-io/etcd/pull/14266) flag to logging all grpc requests and responses.
- Add [`etcd --experimental-compact-hash-check-enabled --experimental-compact-hash-check-time`](https://github.com/etcd-io/etcd/issues/14039) flags to support enabling reliable corruption detection on compacted revisions.
- Add [Protection on maintenance request when auth is enabled](https://github.com/etcd-io/etcd/pull/14663).
- Graduated [`--experimental-warning-unary-request-duration` to `--warning-unary-request-duration`](https://github.com/etcd-io/etcd/pull/14414). Note the experimental flag is deprecated and will be decommissioned in v3.7.
- Add [field `hash_revision` into `HashKVResponse`](https://github.com/etcd-io/etcd/pull/14537).
- Add [`etcd --experimental-snapshot-catch-up-entries`](https://github.com/etcd-io/etcd/pull/15033) flag to configure number of entries for a slow follower to catch up after compacting the raft storage entries and defaults to 5k. 
- Decreased [`--snapshot-count` default value from 100,000 to 10,000](https://github.com/etcd-io/etcd/pull/15408)
- Add [`etcd --tls-min-version --tls-max-version`](https://github.com/etcd-io/etcd/pull/15156) to enable support for TLS 1.3.

### etcd grpc-proxy

- Add [`etcd grpc-proxy start --endpoints-auto-sync-interval`](https://github.com/etcd-io/etcd/pull/14354) flag to enable and configure interval of auto sync of endpoints with server.
- Add [`etcd grpc-proxy start --listen-cipher-suites`](https://github.com/etcd-io/etcd/pull/14308) flag to support adding configurable cipher list.

### tools/benchmark

- [Add etcd client autoSync flag](https://github.com/etcd-io/etcd/pull/13416)

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/latest/metrics/) for all metrics per release.

- Add [`etcd_disk_defrag_inflight`](https://github.com/etcd-io/etcd/pull/13371).
- Add [`etcd_debugging_server_alarms`](https://github.com/etcd-io/etcd/pull/14276).

### Go
- Require [Go 1.20+](https://github.com/etcd-io/etcd/pull/16394).
- Compile with [Go 1.20+](https://golang.org/doc/devel/release.html#go1.20). Please refer to [gc-guide](https://go.dev/doc/gc-guide) to configure `GOGC` and `GOMEMLIMIT` properly. 

### Other

- Use Distroless as base image to make the image less vulnerable and reduce image size.

<hr>
