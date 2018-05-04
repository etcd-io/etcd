

Previous change logs can be found at [CHANGELOG-3.4](https://github.com/coreos/etcd/blob/master/CHANGELOG-3.4.md).


## v3.5.0 (TBD 2018-12)

See [code changes](https://github.com/coreos/etcd/compare/v3.4.0...v3.5.0) and [v3.5 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_5.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.5 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_5.md).**

### Breaking Changes

- [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) only supports [`/v3`](TODO) endpoint.
  - Deprecated [`/v3beta`](https://github.com/coreos/etcd/pull/9298).
  - `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` does work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
- **`etcd --log-output` has been deprecated**. Use **`etcd --log-outputs`** instead.
- **`etcd --logger=capnslog` has been deprecated**. Now, **`etcd --logger=zap`** is the default.
- **`etcd --log-package-levels` for `capnslog` has been deprecated**. Now, **`etcd --logger=zap`** is the default.

### gRPC gateway

- [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) only supports [`/v3`](TODO) endpoint.
  - Deprecated [`/v3beta`](https://github.com/coreos/etcd/pull/9298).
  - `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` does work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
