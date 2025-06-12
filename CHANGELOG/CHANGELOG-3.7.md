
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

### Package `pkg`

- [Optimize find performance by splitting intervals with the same left endpoint by their right endpoints](https://github.com/etcd-io/etcd/pull/19768)

### Dependencies

- Compile binaries using [go 1.24.4](https://github.com/etcd-io/etcd/pull/20163)

### Deprecations

- Deprecated [UsageFunc in pkg/cobrautl](https://github.com/etcd-io/etcd/pull/18356).
