

Previous change logs can be found at [CHANGELOG-3.4](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.4.md).


The minimum recommended etcd versions to run in **production** are 3.2.28+, 3.3.18+, and 3.4.2+.


<hr>


## v3.5.0 (2020 TBD)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.4.0...v3.5.0) and [v3.5 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_5.md) for any breaking changes.

- [v3.5.0](https://github.com/etcd-io/etcd/releases/tag/v3.5.0) (2020 TBD), see [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0-rc.1...v3.5.0).
- [v3.5.0-rc.1](https://github.com/etcd-io/etcd/releases/tag/v3.5.0-rc.1) (2020 TBD), see [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0-rc.0...v3.5.0-rc.1).
- [v3.5.0-rc.0](https://github.com/etcd-io/etcd/releases/tag/v3.5.0-rc.0) (2020 TBD), see [code changes](https://github.com/etcd-io/etcd/compare/v3.4.0...v3.5.0-rc.0).

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.5 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_5.md).**

### Breaking Changes

- [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) only supports [`/v3`](TODO) endpoint.
  - Deprecated [`/v3beta`](https://github.com/etcd-io/etcd/pull/9298).
  - `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` does work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
- **`etcd --experimental-enable-v2v3` flag has been deprecated.** Use **`etcd --enable-v2v3`** instead.
  - Change [`etcd --experimental-enable-v2v3`](TODO) flag to `etcd --enable-v2v3`; v2 storage emulation is now stable.
- **`etcd --experimental-backend-bbolt-freelist-type` flag has been deprecated.** Use **`etcd --backend-bbolt-freelist-type`** instead. The default type is hashmap and it is stable now.
- **`etcd --debug` flag has been deprecated.** Use **`etcd --log-level=debug`** instead.
- Remove [`embed.Config.Debug`](https://github.com/etcd-io/etcd/pull/10947).
- **`etcd --log-output` flag has been deprecated.** Use **`etcd --log-outputs`** instead.
- **`etcd --logger=zap --log-outputs=stderr`** is now the default.
- **`etcd --logger=capnslog` flag value has been deprecated.**
- **`etcd --logger=zap --log-outputs=default` flag value is not supported.**.
  - Use `etcd --logger=zap --log-outputs=stderr`.
  - Or, use `etcd --logger=zap --log-outputs=systemd/journal` to send logs to the local systemd journal.
  - Previously, if etcd parent process ID (PPID) is 1 (e.g. run with systemd), `etcd --logger=capnslog --log-outputs=default` redirects server logs to local systemd journal. And if write to journald fails, it writes to `os.Stderr` as a fallback.
  - However, even with PPID 1, it can fail to dial systemd journal (e.g. run embedded etcd with Docker container). Then, [every single log write will fail](https://github.com/etcd-io/etcd/pull/9729) and fall back to `os.Stderr`, which is inefficient.
  - To avoid this problem, systemd journal logging must be configured manually.
- **`etcd --log-outputs=stderr`** is now the default.
- **`etcd --log-package-levels` flag for `capnslog` has been deprecated.** Now, **`etcd --logger=zap --log-outputs=stderr`** is the default.
- **`[CLIENT-URL]/config/local/log` endpoint has been deprecated, as is `etcd --log-package-levels` flag.**
  - `curl http://127.0.0.1:2379/config/local/log -XPUT -d '{"Level":"DEBUG"}'` won't work.
  - Please use `etcd --logger=zap --log-outputs=stderr` instead.
- Deprecated `etcd_debugging_mvcc_db_total_size_in_bytes` Prometheus metric. Use `etcd_mvcc_db_total_size_in_bytes` instead.
- Deprecated `etcd_debugging_mvcc_put_total` Prometheus metric. Use `etcd_mvcc_put_total` instead.
- Deprecated `etcd_debugging_mvcc_delete_total` Prometheus metric. Use `etcd_mvcc_delete_total` instead.
- Deprecated `etcd_debugging_mvcc_txn_total` Prometheus metric. Use `etcd_mvcc_txn_total` instead.
- Deprecated `etcd_debugging_mvcc_range_total` Prometheus metric. Use `etcd_mvcc_range_total` instead.
- Master branch `/version` outputs `3.5.0-pre`, instead of `3.4.0+git`.
- Changed `proxy` package function signature to [support structured logger](https://github.com/etcd-io/etcd/pull/11614).
  - Previously, `NewClusterProxy(c *clientv3.Client, advaddr string, prefix string) (pb.ClusterServer, <-chan struct{})`, now `NewClusterProxy(lg *zap.Logger, c *clientv3.Client, advaddr string, prefix string) (pb.ClusterServer, <-chan struct{})`.
  - Previously, `Register(c *clientv3.Client, prefix string, addr string, ttl int)`, now `Register(lg *zap.Logger, c *clientv3.Client, prefix string, addr string, ttl int) <-chan struct{}`.
  - Previously, `NewHandler(t *http.Transport, urlsFunc GetProxyURLs, failureWait time.Duration, refreshInterval time.Duration) http.Handler`, now `NewHandler(lg *zap.Logger, t *http.Transport, urlsFunc GetProxyURLs, failureWait time.Duration, refreshInterval time.Duration) http.Handler`.
- Changed `pkg/flags` function signature to [support structured logger](https://github.com/etcd-io/etcd/pull/11616).
  - Previously, `SetFlagsFromEnv(prefix string, fs *flag.FlagSet) error`, now `SetFlagsFromEnv(lg *zap.Logger, prefix string, fs *flag.FlagSet) error`.
  - Previously, `SetPflagsFromEnv(prefix string, fs *pflag.FlagSet) error`, now `SetPflagsFromEnv(lg *zap.Logger, prefix string, fs *pflag.FlagSet) error`.

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Deprecated `etcd_debugging_mvcc_db_total_size_in_bytes` Prometheus metric. Use `etcd_mvcc_db_total_size_in_bytes` instead.
- Deprecated `etcd_debugging_mvcc_put_total` Prometheus metric. Use `etcd_mvcc_put_total` instead.
- Deprecated `etcd_debugging_mvcc_delete_total` Prometheus metric. Use `etcd_mvcc_delete_total` instead.
- Deprecated `etcd_debugging_mvcc_txn_total` Prometheus metric. Use `etcd_mvcc_txn_total` instead.
- Deprecated `etcd_debugging_mvcc_range_total` Prometheus metric. Use `etcd_mvcc_range_total` instead.
- Add [`etcd_debugging_mvcc_current_revision`](https://github.com/etcd-io/etcd/pull/11126) Prometheus metric.
- Add [`etcd_debugging_mvcc_compact_revision`](https://github.com/etcd-io/etcd/pull/11126) Prometheus metric.
- Change [`etcd_cluster_version`](https://github.com/etcd-io/etcd/pull/11254) Prometheus metrics to include only major and minor version.
- Add [`etcd_debugging_mvcc_total_put_size_in_bytes`](https://github.com/etcd-io/etcd/pull/11374) Prometheus metric.
- Add [`etcd_server_client_requests_total` with `"type"` and `"client_api_version"` labels](https://github.com/etcd-io/etcd/pull/11687).

### etcd server

- [`etcd --enable-v2v3`](TODO) flag is now stable.
  - `etcd --experimental-enable-v2v3` has been deprecated.
  - Added [more v2v3 integration tests](https://github.com/etcd-io/etcd/pull/9634).
  - `etcd --enable-v2=true --enable-v2v3=''` by default, to enable v2 API server that is backed by **v2 store**.
  - `etcd --enable-v2=true --enable-v2v3=/aaa` to enable v2 API server that is backed by **v3 storage**.
  - `etcd --enable-v2=false --enable-v2v3=''` to disable v2 API server.
  - `etcd --enable-v2=false --enable-v2v3=/aaa` to disable v2 API server. TODO: error?
  - Automatically [create parent directory if it does not exist](https://github.com/etcd-io/etcd/pull/9626) (fix [issue#9609](https://github.com/etcd-io/etcd/issues/9609)).
  - v4.0 will configure `etcd --enable-v2=true --enable-v2v3=/aaa` to enable v2 API server that is backed by **v3 storage**.
- [`etcd --backend-bbolt-freelist-type`] flag is now stable.
  - `etcd --experimental-backend-bbolt-freelist-type` has been deprecated.
- Support [rollback/downgrade](TODO).
- Deprecate v2 apply on cluster version. [Use v3 request to set cluster version and recover cluster version from v3 backend](https://github.com/etcd-io/etcd/pull/11427).
- [Fix corruption bug in defrag](https://github.com/etcd-io/etcd/pull/11613).
- Fix [quorum protection logic when promoting a learner](https://github.com/etcd-io/etcd/pull/11640).
- Improve [peer corruption checker](https://github.com/etcd-io/etcd/pull/11621) to work when peer mTLS is enabled.
- Log [`[CLIENT-PORT]/health` check in server side](https://github.com/etcd-io/etcd/pull/11704).

### Package `embed`

- Remove [`embed.Config.Debug`](https://github.com/etcd-io/etcd/pull/10947).
  - Use `embed.Config.LogLevel` instead.
- Add [`embed.Config.ZapLoggerBuilder`](https://github.com/etcd-io/etcd/pull/11147) to allow creating a custom zap logger.

### Package `clientv3`

- Add [TryLock](https://github.com/etcd-io/etcd/pull/11104) method to `clientv3/concurrency/Mutex`. A non-blocking method on `Mutex` which does not wait to get lock on the Mutex, returns immediately if Mutex is locked by another session.
- Fix [client balancer failover against multiple endpoints](https://github.com/etcd-io/etcd/pull/11184).
  - Fix ["kube-apiserver: failover on multi-member etcd cluster fails certificate check on DNS mismatch" (kubernetes#83028)](https://github.com/kubernetes/kubernetes/issues/83028).
- Fix [IPv6 endpoint parsing in client](https://github.com/etcd-io/etcd/pull/11211).
  - Fix ["1.16: etcd client does not parse IPv6 addresses correctly when members are joining" (kubernetes#83550)](https://github.com/kubernetes/kubernetes/issues/83550).
- Use [ServerName as the authority](https://github.com/etcd-io/etcd/pull/11574) after bumping to grpc v1.26.0. Remove workaround in [#11184](https://github.com/etcd-io/etcd/pull/11184).
- Fix [`"hasleader"` metadata embedding](https://github.com/etcd-io/etcd/pull/11687).
  - Previously, `clientv3.WithRequireLeader(ctx)` was overwriting existing context keys.

### etcdctl v3

- Fix `etcdctl member add` command to prevent potential timeout. ([PR#11194](https://github.com/etcd-io/etcd/pull/11194) and [PR#11638](https://github.com/etcd-io/etcd/pull/11638))
- Add [`etcdctl watch --progress-notify`](https://github.com/etcd-io/etcd/pull/11462) flag.
- Add [`etcdctl auth status`](https://github.com/etcd-io/etcd/pull/11536) command to check if authentication is enabled

### gRPC gateway

- [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) only supports [`/v3`](TODO) endpoint.
  - Deprecated [`/v3beta`](https://github.com/etcd-io/etcd/pull/9298).
  - `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` does work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.

### gRPC Proxy

- Fix [`panic on error`](https://github.com/etcd-io/etcd/pull/11694) for metrics handler.

### Auth

- Fix [NoPassword check when adding user through GRPC gateway](https://github.com/etcd-io/etcd/pull/11418) ([issue#11414](https://github.com/etcd-io/etcd/issues/11414))
- Fix bug where [some auth related messages are logged at wrong level](https://github.com/etcd-io/etcd/pull/11586)

### API

- Add [`/v3/auth/status`](https://github.com/etcd-io/etcd/pull/11536) endpoint to check if authentication is enabled


### Dependency

- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) from [**`v1.23.0`**](https://github.com/grpc/grpc-go/releases/tag/v1.23.0) to [**`v1.26.0`**](https://github.com/grpc/grpc-go/releases/tag/v1.26.0).

### Go

- Require [*Go 1.13+*](https://github.com/etcd-io/etcd/pull/11110).
- Compile with [*Go 1.13*](https://golang.org/doc/devel/release.html#go1.13)

### Project Governance

- The etcd team has added, a well defined and openly discussed, project [governance](https://github.com/etcd-io/etcd/pull/11175).


<hr>

