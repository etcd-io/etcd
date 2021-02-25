

Previous change logs can be found at [CHANGELOG-3.x](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.x.md).


The minimum recommended etcd versions to run in **production** are 3.2.28+, 3.3.18+, and 3.4.2+.


<hr>


## v4.0.0 (TBD)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0...v4.0.0) and [v4.0 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_4_0/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v4.0 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_4_0/).**

### Breaking Changes

- [Secure etcd by default](https://github.com/etcd-io/etcd/issues/9475)?
- Change `/health` endpoint output.
  - Previously, `{"health":"true"}`.
  - Now, `{"health":true}`.
  - Breaks [Kubernetes `kubectl get componentstatuses` command](https://github.com/kubernetes/kubernetes/issues/58240).
- Deprecate [`etcd --proxy*`](TODO) flags; **no more v2 proxy**.
- Deprecate [v2 storage backend](https://github.com/etcd-io/etcd/issues/9232); **no more v2 store**.
  - v2 API is still supported via [v2 emulation](TODO).
- Deprecate [`etcdctl backup`](TODO) command.
- `clientv3.Client.KeepAlive(ctx context.Context, id LeaseID) (<-chan *LeaseKeepAliveResponse, error)` is now [`clientv4.Client.KeepAlive(ctx context.Context, id LeaseID) <-chan *LeaseKeepAliveResponse`](TODO).
  - Similar to `Watch`, [`KeepAlive` does not return errors](https://github.com/etcd-io/etcd/issues/7488).
  - If there's an unknown server error, kill all open channels and create a new stream on the next `KeepAlive` call.
- Rename `github.com/coreos/client` to `github.com/coreos/clientv2`.
- [`etcd --experimental-initial-corrupt-check`](TODO) has been  deprecated.
  - Use [`etcd --initial-corrupt-check`](TODO) instead.
- [`etcd --experimental-corrupt-check-time`](TODO) has been  deprecated.
  - Use [`etcd --corrupt-check-time`](TODO) instead.
- Enable TLS 1.13, deprecate TLS cipher suites.

### etcd server

- [`etcd --initial-corrupt-check`](TODO) flag is now stable (`etcd --experimental-initial-corrupt-check` has been  deprecated).
  - `etcd --initial-corrupt-check=true` by default, to check cluster database hashes before serving client/peer traffic.
- [`etcd --corrupt-check-time`](TODO) flag is now stable (`etcd --experimental-corrupt-check-time` has been  deprecated).
  - `etcd --corrupt-check-time=12h` by default, to check cluster database hashes for every 12-hour.
- Enable TLS 1.13, deprecate TLS cipher suites.

### Go

- Require [*Go 2*](https://blog.golang.org/go2draft).


<hr>

