

Previous change logs can be found at [CHANGELOG-3.5](https://github.com/etcd-io/etcd/blob/main/CHANGELOG-3.5.md).

<hr>

## v3.6.0 (TBD)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0...v3.6.0).

### etcdctl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13133).

### etcdutl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13142).
- Add `migrate` command for downgrading/upgrading etcd data dir files.

### Package `server`

- Package `mvcc` was moved to `storage/mvcc`
- Package `mvcc/backend` was moved to `storage/backend`
- Package `mvcc/buckets` was moved to `storage/schema`
- Package `wal` was moved to `storage/wal`
- Package `datadir` was moved to `storage/datadir`
