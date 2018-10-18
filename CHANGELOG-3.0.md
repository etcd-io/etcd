

## [v3.0.16](https://github.com/etcd-io/etcd/releases/tag/v3.0.16) (2016-11-13)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.15...v3.0.16) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.4*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.15](https://github.com/etcd-io/etcd/releases/tag/v3.0.15) (2016-11-11)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.14...v3.0.15) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Fixed

- Fix cancel watch request with wrong range end.

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.14](https://github.com/etcd-io/etcd/releases/tag/v3.0.14) (2016-11-04)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.13...v3.0.14) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Added

- v3 `etcdctl migrate` command now supports `--no-ttl` flag to discard keys on transform.

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.13](https://github.com/etcd-io/etcd/releases/tag/v3.0.13) (2016-10-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.12...v3.0.13) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.12](https://github.com/etcd-io/etcd/releases/tag/v3.0.12) (2016-10-07)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.11...v3.0.12) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.11](https://github.com/etcd-io/etcd/releases/tag/v3.0.11) (2016-10-07)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.10...v3.0.11) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Added

- Server returns previous key-value (optional)
  - `clientv3.WithPrevKV` option
  - v3 etcdctl `put,watch,del --prev-kv` flag

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.10](https://github.com/etcd-io/etcd/releases/tag/v3.0.10) (2016-09-23)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.9...v3.0.10) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.9](https://github.com/etcd-io/etcd/releases/tag/v3.0.9) (2016-09-15)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.8...v3.0.9) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Added

- Warn on domain names on listen URLs (v3.2 will reject domain names).

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.8](https://github.com/etcd-io/etcd/releases/tag/v3.0.8) (2016-09-09)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.7...v3.0.8) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Other

- Allow only IP addresses in listen URLs (domain names are rejected).

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.7](https://github.com/etcd-io/etcd/releases/tag/v3.0.7) (2016-08-31)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.6...v3.0.7) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Other

- SRV records only allow A records (RFC 2052).

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.6](https://github.com/etcd-io/etcd/releases/tag/v3.0.6) (2016-08-19)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.5...v3.0.6) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.5](https://github.com/etcd-io/etcd/releases/tag/v3.0.5) (2016-08-19)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.4...v3.0.5) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Other

- SRV records (e.g., infra1.example.com) must match the discovery domain (i.e., example.com) if no custom certificate authority is given.

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.4](https://github.com/etcd-io/etcd/releases/tag/v3.0.4) (2016-07-27)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.3...v3.0.4) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Added

- v2 `etcdctl ls` command now supports `--output=json`.
- Add /var/lib/etcd directory to etcd official Docker image.

### Other

- v2 auth can now use common name from TLS certificate when `--client-cert-auth` is enabled.

### Go

- Compile with [*Go 1.6.3*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.3](https://github.com/etcd-io/etcd/releases/tag/v3.0.3) (2016-07-15)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.2...v3.0.3) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Other

- Revert Dockerfile to use `CMD`, instead of `ENTRYPOINT`, to support `etcdctl` run.
  - Docker commands for v3.0.2 won't work without specifying executable binary paths.
- v3 etcdctl default endpoints are now `127.0.0.1:2379`.

### Go

- Compile with [*Go 1.6.2*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.2](https://github.com/etcd-io/etcd/releases/tag/v3.0.2) (2016-07-08)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.1...v3.0.2) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Other

- Dockerfile uses `ENTRYPOINT`, instead of `CMD`, to run etcd without binary path specified.

### Go

- Compile with [*Go 1.6.2*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.1](https://github.com/etcd-io/etcd/releases/tag/v3.0.1) (2016-07-01)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.0...v3.0.1) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.2*](https://golang.org/doc/devel/release.html#go1.6).


## [v3.0.0](https://github.com/etcd-io/etcd/releases/tag/v3.0.0) (2016-06-30)

See [code changes](https://github.com/etcd-io/etcd/compare/v2.3.0...v3.0.0) and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.0 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md).**

### Go

- Compile with [*Go 1.6.2*](https://golang.org/doc/devel/release.html#go1.6).

