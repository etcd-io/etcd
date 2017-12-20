## [v3.3.0](https://github.com/coreos/etcd/releases/tag/v3.3.0)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.0...v3.3.0) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

### Improved

- Use [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to replace [`boltdb/bolt`](https://github.com/boltdb/bolt#project-status).
  - Fix [etcd database size grows until `mvcc: database space exceeded`](https://github.com/coreos/etcd/issues/8009).
- [Reduce memory allocation](https://github.com/coreos/etcd/pull/8428) on [Range operations](https://github.com/coreos/etcd/pull/8475).
- [Rate limit](https://github.com/coreos/etcd/pull/8099) and [randomize](https://github.com/coreos/etcd/pull/8101) lease revoke on restart or leader elections.
  - Prevent [spikes in Raft proposal rate](https://github.com/coreos/etcd/issues/8096).
- Support `clientv3` balancer failover under [network faults/partitions](https://github.com/coreos/etcd/issues/8711).
- Better warning on [mismatched `--initial-cluster`](https://github.com/coreos/etcd/pull/8083) flag.

### Changed(Breaking Changes)

- Require [Go 1.9+](https://github.com/coreos/etcd/issues/6174).
  - Compile with *Go 1.9.2*.
  - Deprecate [`golang.org/x/net/context`](https://github.com/coreos/etcd/pull/8511).
- Require [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) [**`v1.7.4`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.4) or [**`v1.7.5+`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5):
  - Deprecate [`metadata.Incoming/OutgoingContext`](https://github.com/coreos/etcd/pull/7896).
  - Deprecate `grpclog.Logger`, upgrade to [`grpclog.LoggerV2`](https://github.com/coreos/etcd/pull/8533).
  - Deprecate [`grpc.ErrClientConnTimeout`](https://github.com/coreos/etcd/pull/8505) errors in `clientv3`.
  - Use [`MaxRecvMsgSize` and `MaxSendMsgSize`](https://github.com/coreos/etcd/pull/8437) to limit message size, in etcd server.
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) `v1.2.2` to `v1.3.0`.
- Translate [gRPC status error in v3 client `Snapshot` API](https://github.com/coreos/etcd/pull/9038).
- Upgrade [`github.com/ugorji/go/codec`](https://github.com/ugorji/go) for v2 `client`.
  - [Regenerated](https://github.com/coreos/etcd/pull/8721) v2 `client` source code with latest `ugorji/go/codec`.
- Fix [`/health` endpoint JSON output](https://github.com/coreos/etcd/pull/8312).
- v3 `etcdctl` [`lease timetolive LEASE_ID`](https://github.com/coreos/etcd/issues/9028) on expired lease now prints [`lease LEASE_ID already expired`](https://github.com/coreos/etcd/pull/9047).
  - <=3.2 prints `lease LEASE_ID granted with TTL(0s), remaining(-1s)`.

### Added(`etcd`)

- Add [`--experimental-enable-v2v3`](https://github.com/coreos/etcd/pull/8407) flag to [emulate v2 API with v3](https://github.com/coreos/etcd/issues/6925).
- Add [`--experimental-corrupt-check-time`](https://github.com/coreos/etcd/pull/8420) flag to [raise corrupt alarm monitoring](https://github.com/coreos/etcd/issues/7125).
- Add [`--experimental-initial-corrupt-check`](https://github.com/coreos/etcd/pull/8554) flag to [check database hash before serving client/peer traffic](https://github.com/coreos/etcd/issues/8313).
- Add [`--max-txn-ops`](https://github.com/coreos/etcd/pull/7976) flag to [configure maximum number operations in transaction](https://github.com/coreos/etcd/issues/7826).
- Add [`--max-request-bytes`](https://github.com/coreos/etcd/pull/7968) flag to [configure maximum client request size](https://github.com/coreos/etcd/issues/7923).
  - If not configured, it defaults to 1.5 MiB.
- Add [`--client-crl-file`, `--peer-crl-file`](https://github.com/coreos/etcd/pull/8124) flags for [Certificate revocation list](https://github.com/coreos/etcd/issues/4034).
- Add [`--peer-require-cn`](https://github.com/coreos/etcd/pull/8616) flag to support [CN-based auth for inter-peer connection](https://github.com/coreos/etcd/issues/8262).
- Add [`--listen-metrics-urls`](https://github.com/coreos/etcd/pull/8242) flag for additional `/metrics` endpoints.
  - Support [additional (non) TLS `/metrics` endpoints for a TLS-enabled cluster](https://github.com/coreos/etcd/pull/8282).
  - e.g. `--listen-metrics-urls=https://localhost:2378,http://localhost:9379` to serve `/metrics` in secure port 2378 and insecure port 9379.
  - Useful for [bypassing critical APIs when monitoring etcd](https://github.com/coreos/etcd/issues/8060).
- Add [`--auto-compaction-mode`](https://github.com/coreos/etcd/pull/8123) flag to [support revision-based compaction](https://github.com/coreos/etcd/issues/8098).
- Change `--auto-compaction-retention` flag to [accept string values](https://github.com/coreos/etcd/pull/8563) with [finer granularity](https://github.com/coreos/etcd/issues/8503).
- Add [`--grpc-keepalive-min-time`, `--grpc-keepalive-interval`, `--grpc-keepalive-timeout`](https://github.com/coreos/etcd/pull/8535) flags to configure server-side keepalive policies.
- Serve [`/health` endpoint as unhealthy](https://github.com/coreos/etcd/pull/8272) when [alarm is raised](https://github.com/coreos/etcd/issues/8207).
- Provide [error information in `/health`](https://github.com/coreos/etcd/pull/8312).
  - e.g. `{"health":false,"errors":["NOSPACE"]}`.
- Move [logging setup to embed package](https://github.com/coreos/etcd/pull/8810)
  - Disable gRPC server log by default.
- Use [monotonic time in Go 1.9](https://github.com/coreos/etcd/pull/8507) for `lease` package.
- Warn on [empty hosts in advertise URLs](https://github.com/coreos/etcd/pull/8384).
  - Address [advertise client URLs accepts empty hosts](https://github.com/coreos/etcd/issues/8379).
  - etcd `v3.4` will exit on this error.
    - e.g. `--advertise-client-urls=http://:2379`.
- Warn on [shadowed environment variables](https://github.com/coreos/etcd/pull/8385).
  - Address [error on shadowed environment variables](https://github.com/coreos/etcd/issues/8380).
  - etcd `v3.4` will exit on this error.

### Added(API)

- Support [ranges in transaction comparisons](https://github.com/coreos/etcd/pull/8025) for [disconnected linearized reads](https://github.com/coreos/etcd/issues/7924).
- Add [nested transactions](https://github.com/coreos/etcd/pull/8102) to extend [proxy use cases](https://github.com/coreos/etcd/issues/7857).
- Add [lease comparison target in transaction](https://github.com/coreos/etcd/pull/8324).
- Add [lease list](https://github.com/coreos/etcd/pull/8358).
- Add [hash by revision](https://github.com/coreos/etcd/pull/8263) for [better corruption checking against boltdb](https://github.com/coreos/etcd/issues/8016).

### Added(`etcd/clientv3`)

- Add [health balancer](https://github.com/coreos/etcd/pull/8545) to fix [watch API hangs](https://github.com/coreos/etcd/issues/7247), improve [endpoint switch under network faults](https://github.com/coreos/etcd/issues/7941).
- [Refactor balancer](https://github.com/coreos/etcd/pull/8840) and add [client-side keepalive pings](https://github.com/coreos/etcd/pull/8199) to handle [network partitions](https://github.com/coreos/etcd/issues/8711).
- Add [`MaxCallSendMsgSize` and `MaxCallRecvMsgSize`](https://github.com/coreos/etcd/pull/9047) fields to [`clientv3.Config`](https://godoc.org/github.com/coreos/etcd/clientv3#Config).
  - Fix [exceeded response size limit error in client-side](https://github.com/coreos/etcd/issues/9043).
  - Address [kubernetes#51099](https://github.com/kubernetes/kubernetes/issues/51099).
  - `MaxCallSendMsgSize` default value is 2 MiB, if not configured.
  - `MaxCallRecvMsgSize` default value is `math.MaxInt32`, if not configured.
- Accept [`Compare_LEASE`](https://github.com/coreos/etcd/pull/8324) in [`clientv3.Compare`](https://godoc.org/github.com/coreos/etcd/clientv3#Compare).
- Add [`LeaseValue` helper](https://github.com/coreos/etcd/pull/8488) to `Cmp` `LeaseID` values in `Txn`.
- Add [`MoveLeader`](https://github.com/coreos/etcd/pull/8153) to `Maintenance`.
- Add [`HashKV`](https://github.com/coreos/etcd/pull/8351) to `Maintenance`.
- Add [`Leases`](https://github.com/coreos/etcd/pull/8358) to `Lease`.
- Add [`clientv3/ordering`](https://github.com/coreos/etcd/pull/8092) for enforce [ordering in serialized requests](https://github.com/coreos/etcd/issues/7623).

### Added(v2 `etcdctl`)

- Add [`backup --with-v3`](https://github.com/coreos/etcd/pull/8479) flag.

### Added(v3 `etcdctl`)

- Add [`--discovery-srv`](https://github.com/coreos/etcd/pull/8462) flag.
- Add [`--keepalive-time`, `--keepalive-timeout`](https://github.com/coreos/etcd/pull/8663) flags.
- Add [`lease list`](https://github.com/coreos/etcd/pull/8358) command.
- Add [`lease keep-alive --once`](https://github.com/coreos/etcd/pull/8775) flag.
- Make [`lease timetolive LEASE_ID`](https://github.com/coreos/etcd/issues/9028) on expired lease print [`lease LEASE_ID already expired`](https://github.com/coreos/etcd/pull/9047).
  - <=3.2 prints `lease LEASE_ID granted with TTL(0s), remaining(-1s)`.
- Add [`defrag --data-dir`](https://github.com/coreos/etcd/pull/8367) flag.
- Add [`move-leader`](https://github.com/coreos/etcd/pull/8153) command.
- Add [`endpoint hashkv`](https://github.com/coreos/etcd/pull/8351) command.
- Add [`endpoint --cluster`](https://github.com/coreos/etcd/pull/8143) flag, equivalent to [v2 `etcdctl cluster-health`](https://github.com/coreos/etcd/issues/8117).
- Make `endpoint health` command terminate with [non-zero exit code on unhealthy status](https://github.com/coreos/etcd/pull/8342).
- Add [`lock --ttl`](https://github.com/coreos/etcd/pull/8370) flag.
- Support [`watch [key] [range_end] -- [exec-command…]`](https://github.com/coreos/etcd/pull/8919), equivalent to [v2 `etcdctl exec-watch`](https://github.com/coreos/etcd/issues/8814).
- Enable [`clientv3.WithRequireLeader(context.Context)` for `watch`](https://github.com/coreos/etcd/pull/8672) command.
- Print [`"del"` instead of `"delete"`](https://github.com/coreos/etcd/pull/8297) in `txn` interactive mode.
- Print [`ETCD_INITIAL_ADVERTISE_PEER_URLS` in `member add`](https://github.com/coreos/etcd/pull/8332).

### Added(metrics)

- Add [`etcd --listen-metrics-urls`](https://github.com/coreos/etcd/pull/8242) flag for additional `/metrics` endpoints.
  - Useful for [bypassing critical APIs when monitoring etcd](https://github.com/coreos/etcd/issues/8060).
- Add [`etcd_server_version`](https://github.com/coreos/etcd/pull/8960) Prometheus metric.
  - To replace [Kubernetes `etcd-version-monitor`](https://github.com/coreos/etcd/issues/8948).
- Add [`etcd_debugging_mvcc_db_compaction_keys_total`](https://github.com/coreos/etcd/pull/8280) Prometheus metric.
- Add [`etcd_debugging_server_lease_expired_total`](https://github.com/coreos/etcd/pull/8064) Prometheus metric.
  - To improve [lease revoke monitoring](https://github.com/coreos/etcd/issues/8050).
- Document [Prometheus 2.0 rules](https://github.com/coreos/etcd/pull/8879).
- Initialize gRPC server [metrics with zero values](https://github.com/coreos/etcd/pull/8878).

### Added(`grpc-proxy`)

- Add [`grpc-proxy start --experimental-leasing-prefix`](https://github.com/coreos/etcd/pull/8341) flag:
  - For disconnected linearized reads.
  - Based on [V system leasing](https://github.com/coreos/etcd/issues/6065).
  - See ["Disconnected consistent reads with etcd" blog post](https://coreos.com/blog/coreos-labs-disconnected-consistent-reads-with-etcd).
- Add [`grpc-proxy start --experimental-serializable-ordering`](https://github.com/coreos/etcd/pull/8315) flag.
  - To ensure serializable reads have monotonically increasing store revisions across endpoints.
- Add [`grpc-proxy start --metrics-addr`](https://github.com/coreos/etcd/pull/8242) flag for an additional `/metrics` endpoint.
  - Set `--metrics-addr=http://[HOST]:9379` to serve `/metrics` in insecure port 9379.
- Serve [`/health` endpoint in grpc-proxy](https://github.com/coreos/etcd/pull/8322).
- Add [`grpc-proxy start --debug`](https://github.com/coreos/etcd/pull/8994) flag.

### Added(gRPC gateway)

- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint with [`/v3beta`](https://github.com/coreos/etcd/pull/8880).
  - To deprecate [`/v3alpha`](https://github.com/coreos/etcd/issues/8125) in `v3.4`.
- Support ["authorization" token](https://github.com/coreos/etcd/pull/7999).
- Support [websocket for bi-directional streams](https://github.com/coreos/etcd/pull/8257).
  - Fix [`Watch` API with gRPC gateway](https://github.com/coreos/etcd/issues/8237).
- Upgrade gRPC gateway to [v1.3.0](https://github.com/coreos/etcd/issues/8838).

### Added(`etcd/raft`)

- Add [non-voting member](https://github.com/coreos/etcd/pull/8751).
  - To implement [Raft thesis 4.2.1 Catching up new servers](https://github.com/coreos/etcd/issues/8568).
  - `Learner` node does not vote or promote itself.

### Added/Fixed(Security/Auth)

- Add [CRL based connection rejection](https://github.com/coreos/etcd/pull/8124) to manage [revoked certs](https://github.com/coreos/etcd/issues/4034).
- Document [TLS authentication changes](https://github.com/coreos/etcd/pull/8895):
  - [Server accepts connections if IP matches, without checking DNS entries](https://github.com/coreos/etcd/pull/8223). For instance, if peer cert contains IP addresses and DNS names in Subject Alternative Name (SAN) field, and the remote IP address matches one of those IP addresses, server just accepts connection without further checking the DNS names.
  - [Server supports reverse-lookup on wildcard DNS `SAN`](https://github.com/coreos/etcd/pull/8281). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server first reverse-lookups the remote IP address to get a list of names mapping to that address (e.g. `nslookup IPADDR`). Then accepts the connection if those names have a matching name with peer cert's DNS names (either by exact or wildcard match). If none is matched, server forward-lookups each DNS entry in peer cert (e.g. look up `example.default.svc` when the entry is `*.example.default.svc`), and accepts connection only when the host's resolved addresses have the matching IP address with the peer's remote IP address.
- Add [`etcd --peer-require-cn`](https://github.com/coreos/etcd/pull/8616) flag.
  - To support [CommonName(CN) based auth](https://github.com/coreos/etcd/issues/8262) for inter peer connection.
- [Swap priority](https://github.com/coreos/etcd/pull/8594) of cert CommonName(CN) and username + password.
  - To address ["username and password specified in the request should take priority over CN in the cert"](https://github.com/coreos/etcd/issues/8584).
- Protect [lease revoke with auth](https://github.com/coreos/etcd/pull/8031).
- Provide user's role on [auth permission error](https://github.com/coreos/etcd/pull/8164).
- Fix [auth store panic with disabled token](https://github.com/coreos/etcd/pull/8695).
- Update `golang.org/x/crypto/bcrypt` (see [golang/crypto@6c586e1](https://github.com/golang/crypto/commit/6c586e17d90a7d08bbbc4069984180dce3b04117)).

### Fixed(v2)

- [Fail-over v2 client](https://github.com/coreos/etcd/pull/8519) to next endpoint on [oneshot failure](https://github.com/coreos/etcd/issues/8515).
- [Put back `/v2/machines`](https://github.com/coreos/etcd/pull/8062) endpoint for python-etcd wrapper.

### Fixed(v3)

- Fix [range/put/delete operation metrics](https://github.com/coreos/etcd/pull/8054) with transaction:
  - `etcd_debugging_mvcc_range_total`
  - `etcd_debugging_mvcc_put_total`
  - `etcd_debugging_mvcc_delete_total`
  - `etcd_debugging_mvcc_txn_total`
- Fix [`etcd_debugging_mvcc_keys_total`](https://github.com/coreos/etcd/pull/8390) on restore.
- Fix [`etcd_debugging_mvcc_db_total_size_in_bytes`](https://github.com/coreos/etcd/pull/8120) on restore.
  - Also change to [`prometheus.NewGaugeFunc`](https://github.com/coreos/etcd/pull/8150).
- Fix [backend database in-memory index corruption](https://github.com/coreos/etcd/pull/8127) issue on restore (only 3.2.0 is affected).
- Fix [watch restore from snapshot](https://github.com/coreos/etcd/pull/8427).
- Fix ["put at-most-once" in `clientv3`](https://github.com/coreos/etcd/pull/8335).
- Handle [empty key permission](https://github.com/coreos/etcd/pull/8514) in `etcdctl`.
- [Fix server crash](https://github.com/coreos/etcd/pull/8010) on [invalid transaction request from gRPC gateway](https://github.com/coreos/etcd/issues/7889).
- Fix [`clientv3.WatchResponse.Canceled`](https://github.com/coreos/etcd/pull/8283) on [compacted watch request](https://github.com/coreos/etcd/issues/8231).
- Handle [WAL renaming failure on Windows](https://github.com/coreos/etcd/pull/8286).
- Make [peer dial timeout longer](https://github.com/coreos/etcd/pull/8599).
  - See [coreos/etcd-operator#1300](https://github.com/coreos/etcd-operator/issues/1300) for more detail.
- Make server [wait up to request time-out](https://github.com/coreos/etcd/pull/8267) with [pending RPCs](https://github.com/coreos/etcd/issues/8224).
- Fix [`grpc.Server` panic on `GracefulStop`](https://github.com/coreos/etcd/pull/8987) with [TLS-enabled server](https://github.com/coreos/etcd/issues/8916).
- Fix ["multiple peer URLs cannot start" issue](https://github.com/coreos/etcd/issues/8383).
- Fix server-side auth so [concurrent auth operations do not return old revision error](https://github.com/coreos/etcd/pull/8442).
- Fix [`concurrency/stm` `Put` with serializable snapshot](https://github.com/coreos/etcd/pull/8439).
  - Use store revision from first fetch to resolve write conflicts instead of modified revision.
- Fix [`grpc-proxy` Snapshot API error handling](https://github.com/coreos/etcd/commit/dbd16d52fbf81e5fd806d21ff5e9148d5bf203ab).
- Fix [`grpc-proxy` KV API `PrevKv` flag handling](https://github.com/coreos/etcd/pull/8366).
- Fix [`grpc-proxy` KV API `KeysOnly` flag handling](https://github.com/coreos/etcd/pull/8552).
- Upgrade [`coreos/go-systemd`](https://github.com/coreos/go-systemd/releases) to `v15` (see https://github.com/coreos/go-systemd/releases/tag/v15).

### Other

- Support previous two minor versions (see our [new release policy](https://github.com/coreos/etcd/pull/8805)).
- `v3.3.x` is the last release cycle that supports `ACI`:
  - AppC was [officially suspended](https://github.com/appc/spec#-disclaimer-), as of late 2016.
  - [`acbuild`](https://github.com/containers/build#this-project-is-currently-unmaintained) is not maintained anymore.
  - `*.aci` files won't be available from etcd `v3.4` release.
- Add container registry [`gcr.io/etcd-development/etcd`](https://gcr.io/etcd-development/etcd).
  - [quay.io/coreos/etcd](https://quay.io/coreos/etcd) is still supported as secondary.


## [v3.2.12](https://github.com/coreos/etcd/releases/tag/v3.2.12) (2017-12-20)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.11...v3.2.12) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix [error message of `Revision` compactor](https://github.com/coreos/etcd/pull/8999) in server-side.

### Added(`etcd/clientv3`,`etcdctl/v3`)

- Add [`MaxCallSendMsgSize` and `MaxCallRecvMsgSize`](https://github.com/coreos/etcd/pull/9047) fields to [`clientv3.Config`](https://godoc.org/github.com/coreos/etcd/clientv3#Config).
  - Fix [exceeded response size limit error in client-side](https://github.com/coreos/etcd/issues/9043).
  - Address [kubernetes#51099](https://github.com/kubernetes/kubernetes/issues/51099).
  - `MaxCallSendMsgSize` default value is 2 MiB, if not configured.
  - `MaxCallRecvMsgSize` default value is `math.MaxInt32`, if not configured.

### Other

- Pin [grpc v1.7.5](https://github.com/grpc/grpc-go/releases/tag/v1.7.5), [grpc-gateway v1.3.0](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3.0).
  - No code change, just to be explicit about recommended versions.


## [v3.2.11](https://github.com/coreos/etcd/releases/tag/v3.2.11) (2017-12-05)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.10...v3.2.11) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Fix racey grpc-go's server handler transport `WriteStatus` call to prevent [TLS-enabled etcd server crash](https://github.com/coreos/etcd/issues/8904):
  - Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) `v1.7.3` to `v1.7.4`.
  - Add [gRPC RPC failure warnings](https://github.com/coreos/etcd/pull/8939) to help debug such issues in the future.
- Remove `--listen-metrics-urls` flag in monitoring document (non-released in `v3.2.x`, planned for `v3.3.x`).

### Added

- Provide [more cert details](https://github.com/coreos/etcd/pull/8952/files) on TLS handshake failures.


## [v3.1.11](https://github.com/coreos/etcd/releases/tag/v3.1.11) (2017-11-28)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.10...v3.1.11) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- [#8411](https://github.com/coreos/etcd/issues/8411),[#8806](https://github.com/coreos/etcd/pull/8806) mvcc: fix watch restore from snapshot
- [#8009](https://github.com/coreos/etcd/issues/8009),[#8902](https://github.com/coreos/etcd/pull/8902) backport coreos/bbolt v1.3.1-coreos.5


## [v3.2.10](https://github.com/coreos/etcd/releases/tag/v3.2.10) (2017-11-16)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.9...v3.2.10) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Fixed

- Replace backend key-value database `boltdb/bolt` with [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to address [backend database size issue](https://github.com/coreos/etcd/issues/8009).
- Fix `clientv3` balancer to handle [network partitions](https://github.com/coreos/etcd/issues/8711):
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


## [v3.1.10](https://github.com/coreos/etcd/releases/tag/v3.1.10) (2017-07-14)

See [code changes](https://github.com/coreos/etcd/compare/v3.1.9...v3.1.10) and [v3.1 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_1.md) for any breaking changes.

### Changed

- Compile with Go 1.8.3 to fix panic on `net/http.CloseNotify`

### Added

- Tag docker images with minor versions.
  - e.g. `docker pull quay.io/coreos/etcd:v3.1` to fetch latest v3.1 versions.


## [v3.2.2](https://github.com/coreos/etcd/releases/tag/v3.2.2) (2017-07-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.1...v3.2.2) and [v3.2 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes.

### Improved

- Rate-limit lease revoke on expiration.
- Extend leases on promote to avoid queueing effect on lease expiration.

### Fixed

- Use user-provided listen address to connect to gRPC gateway:
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

### Fixed

- Allow v2 snapshot over 512MB.


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


## [v2.3.8](https://github.com/coreos/etcd/releases/tag/v2.3.8) (2017-02-17)

See [code changes](https://github.com/coreos/etcd/compare/v2.3.7...v2.3.8).

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

- Deprecated following gRPC metrics in favor of [go-grpc-prometheus](https://github.com/grpc-ecosystem/go-grpc-prometheus):
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


## [v3.0.16](https://github.com/coreos/etcd/releases/tag/v3.0.16) (2016-11-13)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.15...v3.0.16) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.


## [v3.0.15](https://github.com/coreos/etcd/releases/tag/v3.0.15) (2016-11-11)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.14...v3.0.15) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Fixed

- Fix cancel watch request with wrong range end.


## [v3.0.14](https://github.com/coreos/etcd/releases/tag/v3.0.14) (2016-11-04)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.13...v3.0.14) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Added

- v3 `etcdctl migrate` command now supports `--no-ttl` flag to discard keys on transform.


## [v3.0.13](https://github.com/coreos/etcd/releases/tag/v3.0.13) (2016-10-24)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.12...v3.0.13) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.


## [v3.0.12](https://github.com/coreos/etcd/releases/tag/v3.0.12) (2016-10-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.11...v3.0.12) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.


## [v3.0.11](https://github.com/coreos/etcd/releases/tag/v3.0.11) (2016-10-07)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.10...v3.0.11) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Added

- Server returns previous key-value (optional)
  - `clientv3.WithPrevKV` option
  - v3 etcdctl `put,watch,del --prev-kv` flag


## [v3.0.10](https://github.com/coreos/etcd/releases/tag/v3.0.10) (2016-09-23)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.9...v3.0.10) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.


## [v3.0.9](https://github.com/coreos/etcd/releases/tag/v3.0.9) (2016-09-15)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.8...v3.0.9) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Added

- Warn on domain names on listen URLs (v3.2 will reject domain names).


## [v3.0.8](https://github.com/coreos/etcd/releases/tag/v3.0.8) (2016-09-09)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.7...v3.0.8) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Changed

- Allow only IP addresses in listen URLs (domain names are rejected).


## [v3.0.7](https://github.com/coreos/etcd/releases/tag/v3.0.7) (2016-08-31)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.6...v3.0.7) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Changed

- SRV records only allow A records (RFC 2052).


## [v3.0.6](https://github.com/coreos/etcd/releases/tag/v3.0.6) (2016-08-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.5...v3.0.6) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.


## [v3.0.5](https://github.com/coreos/etcd/releases/tag/v3.0.5) (2016-08-19)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.4...v3.0.5) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Changed

- SRV records (e.g., infra1.example.com) must match the discovery domain (i.e., example.com) if no custom certificate authority is given.


## [v3.0.4](https://github.com/coreos/etcd/releases/tag/v3.0.4) (2016-07-27)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.3...v3.0.4) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Changed

- v2 auth can now use common name from TLS certificate when `--client-cert-auth` is enabled.

### Added

- v2 `etcdctl ls` command now supports `--output=json`.
- Add /var/lib/etcd directory to etcd official Docker image.


## [v3.0.3](https://github.com/coreos/etcd/releases/tag/v3.0.3) (2016-07-15)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.2...v3.0.3) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Changed

- Revert Dockerfile to use `CMD`, instead of `ENTRYPOINT`, to support `etcdctl` run.
  - Docker commands for v3.0.2 won't work without specifying executable binary paths.
- v3 etcdctl default endpoints are now `127.0.0.1:2379`.


## [v3.0.2](https://github.com/coreos/etcd/releases/tag/v3.0.2) (2016-07-08)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.1...v3.0.2) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.

### Changed

- Dockerfile uses `ENTRYPOINT`, instead of `CMD`, to run etcd without binary path specified.


## [v3.0.1](https://github.com/coreos/etcd/releases/tag/v3.0.1) (2016-07-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.0.0...v3.0.1) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.


## [v3.0.0](https://github.com/coreos/etcd/releases/tag/v3.0.0) (2016-06-30)

See [code changes](https://github.com/coreos/etcd/compare/v2.3.0...v3.0.0) and [v3.0 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_0.md) for any breaking changes.
