

## [v3.2.15](https://github.com/coreos/etcd/releases/tag/v3.2.15) (2018-01-22)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.14...v3.2.15) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Prevent [server panic from member update/add](https://github.com/coreos/etcd/pull/9174) with [wrong scheme URLs](https://github.com/coreos/etcd/issues/9173).
- Log [user context cancel errors on stream APIs in debug level with TLS](https://github.com/coreos/etcd/pull/9178).


## [v3.2.14](https://github.com/coreos/etcd/releases/tag/v3.2.14) (2018-01-11)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.13...v3.2.14) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix [`mvcc/backend.defragdb` nil-pointer dereference on create bucket failure](https://github.com/coreos/etcd/pull/9119).

### Improved

- Log [user context cancel errors on stream APIs in debug level](https://github.com/coreos/etcd/pull/9105).


## [v3.2.13](https://github.com/coreos/etcd/releases/tag/v3.2.13) (2018-01-02)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.12...v3.2.13) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Remove [verbose error messages on stream cancel and gRPC info-level logs](https://github.com/coreos/etcd/pull/9080) in server-side.
- Fix [gRPC server panic on `GracefulStop` TLS-enabled server](https://github.com/coreos/etcd/pull/8987).


## [v3.2.12](https://github.com/coreos/etcd/releases/tag/v3.2.12) (2017-12-20)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.11...v3.2.12) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix [error message of `Revision` compactor](https://github.com/coreos/etcd/pull/8999) in server-side.

### Added(`etcd/clientv3`)

- Add [`MaxCallSendMsgSize` and `MaxCallRecvMsgSize`](https://github.com/coreos/etcd/pull/9047) fields to [`clientv3.Config`](https://godoc.org/github.com/coreos/etcd/clientv3#Config).
  - Fix [exceeded response size limit error in client-side](https://github.com/coreos/etcd/issues/9043).
  - Address [kubernetes#51099](https://github.com/kubernetes/kubernetes/issues/51099).
    - In previous versions(v3.2.10, v3.2.11), client response size was limited to only 4 MiB.
  - `MaxCallSendMsgSize` default value is 2 MiB, if not configured.
  - `MaxCallRecvMsgSize` default value is `math.MaxInt32`, if not configured.

### Other

- Pin [grpc v1.7.5](https://github.com/grpc/grpc-go/releases/tag/v1.7.5), [grpc-gateway v1.3.0](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3.0).
  - No code change, just to be explicit about recommended versions.


## [v3.2.11](https://github.com/coreos/etcd/releases/tag/v3.2.11) (2017-12-05)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.10...v3.2.11) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix racey grpc-go's server handler transport `WriteStatus` call to prevent [TLS-enabled etcd server crash](https://github.com/coreos/etcd/issues/8904).
  - Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) `v1.7.3` to `v1.7.4`.
  - Add [gRPC RPC failure warnings](https://github.com/coreos/etcd/pull/8939) to help debug such issues in the future.
- Remove `--listen-metrics-urls` flag in monitoring document (non-released in `v3.2.x`, planned for `v3.3.x`).

### Added

- Provide [more cert details](https://github.com/coreos/etcd/pull/8952/files) on TLS handshake failures.


## [v3.2.10](https://github.com/coreos/etcd/releases/tag/v3.2.10) (2017-11-16)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.9...v3.2.10) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Replace backend key-value database `boltdb/bolt` with [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to address [backend database size issue](https://github.com/coreos/etcd/issues/8009).
- Fix `clientv3` balancer to handle [network partitions](https://github.com/coreos/etcd/issues/8711).
  - Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) `v1.2.1` to `v1.7.3`.
  - Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) `v1.2` to `v1.3`.
- Revert [discovery SRV auth `ServerName` with `*.{ROOT_DOMAIN}`](https://github.com/coreos/etcd/pull/8651) to support non-wildcard subject alternative names in the certs (see [issue #8445](https://github.com/coreos/etcd/issues/8445) for more contexts).
  - For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `etcd.local` (**not `*.etcd.local`**) as an entry in Subject Alternative Name (SAN) field.


## [v3.2.9](https://github.com/coreos/etcd/releases/tag/v3.2.9) (2017-10-06)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.8...v3.2.9) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed(Security)

- Compile with [Go 1.8.4](https://groups.google.com/d/msg/golang-nuts/sHfMg4gZNps/a-HDgDDDAAAJ).
- Update `golang.org/x/crypto/bcrypt` (see [golang/crypto@6c586e1](https://github.com/golang/crypto/commit/6c586e17d90a7d08bbbc4069984180dce3b04117)).
- Fix discovery SRV bootstrapping to [authenticate `ServerName` with `*.{ROOT_DOMAIN}`](https://github.com/coreos/etcd/pull/8651), in order to support sub-domain wildcard matching (see [issue #8445](https://github.com/coreos/etcd/issues/8445) for more contexts).
  - For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `*.etcd.local` as an entry in Subject Alternative Name (SAN) field.


## [v3.2.8](https://github.com/coreos/etcd/releases/tag/v3.2.8) (2017-09-29)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.7...v3.2.8) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix v2 client failover to next endpoint on mutable operation.
- Fix grpc-proxy to respect `KeysOnly` flag.


## [v3.2.7](https://github.com/coreos/etcd/releases/tag/v3.2.7) (2017-09-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.6...v3.2.7) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix server-side auth so concurrent auth operations do not return old revision error.
- Fix concurrency/stm Put with serializable snapshot
  - Use store revision from first fetch to resolve write conflicts instead of modified revision.


## [v3.2.6](https://github.com/coreos/etcd/releases/tag/v3.2.6) (2017-08-21)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.5...v3.2.6).

### Fixed

- Fix watch restore from snapshot.
- Fix `etcd_debugging_mvcc_keys_total` inconsistency.
- Fix multiple URLs for `--listen-peer-urls` flag.
- Add `--enable-pprof` flag to etcd configuration file format.


## [v3.2.5](https://github.com/coreos/etcd/releases/tag/v3.2.5) (2017-08-04)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.4...v3.2.5) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Changed

- Use reverse lookup to match wildcard DNS SAN.
- Return non-zero exit code on unhealthy `endpoint health`.

### Fixed

- Fix unreachable /metrics endpoint when `--enable-v2=false`.
- Fix grpc-proxy to respect `PrevKv` flag.

### Added

- Add container registry `gcr.io/etcd-development/etcd`.


## [v3.2.4](https://github.com/coreos/etcd/releases/tag/v3.2.4) (2017-07-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.3...v3.2.4) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Do not block on active client stream when stopping server
- Fix gRPC proxy Snapshot RPC error handling


## [v3.2.3](https://github.com/coreos/etcd/releases/tag/v3.2.3) (2017-07-14)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.2...v3.2.3) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Let clients establish unlimited streams

### Added

- Tag docker images with minor versions
  - e.g. `docker pull quay.io/coreos/etcd:v3.2` to fetch latest v3.2 versions


## [v3.2.2](https://github.com/coreos/etcd/releases/tag/v3.2.2) (2017-07-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.1...v3.2.2) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Improved

- Rate-limit lease revoke on expiration.
- Extend leases on promote to avoid queueing effect on lease expiration.

### Fixed

- Use user-provided listen address to connect to gRPC gateway.
  - `net.Listener` rewrites IPv4 0.0.0.0 to IPv6 [::], breaking IPv6 disabled hosts.
  - Only v3.2.0, v3.2.1 are affected.
- Accept connection with matched IP SAN but no DNS match.
  - Don't check DNS entries in certs if there's a matching IP.
- Fix 'tools/benchmark' watch command.


## [v3.2.1](https://github.com/coreos/etcd/releases/tag/v3.2.1) (2017-06-23)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.0...v3.2.1) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix backend database in-memory index corruption issue on restore (only 3.2.0 is affected).
- Fix gRPC gateway Txn marshaling issue.
- Fix backend database size debugging metrics.


## [v3.2.0](https://github.com/coreos/etcd/releases/tag/v3.2.0) (2017-06-09)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.0...v3.2.0) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Improved

- Improve backend read concurrency.

### Added

- Embedded etcd
  - `Etcd.Peers` field is now `[]*peerListener`.
- RPCs
  - Add Election, Lock service.
- Native client etcdserver/api/v3client
  - client "embedded" in the server.
- gRPC proxy
  - Proxy endpoint discovery.
  - Namespaces.
  - Coalesce lease requests.
- v3 client
  - STM prefetching.
  - Add namespace feature.
  - Add `ErrOldCluster` with server version checking.
  - Translate `WithPrefix()` into `WithFromKey()` for empty key.
- v3 etcdctl
  - Add `check perf` command.
  - Add `--from-key` flag to role grant-permission command.
  - `lock` command takes an optional command to execute.
- etcd flags
  - Add `--enable-v2` flag to configure v2 backend (enabled by default).
  - Add `--auth-token` flag.
- `etcd gateway`
  - Support DNS SRV priority.
- Auth
  - Support Watch API.
  - JWT tokens.
- Logging, monitoring
  - Server warns large snapshot operations.
  - Add `etcd_debugging_server_lease_expired_total` metrics.
- Security
  - Deny incoming peer certs with wrong IP SAN.
  - Resolve TLS `DNSNames` when SAN checking.
  - Reload TLS certificates on every client connection.
- Release
  - Annotate acbuild with supports-systemd-notify.
  - Add `nsswitch.conf` to Docker container image.
  - Add ppc64le, arm64(experimental) builds.
  - Compile with `Go 1.8.3`.

### Changed

- v3 client
  - `LeaseTimeToLive` returns TTL=-1 resp on lease not found.
  - `clientv3.NewFromConfigFile` is moved to `clientv3/yaml.NewConfig`.
  - concurrency package's elections updated to match RPC interfaces.
  - let client dial endpoints not in the balancer.
- Dependencies
  - Update [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) to `v1.2.1`.
  - Update [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) to `v1.2.0`.
- Rejects domains names for `--listen-peer-urls` and `--listen-client-urls` (3.1 only prints out warnings), since [domain name is invalid for network interface binding](https://github.com/coreos/etcd/issues/6336).

### Fixed

- Allow v2 snapshot over 512MB.

