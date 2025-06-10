

Previous change logs can be found at [CHANGELOG-3.5](https://github.com/etcd-io/etcd/blob/main/CHANGELOG/CHANGELOG-3.5.md).

---

## v3.6.2 (TBD)

### Dependencies

- [Bump bbolt to v1.4.1](https://github.com/etcd-io/etcd/pull/20154)

---

## v3.6.1 (2025-06-06)

### etcd server

- [Replaced the deprecated/removed `UnaryServerInterceptor` and `StreamServerInterceptor` in otelgrpc with `NewServerHandler`](https://github.com/etcd-io/etcd/pull/20043)
- [Add protection on `PromoteMember` and `UpdateRaftAttributes` to prevent panicking](https://github.com/etcd-io/etcd/pull/20051)
- [Fix the issue that `--force-new-cluster` can't remove all other members in a corner case](https://github.com/etcd-io/etcd/pull/20071)
- Fix [mvcc: avoid double decrement of watcher gauge on close/cancel race](https://github.com/etcd-io/etcd/pull/20067)
- [Add validation to ensure there is no empty v3discovery endpoint](https://github.com/etcd-io/etcd/pull/20113)

### etcdctl

- Fix [command `etcdctl endpoint health` doesn't work when options are set via environment variables](https://github.com/etcd-io/etcd/pull/20121)

### Dependencies

- Compile binaries using [go 1.23.10](https://github.com/etcd-io/etcd/pull/20128).

---

## v3.6.0 (2025-05-15)

There isn't any production code change since v3.6.0-rc.5.

---

## v3.6.0-rc.5 (2025-05-08)

### etcd server

- Fix [the compaction pause duration metric is not emitted for every compaction batch](https://github.com/etcd-io/etcd/pull/19770)

### Package `clientv3`

- [Replace `resolver.State.Addresses` with `resolver.State.Endpoint.Addresses`](https://github.com/etcd-io/etcd/pull/19782).
- [Deprecated the Metadata field in the Endpoint struct from the client/v3/naming/endpoints package](https://github.com/etcd-io/etcd/pull/19842).

### Dependencies

- Compile binaries using [go 1.23.9](https://github.com/etcd-io/etcd/pull/19867).

---

## v3.6.0-rc.4 (2025-04-15)

### etcd server

- [Switch to validating v3 when v2 and v3 are synchronized](https://github.com/etcd-io/etcd/pull/19703).

### Dependencies

- Compile binaries using [go 1.23.8](https://github.com/etcd-io/etcd/pull/19724)

---

## v3.6.0-rc.3 (2025-03-27)

### etcd server

- [Auto sync members in v3store for the issues which have already been affected by #19557](https://github.com/etcd-io/etcd/pull/19636).
- [Move `client/internal/v2` into `server/internel/clientv2`](https://github.com/etcd-io/etcd/pull/19673).
- [Replace ExperimentalMaxLearners with a Feature Gate](https://github.com/etcd-io/etcd/pull/19560).

### etcd grpc-proxy

- Fix [grpcproxy can get stuck in and endless loop causing high CPU usage](https://github.com/etcd-io/etcd/pull/19562)

### Dependencies

- Bump [github.com/golang-jwt/jwt/v5 from 5.2.1 to 5.2.2 to address CVE-2025-30204](https://github.com/etcd-io/etcd/pull/19647).
- Bump [bump golang.org/x/net from v0.37.0 to v0.38.0 to address CVE-2025-22872](https://github.com/etcd-io/etcd/pull/19687).

---

## v3.6.0-rc.2 (2025-03-05)

### etcd server

- Add [Prometheus metric to query server feature gates](https://github.com/etcd-io/etcd/pull/19495).

### Dependencies

- Compile binaries using [go 1.23.7](https://github.com/etcd-io/etcd/pull/19527).
- Bump [golang.org/x/net to v0.36.0 to address CVE-2025-22870](https://github.com/etcd-io/etcd/pull/19531).
- Bump [github.com/grpc-ecosystem/grpc-gateway/v2 to v2.26.3 to fix the issue of etcdserver crashing on receiving REST watch stream requests](https://github.com/etcd-io/etcd/pull/19522).

---

## v3.6.0-rc.1 (2025-02-25)

### etcdctl v3

- Add [`DowngradeInfo` in result of endpoint status](https://github.com/etcd-io/etcd/pull/19471)

### etcd server

- Add [`DowngradeInfo` to endpoint status response](https://github.com/etcd-io/etcd/pull/19471)

### Dependencies

- Bump [golang.org/x/crypto to v0.35.0 to address CVE-2025-22869](https://github.com/etcd-io/etcd/pull/19480).

---

## v3.6.0-rc.0 (2025-02-13)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.5.0...v3.6.0).

### Breaking Changes

- `etcd` will no longer start on data dir created by newer versions (for example etcd v3.6 will not run on v3.7+ data dir). To downgrade data dir please check out `etcdutl migrate` command.
- `etcd` doesn't support serving client requests on the peer listen endpoints (--listen-peer-urls). See [pull/13565](https://github.com/etcd-io/etcd/pull/13565).
- `etcdctl` will sleep(2s) in case of range delete without `--range` flag. See [pull/13747](https://github.com/etcd-io/etcd/pull/13747)
- Applications which depend on etcd v3.6 packages must be built with go version >= v1.18.

#### Flags Removed

- The following flags have been removed:

  - `--enable-v2`
  - `--experimental-enable-v2v3`
  - `--proxy`
  - `--proxy-failure-wait`
  - `--proxy-refresh-interval`
  - `--proxy-dial-timeout`
  - `--proxy-write-timeout`
  - `--proxy-read-timeout`

### Deprecations

- Deprecated [V2 discovery](https://etcd.io/docs/v3.5/dev-internal/discovery_protocol/).
- Deprecated [SetKeepAlive and SetKeepAlivePeriod in limitListenerConn](https://github.com/etcd-io/etcd/pull/14356).
- Removed [etcdctl defrag --data-dir](https://github.com/etcd-io/etcd/pull/13793).
- Removed [etcdctl snapshot status](https://github.com/etcd-io/etcd/pull/13809).
- Removed [etcdctl snapshot restore](https://github.com/etcd-io/etcd/pull/13809).
- Removed [NewZapCoreLoggerBuilder in server/embed](https://github.com/etcd-io/etcd/pull/19404)

### etcdctl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13133).
- When print endpoint status, [show db size in use](https://github.com/etcd-io/etcd/pull/13639)
- [Always print the raft_term in decimal](https://github.com/etcd-io/etcd/pull/13711) when displaying member list in json.
- [Add one more field `storageVersion`](https://github.com/etcd-io/etcd/pull/13773) into the response of command `etcdctl endpoint status`.
- Add [`--max-txn-ops`](https://github.com/etcd-io/etcd/pull/14340) flag to make-mirror command.
- Add [`--consistency`](https://github.com/etcd-io/etcd/pull/15261) flag to member list command.
- Display [field `hash_revision`](https://github.com/etcd-io/etcd/pull/14812) for `etcdctl endpoint hash` command.
- Add [`--max-request-bytes` and `--max-recv-bytes`](https://github.com/etcd-io/etcd/pull/18718) global flags.

### etcdutl v3

- Add command to generate [shell completion](https://github.com/etcd-io/etcd/pull/13142).
- Add `migrate` command for downgrading/upgrading etcd data dir files.
- Add [optional --bump-revision and --mark-compacted flag to etcdutl snapshot restore operation](https://github.com/etcd-io/etcd/pull/16029).
- Add [hashkv](https://github.com/etcd-io/etcd/pull/15965) command to print hash of keys and values up to given revision
- Removed [legacy etcdutl backup](https://github.com/etcd-io/etcd/pull/16662)
- [Count the number of keys from users perspective](https://github.com/etcd-io/etcd/pull/19344)

### Package `clientv3`

- [Support serializable `MemberList` operation](https://github.com/etcd-io/etcd/pull/15261).

### Package `server`

- Package `mvcc` was moved to `storage/mvcc`
- Package `mvcc/backend` was moved to `storage/backend`
- Package `mvcc/buckets` was moved to `storage/schema`
- Package `wal` was moved to `storage/wal`
- Package `datadir` was moved to `storage/datadir`

### Package `raft`
- [Decouple raft from etcd](https://github.com/etcd-io/etcd/issues/14713). Migrated raft to a separate [repository](https://github.com/etcd-io/raft), and renamed raft module to `go.etcd.io/raft/v3`.

### etcd server

- Add [`etcd --log-format`](https://github.com/etcd-io/etcd/pull/13339) flag to support log format.
- Add [`etcd --experimental-max-learners`](https://github.com/etcd-io/etcd/pull/13377) flag to allow configuration of learner max membership.
- Add [`etcd --experimental-enable-lease-checkpoint-persist`](https://github.com/etcd-io/etcd/pull/13508) flag to handle upgrade from v3.5.2 clusters with this feature enabled.
- Add [`etcdctl make-mirror --rev`](https://github.com/etcd-io/etcd/pull/13519) flag to support incremental mirror.
- Add [v3 discovery](https://github.com/etcd-io/etcd/pull/13635) to bootstrap a new etcd cluster.
- Add [field `storage`](https://github.com/etcd-io/etcd/pull/13772) into the response body of endpoint `/version`.
- Add [`etcd --max-concurrent-streams`](https://github.com/etcd-io/etcd/pull/14169) flag to configure the max concurrent streams each client can open at a time, and defaults to math.MaxUint32.
- Add [`etcd grpc-proxy --experimental-enable-grpc-logging`](https://github.com/etcd-io/etcd/pull/14266) flag to logging all grpc requests and responses.
- Add [`etcd --experimental-compact-hash-check-enabled --experimental-compact-hash-check-time`](https://github.com/etcd-io/etcd/issues/14039) flags to support enabling reliable corruption detection on compacted revisions.
- Add [Protection on maintenance request when auth is enabled](https://github.com/etcd-io/etcd/pull/14663).
- Graduated [`--experimental-warning-unary-request-duration` to `--warning-unary-request-duration`](https://github.com/etcd-io/etcd/pull/14414). Note the experimental flag is deprecated and will be decommissioned in v3.7.
- Add [field `hash_revision` into `HashKVResponse`](https://github.com/etcd-io/etcd/pull/14537).
- Add [`etcd --experimental-snapshot-catch-up-entries`](https://github.com/etcd-io/etcd/pull/15033) flag to configure number of entries for a slow follower to catch up after compacting the raft storage entries and defaults to 5k. 
- Decreased [`--snapshot-count` default value from 100,000 to 10,000](https://github.com/etcd-io/etcd/pull/15408)
- Add [`etcd --tls-min-version --tls-max-version`](https://github.com/etcd-io/etcd/pull/15156) to enable support for TLS 1.3.
- Add [quota to endpoint status response](https://github.com/etcd-io/etcd/pull/17877)
- Add [feature gate `SetMemberLocalAddr`](https://github.com/etcd-io/etcd/pull/19413) to [enable using the first specified and non-loopback local address from initial-advertise-peer-urls as the local address when communicating with a peer]((https://github.com/etcd-io/etcd/pull/17661))
- Add [Support multiple values for allowed client and peer TLS identities](https://github.com/etcd-io/etcd/pull/18015)
- Add [`embed.Config.GRPCAdditionalServerOptions`](https://github.com/etcd-io/etcd/pull/14066) to support updating the default internal gRPC configuration for embedded use cases.

### etcd grpc-proxy

- Add [`etcd grpc-proxy start --endpoints-auto-sync-interval`](https://github.com/etcd-io/etcd/pull/14354) flag to enable and configure interval of auto sync of endpoints with server.
- Add [`etcd grpc-proxy start --listen-cipher-suites`](https://github.com/etcd-io/etcd/pull/14308) flag to support adding configurable cipher list.
- Add [`tls min/max version to grpc proxy`](https://github.com/etcd-io/etcd/pull/18816) to support setting TLS min and max version.

### tools/benchmark

- [Add etcd client autoSync flag](https://github.com/etcd-io/etcd/pull/13416)

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/latest/metrics/) for all metrics per release.

- Add [`etcd_disk_defrag_inflight`](https://github.com/etcd-io/etcd/pull/13371).
- Add [`etcd_debugging_server_alarms`](https://github.com/etcd-io/etcd/pull/14276).
- Add [`etcd_server_range_duration_seconds`](https://github.com/etcd-io/etcd/pull/17983).

### Go
- Require [Go 1.23+](https://github.com/etcd-io/etcd/pull/16594).
- Compile with [Go 1.23+](https://go.dev/doc/devel/release#go1.21.minor). Please refer to [gc-guide](https://go.dev/doc/gc-guide) to configure `GOGC` and `GOMEMLIMIT` properly. 

### Other

- Use Distroless as base image to make the image less vulnerable and reduce image size.
- [Upgrade grpc-gateway from v1 to v2](https://github.com/etcd-io/etcd/pull/16595).
- [Switch from grpc-ecosystem/go-grpc-prometheus to grpc-ecosystem/go-grpc-middleware/providers/prometheus](https://github.com/etcd-io/etcd/pull/19195).

---
