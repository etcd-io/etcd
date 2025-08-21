
Previous change logs can be found at [CHANGELOG-3.6](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.6.md).

---

## v3.7.0 (TBD)

### Breaking Changes

- [Removed all deprecated experimental flags](https://github.com/etcd-io/etcd/pull/19959)
- [Removed v2discovery](https://github.com/etcd-io/etcd/pull/20109)
- [Removed client/v2](https://github.com/etcd-io/etcd/pull/20117)

### etcd server

- [Update go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc to v0.61.0 and replaced the deprecated `UnaryServerInterceptor` and `StreamServerInterceptor` with `NewServerHandler`](https://github.com/etcd-io/etcd/pull/20017)
- [Add Support for Unix Socket endpoints](https://github.com/etcd-io/etcd/pull/19760)
- [Improves performance of lease and user/role operations (up to 2x) by updating `(*readView) Rev()` to use `SharedBufReadTxMode`](https://github.com/etcd-io/etcd/pull/20411)

### Package `pkg`

- [Optimize find performance by splitting intervals with the same left endpoint by their right endpoints](https://github.com/etcd-io/etcd/pull/19768)
- [netutil: Refactor IPv6 address comparison logic](https://github.com/etcd-io/etcd/pull/20365)

### Dependencies

- Compile binaries using [go 1.24.6](https://github.com/etcd-io/etcd/pull/20460)
- [Migrate the deprecated go-grpc-middleware v1 logging and tags libraries to v2 interceptors](https://github.com/etcd-io/etcd/pull/20420)

### Deprecations

- Deprecated [UsageFunc in pkg/cobrautl](https://github.com/etcd-io/etcd/pull/18356).

### etcdctl

- [Organize etcdctl commands](https://github.com/etcd-io/etcd/pull/20162) to make them more concise and easier to understand.
- [Hide the global flags](https://github.com/etcd-io/etcd/pull/20493) to make the output of `etcdctl --help` looks cleaner and is consistent with kubectl.
