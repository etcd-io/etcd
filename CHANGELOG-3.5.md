

## v3.5.0 (TBD 2018-10-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.4.0...v3.5.0) and [v3.5 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_5.md) for any breaking changes.

### Breaking Changes

- [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) only supports [`/v3`](TODO) endpoint.
  - Deprecated [`/v3beta`](https://github.com/coreos/etcd/pull/9298).
  - `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` does work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.

### Added: gRPC gateway

- [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) only supports [`/v3`](TODO) endpoint.
  - Deprecated [`/v3beta`](https://github.com/coreos/etcd/pull/9298).
  - `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` does work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
