Previous change logs can be found at [CHANGELOG-3.7](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.7.md).

---

## v3.8.0 (TBC)

### etcd server

- [Cleanup everything related to v2 snapshot](https://github.com/etcd-io/etcd/issues/20187), see notable changes below,
  - [Stop generating v2 snapshot files](https://github.com/etcd-io/etcd/pull/22263)
  - [Remove the periodic job of purging v2 snapshot files](https://github.com/etcd-io/etcd/pull/22271)
  - [Remove flag `--max-snapshots` and `--v2-deprecation`](https://github.com/etcd-io/etcd/pull/22306)
  - [Cleanup the legacy v2 snapshot files on bootstrap](https://github.com/etcd-io/etcd/pull/22336)
  - [Cleanup the legacy v2 snapshot source code and cleanup orphaned defragmentation files on bootstrap](https://github.com/etcd-io/etcd/pull/22341)

### Dependencies

- Compile binaries using [go 1.26.5](https://github.com/etcd-io/etcd/pull/22062).

### etcdutl

- [Add bbolt subcommand](https://github.com/etcd-io/etcd/pull/20162) to etcdutl

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/latest/metrics/) for all metrics per release.

- Expose the full set of Go `runtime/metrics` on `/metrics` when `--metrics extensive` is set.
