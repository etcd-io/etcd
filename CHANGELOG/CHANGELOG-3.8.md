Previous change logs can be found at [CHANGELOG-3.7](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.7.md).

---

## v3.8.0 (TBC)

### Dependencies

- Compile binaries using [go 1.26.5](https://github.com/etcd-io/etcd/pull/22062).

### etcdutl

- [Add bbolt subcommand](https://github.com/etcd-io/etcd/pull/20162) to etcdutl

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/latest/metrics/) for all metrics per release.

- Expose the full set of Go `runtime/metrics` on `/metrics` when `--metrics extensive` is set.
