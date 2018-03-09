
## [v3.1.13](https://github.com/coreos/etcd/releases/tag/v3.1.13) (TBD)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.12...v3.1.13) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Improved

- Adjust [election timeout on server restart](https://github.com/coreos/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/coreos/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.


## [v3.1.12](https://github.com/coreos/etcd/releases/tag/v3.1.12) (2018-03-08)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.11...v3.1.12) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Fixed

- Fix [`mvcc` "unsynced" watcher restore operation](https://github.com/coreos/etcd/pull/9297).
  - "unsynced" watcher is watcher that needs to be in sync with events that have happened.
  - That is, "unsynced" watcher is the slow watcher that was requested on old revision.
  - "unsynced" watcher restore operation was not correctly populating its underlying watcher group.
  - Which possibly causes [missing events from "unsynced" watchers](https://github.com/coreos/etcd/issues/9086).

### Security

- Compile with [Go 1.8.7](https://groups.google.com/forum/#!topic/golang-announce/X7N1mvntnoU).


## [v3.1.11](https://github.com/coreos/etcd/releases/tag/v3.1.11) (2017-11-28)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.10...v3.1.11) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Fixed

- [#8411](https://github.com/coreos/etcd/issues/8411),[#8806](https://github.com/coreos/etcd/pull/8806) backport "mvcc: sending events after restore"
- [#8009](https://github.com/coreos/etcd/issues/8009),[#8902](https://github.com/coreos/etcd/pull/8902) backport coreos/bbolt v1.3.1-coreos.5


## [v3.1.10](https://github.com/coreos/etcd/releases/tag/v3.1.10) (2017-07-14)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.9...v3.1.10) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Changed

- Compile with Go 1.8.3; fix panic on `net/http.CloseNotify`

### Added

- Tag docker images with minor versions.
  - e.g. `docker pull quay.io/coreos/etcd:v3.1` to fetch latest v3.1 versions.


## [v3.1.9](https://github.com/coreos/etcd/releases/tag/v3.1.9) (2017-06-09)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.8...v3.1.9) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Fixed

- Allow v2 snapshot over 512MB.


## [v3.1.8](https://github.com/coreos/etcd/releases/tag/v3.1.8) (2017-05-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.7...v3.1.8) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.


## [v3.1.7](https://github.com/coreos/etcd/releases/tag/v3.1.7) (2017-04-28)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.6...v3.1.7) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.


## [v3.1.6](https://github.com/coreos/etcd/releases/tag/v3.1.6) (2017-04-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.5...v3.1.6) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Changed

- Remove auth check in Status API.

### Fixed

- Fill in Auth API response header.


## [v3.1.5](https://github.com/coreos/etcd/releases/tag/v3.1.5) (2017-03-27)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.4...v3.1.5) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Added

- Add `/etc/nsswitch.conf` file to alpine-based Docker image.

### Fixed

- Fix raft memory leak issue.
- Fix Windows file path issues.


## [v3.1.4](https://github.com/coreos/etcd/releases/tag/v3.1.4) (2017-03-22)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.3...v3.1.4) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.


## [v3.1.3](https://github.com/coreos/etcd/releases/tag/v3.1.3) (2017-03-10)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.2...v3.1.3) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Changed

- Use machine default host when advertise URLs are default values(`localhost:2379,2380`) AND if listen URL is `0.0.0.0`.

### Fixed

- Fix `etcd gateway` schema handling in DNS discovery.
- Fix sd_notify behaviors in `gateway`, `grpc-proxy`.


## [v3.1.2](https://github.com/coreos/etcd/releases/tag/v3.1.2) (2017-02-24)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.1...v3.1.2) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Changed

- Use IPv4 default host, by default (when IPv4 and IPv6 are available).

### Fixed

- Fix `etcd gateway` with multiple endpoints.


## [v3.1.1](https://github.com/coreos/etcd/releases/tag/v3.1.1) (2017-02-17)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.0...v3.1.1) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Changed

- Compile with `Go 1.7.5`.


## [v3.1.0](https://github.com/coreos/etcd/releases/tag/v3.1.0) (2017-01-20)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.0...v3.1.0) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Improved

- Faster linearizable reads (implements Raft read-index).
- v3 authentication API is now stable.

### Added

- Automatic leadership transfer when leader steps down.
- etcd flags
  - `--strict-reconfig-check` flag is set by default.
  - Add `--log-output` flag.
  - Add `--metrics` flag.
- v3 client
  - Add `SetEndpoints` method; update endpoints at runtime.
  - Add `Sync` method; auto-update endpoints at runtime.
  - Add `Lease TimeToLive` API; fetch lease information.
  - replace Config.Logger field with global logger.
  - Get API responses are sorted in ascending order by default.
- v3 etcdctl
  - Add `lease timetolive` command.
  - Add `--print-value-only` flag to get command.
  - Add `--dest-prefix` flag to make-mirror command.
  - `get` command responses are sorted in ascending order by default.
- `recipes` now conform to sessions defined in `clientv3/concurrency`.
- ACI has symlinks to `/usr/local/bin/etcd*`.
- Experimental gRPC proxy feature.

### Changed

- Deprecated following gRPC metrics in favor of [go-grpc-prometheus](https://github.com/grpc-ecosystem/go-grpc-prometheus).
  - `etcd_grpc_requests_total`
  - `etcd_grpc_requests_failed_total`
  - `etcd_grpc_active_streams`
  - `etcd_grpc_unary_requests_duration_seconds`
- etcd uses default route IP if advertise URL is not given.
- Cluster rejects removing members if quorum will be lost.
- SRV records (e.g., infra1.example.com) must match the discovery domain (i.e., example.com) if no custom certificate authority is given.
  - `TLSConfig.ServerName` is ignored with user-provided certificates for backwards compatibility; to be deprecated.
  - For example, `etcd --discovery-srv=example.com` will only authenticate peers/clients when the provided certs have root domain `example.com` as an entry in Subject Alternative Name (SAN) field.
- Discovery now has upper limit for waiting on retries.
- Warn on binding listeners through domain names; to be deprecated.

