Previous change logs can be found at [CHANGELOG-3.7](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.7.md).

---

## v3.8.0 (TBC)

### Dependencies

- Compile binaries using [go 1.26.5](https://github.com/etcd-io/etcd/pull/22062).

### etcdutl

- Add `etcdutl init` command to initialize a member's data directory offline, without a snapshot file or a running server. The command is idempotent: an already initialized data directory is validated against the given configuration instead of being overwritten, so init can run for example as a Kubernetes init container.
