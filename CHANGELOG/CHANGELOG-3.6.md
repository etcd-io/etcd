

Previous change logs can be found at [CHANGELOG-3.5](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.5.md).

<hr>

## v3.6.0 (TBD)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0...v3.6.0).

### Breaking Changes

- `etcd` will no longer start on data dir created by newer versions (for example etcd v3.6 will not run on v3.7+ data dir). To downgrade data dir please check out `etcdutl migrate` command.

### etcdctl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13133).
- When print endpoint status, [show db size in use](https://github.com/etcd-io/etcd/pull/13639)

### etcdutl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13142).
- Add `migrate` command for downgrading/upgrading etcd data dir files.

### Package `server`

- Package `mvcc` was moved to `storage/mvcc`
- Package `mvcc/backend` was moved to `storage/backend`
- Package `mvcc/buckets` was moved to `storage/schema`
- Package `wal` was moved to `storage/wal`
- Package `datadir` was moved to `storage/datadir`

### etcd server

- Add [`etcd --log-format`](https://github.com/etcd-io/etcd/pull/13339) flag to support log format.
- Add [`etcd --experimental-max-learners`](https://github.com/etcd-io/etcd/pull/13377) flag to allow configuration of learner max membership.
- Add [`etcd --experimental-enable-lease-checkpoint-persist`](https://github.com/etcd-io/etcd/pull/13508) flag to handle upgrade from v3.5.2 clusters with this feature enabled.
- Add [`etcdctl make-mirror --rev`](https://github.com/etcd-io/etcd/pull/13519) flag to support incremental mirror.
- Add [`etcd --experimental-wait-cluster-ready-timeout`](https://github.com/etcd-io/etcd/pull/13525) flag to wait for cluster to be ready before serving client requests.
- Fix [non mutating requests pass through quotaKVServer when NOSPACE](https://github.com/etcd-io/etcd/pull/13435)
- Fix [exclude the same alarm type activated by multiple peers](https://github.com/etcd-io/etcd/pull/13467).
- Fix [Provide a better liveness probe for when etcd runs as a Kubernetes pod](https://github.com/etcd-io/etcd/pull/13399)
- Fix [Lease checkpoints don't prevent to reset ttl on leader change](https://github.com/etcd-io/etcd/pull/13508).
- Fix [assertion failed due to tx closed when recovering v3 backend from a snapshot db](https://github.com/etcd-io/etcd/pull/13500)
- Fix [A client can panic etcd by passing invalid utf-8 in the client-api-version header](https://github.com/etcd-io/etcd/pull/13560)
- Fix [etcd gateway doesn't format the endpoint of IPv6 address correctly](https://github.com/etcd-io/etcd/pull/13551)
- Fix [A client can cause a nil dereference in etcd by passing an invalid SortTarget](https://github.com/etcd-io/etcd/pull/13555)

### tools/benchmark

- [Add etcd client autoSync flag](https://github.com/etcd-io/etcd/pull/13416)

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/latest/metrics/) for all metrics per release.

- Add [`etcd_disk_defrag_inflight`](https://github.com/etcd-io/etcd/pull/13371).

### Other

- Use Distroless as base image to make the image less vulnerable and reduce image size.

<hr>
