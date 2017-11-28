## [v3.1.11](https://github.com/coreos/etcd/releases/tag/v3.1.11) (2017-11-28)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.10...v3.1.11).

### Fixed

- [#8411](https://github.com/coreos/etcd/issues/8411),[#8806](https://github.com/coreos/etcd/pull/8806) mvcc: fix watch restore from snapshot
- [#8009](https://github.com/coreos/etcd/issues/8009),[#8902](https://github.com/coreos/etcd/pull/8902) backport coreos/bbolt v1.3.1-coreos.5


## [v3.2.10](https://github.com/coreos/etcd/releases/tag/v3.2.10) (2017-11-16)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.9...v3.2.10).

### Fixed

- Replace backend key-value database `boltdb/bolt` with [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to address [backend database size issue](https://github.com/coreos/etcd/issues/8009)
- Fix clientv3 balancer to handle [network partition](https://github.com/coreos/etcd/issues/8711)
  - Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) v1.2.1 to v1.7.3
  - Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) v1.2 to v1.3
- Revert [discovery SRV auth `ServerName` with `*.{ROOT_DOMAIN}`](https://github.com/coreos/etcd/pull/8651) to support non-wildcard subject alternative names in the certs (see [issue #8445](https://github.com/coreos/etcd/issues/8445) for more contexts)
  - For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `etcd.local` (**not `*.etcd.local`**) as an entry in Subject Alternative Name (SAN) field


## [v3.2.9](https://github.com/coreos/etcd/releases/tag/v3.2.9) (2017-10-06)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.8...v3.2.9).

### Fixed(Security)

- Compile with [Go 1.8.4](https://groups.google.com/d/msg/golang-nuts/sHfMg4gZNps/a-HDgDDDAAAJ)
- Update `golang.org/x/crypto/bcrypt` (See [golang/crypto@6c586e1](https://github.com/golang/crypto/commit/6c586e17d90a7d08bbbc4069984180dce3b04117) for more)
- Fix discovery SRV bootstrapping to [authenticate `ServerName` with `*.{ROOT_DOMAIN}`](https://github.com/coreos/etcd/pull/8651), in order to support sub-domain wildcard matching (see [issue #8445](https://github.com/coreos/etcd/issues/8445) for more contexts)
  - For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `*.etcd.local` as an entry in Subject Alternative Name (SAN) field


## [v3.2.8](https://github.com/coreos/etcd/releases/tag/v3.2.8) (2017-09-29)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.7...v3.2.8).

### Fixed

- Fix v2 client failover to next endpoint on mutable operation
- Fix grpc-proxy to respect `KeysOnly` flag


## [v3.2.7](https://github.com/coreos/etcd/releases/tag/v3.2.7) (2017-09-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.6...v3.2.7).

### Fixed

- Fix server-side auth so concurrent auth operations do not return old revision error
- Fix concurrency/stm Put with serializable snapshot
  - Use store revision from first fetch to resolve write conflicts instead of modified revision


## [v3.2.6](https://github.com/coreos/etcd/releases/tag/v3.2.6) (2017-08-21)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.5...v3.2.6).

### Fixed

- Fix watch restore from snapshot
- Fix `etcd_debugging_mvcc_keys_total` inconsistency
- Fix multiple URLs for `--listen-peer-urls` flag
- Add `enable-pprof` to etcd configuration file format


## [v3.2.5](https://github.com/coreos/etcd/releases/tag/v3.2.5) (2017-08-04)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.4...v3.2.5).

### Changed

- Use reverse lookup to match wildcard DNS SAN
- Return non-zero exit code on unhealthy `endpoint health`

### Fixed

- Fix unreachable /metrics endpoint when `--enable-v2=false`
- Fix grpc-proxy to respect `PrevKv` flag

### Added

- Add container registry `gcr.io/etcd-development/etcd`


## [v3.2.4](https://github.com/coreos/etcd/releases/tag/v3.2.4) (2017-07-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.3...v3.2.4).

### Fixed

- Do not block on active client stream when stopping server
- Fix gRPC proxy Snapshot RPC error handling


## [v3.2.3](https://github.com/coreos/etcd/releases/tag/v3.2.3) (2017-07-14)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.2...v3.2.3).

### Fixed

- Let clients establish unlimited streams

### Added

- Tag docker images with minor versions
  - e.g. `docker pull quay.io/coreos/etcd:v3.2` to fetch latest v3.2 versions


## [v3.1.10](https://github.com/coreos/etcd/releases/tag/v3.1.10) (2017-07-14)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.9...v3.1.10).

### Changed

- Compile with Go 1.8.3 to fix panic on `net/http.CloseNotify`

### Added

- Tag docker images with minor versions
  - e.g. `docker pull quay.io/coreos/etcd:v3.1` to fetch latest v3.1 versions


## [v3.2.2](https://github.com/coreos/etcd/releases/tag/v3.2.2) (2017-07-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.1...v3.2.2).

### Improved

- Rate-limit lease revoke on expiration
- Extend leases on promote to avoid queueing effect on lease expiration

### Fixed

- Use user-provided listen address to connect to gRPC gateway
  - `net.Listener` rewrites IPv4 0.0.0.0 to IPv6 [::], breaking IPv6 disabled hosts
  - Only v3.2.0, v3.2.1 are affected
- Accept connection with matched IP SAN but no DNS match
  - Don't check DNS entries in certs if there's a matching IP
- Fix 'tools/benchmark' watch command


## [v3.2.1](https://github.com/coreos/etcd/releases/tag/v3.2.1) (2017-06-23)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.0...v3.2.1).

### Fixed

- Fix backend database in-memory index corruption issue on restore (only 3.2.0 is affected)
- Fix gRPC gateway Txn marshaling issue
- Fix backend database size debugging metrics


## [v3.2.0](https://github.com/coreos/etcd/releases/tag/v3.2.0) (2017-06-09)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.10...v3.2.0).
See [upgrade 3.2](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).

### Improved

- Improve backend read concurrency

### Added

- Embedded etcd
  - `Etcd.Peers` field is now `[]*peerListener`
- RPCs
  - Add Election, Lock service
- Native client etcdserver/api/v3client
  - client "embedded" in the server
- gRPC proxy
  - Proxy endpoint discovery
  - Namespaces
  - Coalesce lease requests
- v3 client
  - STM prefetching
  - Add namespace feature
  - Add `ErrOldCluster` with server version checking
  - Translate WithPrefix() into WithFromKey() for empty key
- v3 etcdctl
  - Add `check perf` command
  - Add `--from-key` flag to role grant-permission command
  - `lock` command takes an optional command to execute
- etcd flags
  - Add `--enable-v2` flag to configure v2 backend (enabled by default)
  - Add `--auth-token` flag
- `etcd gateway`
  - Support DNS SRV priority
- Auth
  - Support Watch API
  - JWT tokens
- Logging, monitoring
  - Server warns large snapshot operations
  - Add `etcd_debugging_server_lease_expired_total` metrics
- Security
  - Deny incoming peer certs with wrong IP SAN
  - Resolve TLS DNSNames when SAN checking
  - Reload TLS certificates on every client connection
- Release
  - Annotate acbuild with supports-systemd-notify
  - Add `nsswitch.conf` to Docker container image
  - Add ppc64le, arm64(experimental) builds
  - Compile with Go 1.8.3

### Changed

- v3 client
  - `LeaseTimeToLive` returns TTL=-1 resp on lease not found
  - `clientv3.NewFromConfigFile` is moved to `clientv3/yaml.NewConfig`
  - concurrency package's elections updated to match RPC interfaces
  - let client dial endpoints not in the balancer
- Dependencies
  - Update gRPC to v1.2.1
  - Update grpc-gateway to v1.2.0

### Fixed

- Allow v2 snapshot over 512MB


## [v3.1.9](https://github.com/coreos/etcd/releases/tag/v3.1.9) (2017-06-09)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.8...v3.1.9).

### Fixed

- Allow v2 snapshot over 512MB


## [v3.1.8](https://github.com/coreos/etcd/releases/tag/v3.1.8) (2017-05-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.7...v3.1.8).


## [v3.1.7](https://github.com/coreos/etcd/releases/tag/v3.1.7) (2017-04-28)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.6...v3.1.7).


## [v3.1.6](https://github.com/coreos/etcd/releases/tag/v3.1.6) (2017-04-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.5...v3.1.6).

### Changed

- Remove auth check in Status API

### Fixed

- Fill in Auth API response header


## [v3.1.5](https://github.com/coreos/etcd/releases/tag/v3.1.5) (2017-03-27)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.4...v3.1.5).

### Added

- Add `/etc/nsswitch.conf` file to alpine-based Docker image

### Fixed

- Fix raft memory leak issue
- Fix Windows file path issues


## [v3.1.4](https://github.com/coreos/etcd/releases/tag/v3.1.4) (2017-03-22)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.3...v3.1.4).


## [v3.1.3](https://github.com/coreos/etcd/releases/tag/v3.1.3) (2017-03-10)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.2...v3.1.3).

### Changed

- Use machine default host when advertise URLs are default values(localhost:2379,2380) AND if listen URL is 0.0.0.0

### Fixed

- Fix `etcd gateway` schema handling in DNS discovery
- Fix sd_notify behaviors in gateway, grpc-proxy


## [v3.1.2](https://github.com/coreos/etcd/releases/tag/v3.1.2) (2017-02-24)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.1...v3.1.2).

### Changed

- Use IPv4 default host, by default (when IPv4 and IPv6 are available)

### Fixed

- Fix `etcd gateway` with multiple endpoints


## [v3.1.1](https://github.com/coreos/etcd/releases/tag/v3.1.1) (2017-02-17)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.0...v3.1.1).

### Changed

- Compile with Go 1.7.5


## [v2.3.8](https://github.com/coreos/etcd/releases/tag/v2.3.8) (2017-02-17)

See [code changes](https://github.com/coreos/etcd/compare/v2.3.7...v2.3.8).

### Changed

- Compile with Go 1.7.5


## [v3.1.0](https://github.com/coreos/etcd/releases/tag/v3.1.0) (2017-01-20)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.16...v3.1.0).
See [upgrade 3.1](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md).

### Improved

- Faster linearizable reads (implements Raft read-index)
- v3 authentication API is now stable

### Added

- Automatic leadership transfer when leader steps down
- etcd flags
  - `--strict-reconfig-check` flag is set by default
  - Add `--log-output` flag
  - Add `--metrics` flag
- v3 client
  - Add SetEndpoints method; update endpoints at runtime
  - Add Sync method; auto-update endpoints at runtime
  - Add Lease TimeToLive API; fetch lease information
  - replace Config.Logger field with global logger
  - Get API responses are sorted in ascending order by default
- v3 etcdctl
  - Add lease timetolive command
  - Add `--print-value-only` flag to get command
  - Add `--dest-prefix` flag to make-mirror command
  - command get responses are sorted in ascending order by default
- `recipes` now conform to sessions defined in clientv3/concurrency
- ACI has symlinks to `/usr/local/bin/etcd*`
- experimental gRPC proxy feature

### Changed

- Deprecated following gRPC metrics in favor of [go-grpc-prometheus](https://github.com/grpc-ecosystem/go-grpc-prometheus):
  - `etcd_grpc_requests_total`
  - `etcd_grpc_requests_failed_total`
  - `etcd_grpc_active_streams`
  - `etcd_grpc_unary_requests_duration_seconds`
- etcd uses default route IP if advertise URL is not given
- Cluster rejects removing members if quorum will be lost
- SRV records (e.g., infra1.example.com) must match the discovery domain (i.e., example.com) if no custom certificate authority is given
  - TLSConfig ServerName is ignored with user-provided certificates for backwards compatibility; to be deprecated in 3.2
  - For example, `etcd --discovery-srv=example.com` will only authenticate peers/clients when the provided certs have root domain `example.com` as an entry in Subject Alternative Name (SAN) field
- Discovery now has upper limit for waiting on retries
- Warn on binding listeners through domain names; to be deprecated in 3.2


## [v3.0.16](https://github.com/coreos/etcd/releases/tag/v3.0.16) (2016-11-13)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.15...v3.0.16).


## [v3.0.15](https://github.com/coreos/etcd/releases/tag/v3.0.15) (2016-11-11)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.14...v3.0.15).

### Fixed

- Fix cancel watch request with wrong range end


## [v3.0.14](https://github.com/coreos/etcd/releases/tag/v3.0.14) (2016-11-04)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.13...v3.0.14).

### Added

- v3 etcdctl migrate command now supports `--no-ttl` flag to discard keys on transform


## [v3.0.13](https://github.com/coreos/etcd/releases/tag/v3.0.13) (2016-10-24)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.12...v3.0.13).


## [v3.0.12](https://github.com/coreos/etcd/releases/tag/v3.0.12) (2016-10-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.11...v3.0.12).


## [v3.0.11](https://github.com/coreos/etcd/releases/tag/v3.0.11) (2016-10-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.10...v3.0.11).

### Added

- Server returns previous key-value (optional)
  - `clientv3.WithPrevKV` option
  - v3 etcdctl put,watch,del `--prev-kv` flag


## [v3.0.10](https://github.com/coreos/etcd/releases/tag/v3.0.10) (2016-09-23)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.9...v3.0.10).


## [v3.0.9](https://github.com/coreos/etcd/releases/tag/v3.0.9) (2016-09-15)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.8...v3.0.9).

### Added

- Warn on domain names on listen URLs (v3.2 will reject domain names)


## [v3.0.8](https://github.com/coreos/etcd/releases/tag/v3.0.8) (2016-09-09)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.7...v3.0.8).

### Changed

- Allow only IP addresses in listen URLs (domain names are rejected)


## [v3.0.7](https://github.com/coreos/etcd/releases/tag/v3.0.7) (2016-08-31)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.6...v3.0.7).

### Changed

- SRV records only allow A records (RFC 2052)


## [v3.0.6](https://github.com/coreos/etcd/releases/tag/v3.0.6) (2016-08-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.5...v3.0.6).


## [v3.0.5](https://github.com/coreos/etcd/releases/tag/v3.0.5) (2016-08-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.4...v3.0.5).

### Changed

- SRV records (e.g., infra1.example.com) must match the discovery domain (i.e., example.com) if no custom certificate authority is given


## [v3.0.4](https://github.com/coreos/etcd/releases/tag/v3.0.4) (2016-07-27)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.3...v3.0.4).

### Changed

- v2 auth can now use common name from TLS certificate when `--client-cert-auth` is enabled

### Added

- v2 etcdctl ls command now supports `--output=json`
- Add /var/lib/etcd directory to etcd official Docker image


## [v3.0.3](https://github.com/coreos/etcd/releases/tag/v3.0.3) (2016-07-15)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.2...v3.0.3).

### Changed

- Revert Dockerfile to use `CMD`, instead of `ENTRYPOINT`, to support etcdctl run
  - Docker commands for v3.0.2 won't work without specifying executable binary paths
- v3 etcdctl default endpoints are now 127.0.0.1:2379


## [v3.0.2](https://github.com/coreos/etcd/releases/tag/v3.0.2) (2016-07-08)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.1...v3.0.2).

### Changed

- Dockerfile uses `ENTRYPOINT`, instead of `CMD`, to run etcd without binary path specified


## [v3.0.1](https://github.com/coreos/etcd/releases/tag/v3.0.1) (2016-07-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.0...v3.0.1).


## [v3.0.0](https://github.com/coreos/etcd/releases/tag/v3.0.0) (2016-06-30)

See [code changes](https://github.com/coreos/etcd/compare/v2.3.8...v3.0.0).
See [upgrade 3.0](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).
