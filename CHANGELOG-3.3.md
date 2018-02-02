

## [v3.3.0](https://github.com/coreos/etcd/releases/tag/v3.3.0) (2018-02-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.0...v3.3.0) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

## [v3.3.0-rc.4](https://github.com/coreos/etcd/releases/tag/v3.3.0-rc.4) (2018-01-22)

See [code changes](https://github.com/coreos/etcd/compare/v3.3.0-rc.3...v3.3.0-rc.4) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

## [v3.3.0-rc.3](https://github.com/coreos/etcd/releases/tag/v3.3.0-rc.3) (2018-01-17)

See [code changes](https://github.com/coreos/etcd/compare/v3.3.0-rc.2...v3.3.0-rc.3) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

## [v3.3.0-rc.2](https://github.com/coreos/etcd/releases/tag/v3.3.0-rc.2) (2018-01-11)

See [code changes](https://github.com/coreos/etcd/compare/v3.3.0-rc.1...v3.3.0-rc.2) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

## [v3.3.0-rc.1](https://github.com/coreos/etcd/releases/tag/v3.3.0-rc.1) (2018-01-02)

See [code changes](https://github.com/coreos/etcd/compare/v3.3.0-rc.0...v3.3.0-rc.1) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

## [v3.3.0-rc.0](https://github.com/coreos/etcd/releases/tag/v3.3.0-rc.0) (2017-12-20)

See [code changes](https://github.com/coreos/etcd/compare/v3.2.0...v3.3.0-rc.0) and [v3.3 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes.

### Improved

- Use [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to replace [`boltdb/bolt`](https://github.com/boltdb/bolt#project-status).
  - Fix [etcd database size grows until `mvcc: database space exceeded`](https://github.com/coreos/etcd/issues/8009).
- [Support database size larger than 8GiB](https://github.com/coreos/etcd/pull/7525) (8GiB is now a suggested maximum size for normal environments)
- [Reduce memory allocation](https://github.com/coreos/etcd/pull/8428) on [Range operations](https://github.com/coreos/etcd/pull/8475).
- [Rate limit](https://github.com/coreos/etcd/pull/8099) and [randomize](https://github.com/coreos/etcd/pull/8101) lease revoke on restart or leader elections.
  - Prevent [spikes in Raft proposal rate](https://github.com/coreos/etcd/issues/8096).
- Support `clientv3` balancer failover under [network faults/partitions](https://github.com/coreos/etcd/issues/8711).
- Better warning on [mismatched `--initial-cluster`](https://github.com/coreos/etcd/pull/8083) flag.
  - etcd compares `--initial-advertise-peer-urls` against corresponding `--initial-cluster` URLs with forward-lookup.
  - If resolved IP addresses of `--initial-advertise-peer-urls` and `--initial-cluster` do not match (e.g. [due to DNS error](https://github.com/coreos/etcd/pull/9210)), etcd will exit with errors.
    - v3.2 error: `--initial-cluster must include s1=https://s1.test:2380 given --initial-advertise-peer-urls=https://s1.test:2380`.
    - v3.3 error: `failed to resolve https://s1.test:2380 to match --initial-cluster=s1=https://s1.test:2380 (failed to resolve "https://s1.test:2380" (error ...))`.

### Changed(Breaking Changes)

- Require [Go 1.9+](https://github.com/coreos/etcd/issues/6174).
  - Compile with *Go 1.9.3*.
  - Deprecate [`golang.org/x/net/context`](https://github.com/coreos/etcd/pull/8511).
- Require [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) [**`v1.7.4`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.4) or [**`v1.7.5`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5).
  - Deprecate [`metadata.Incoming/OutgoingContext`](https://github.com/coreos/etcd/pull/7896).
  - Deprecate `grpclog.Logger`, upgrade to [`grpclog.LoggerV2`](https://github.com/coreos/etcd/pull/8533).
  - Deprecate [`grpc.ErrClientConnTimeout`](https://github.com/coreos/etcd/pull/8505) errors in `clientv3`.
  - Use [`MaxRecvMsgSize` and `MaxSendMsgSize`](https://github.com/coreos/etcd/pull/8437) to limit message size, in etcd server.
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) `v1.2.2` to `v1.3.0`.
- Translate [gRPC status error in v3 client `Snapshot` API](https://github.com/coreos/etcd/pull/9038).
- Upgrade [`github.com/ugorji/go/codec`](https://github.com/ugorji/go) for v2 `client`.
  - [Regenerated](https://github.com/coreos/etcd/pull/8721) v2 `client` source code with latest `ugorji/go/codec`.
- v3 `etcdctl` [`lease timetolive LEASE_ID`](https://github.com/coreos/etcd/issues/9028) on expired lease now prints [`"lease LEASE_ID already expired"`](https://github.com/coreos/etcd/pull/9047).
  - <=3.2 prints `"lease LEASE_ID granted with TTL(0s), remaining(-1s)"`.

### Added(`etcd`)

- Add [`--experimental-enable-v2v3`](https://github.com/coreos/etcd/pull/8407) flag to [emulate v2 API with v3](https://github.com/coreos/etcd/issues/6925).
- Add [`--experimental-corrupt-check-time`](https://github.com/coreos/etcd/pull/8420) flag to [raise corrupt alarm monitoring](https://github.com/coreos/etcd/issues/7125).
- Add [`--experimental-initial-corrupt-check`](https://github.com/coreos/etcd/pull/8554) flag to [check database hash before serving client/peer traffic](https://github.com/coreos/etcd/issues/8313).
- Add [`--max-txn-ops`](https://github.com/coreos/etcd/pull/7976) flag to [configure maximum number operations in transaction](https://github.com/coreos/etcd/issues/7826).
- Add [`--max-request-bytes`](https://github.com/coreos/etcd/pull/7968) flag to [configure maximum client request size](https://github.com/coreos/etcd/issues/7923).
  - If not configured, it defaults to 1.5 MiB.
- Add [`--client-crl-file`, `--peer-crl-file`](https://github.com/coreos/etcd/pull/8124) flags for [Certificate revocation list](https://github.com/coreos/etcd/issues/4034).
- Add [`--peer-cert-allowed-cn`](https://github.com/coreos/etcd/pull/8616) flag to support [CN-based auth for inter-peer connection](https://github.com/coreos/etcd/issues/8262).
- Add [`--listen-metrics-urls`](https://github.com/coreos/etcd/pull/8242) flag for additional `/metrics` endpoints.
  - Support [additional (non) TLS `/metrics` endpoints for a TLS-enabled cluster](https://github.com/coreos/etcd/pull/8282).
  - e.g. `--listen-metrics-urls=https://localhost:2378,http://localhost:9379` to serve `/metrics` in secure port 2378 and insecure port 9379.
  - Useful for [bypassing critical APIs when monitoring etcd](https://github.com/coreos/etcd/issues/8060).
- Add [`--auto-compaction-mode`](https://github.com/coreos/etcd/pull/8123) flag to [support revision-based compaction](https://github.com/coreos/etcd/issues/8098).
- Change `--auto-compaction-retention` flag to [accept string values](https://github.com/coreos/etcd/pull/8563) with [finer granularity](https://github.com/coreos/etcd/issues/8503).
  - e.g. `--auto-compaction-mode=periodic --auto-compaction-retention=30m` automatically `Compact` on latest revision every 30-minute.
  - e.g. `--auto-compaction-mode=revision --auto-compaction-retention=1000` automatically `Compact` on `"latest revision" - 1000` every 5-minute (when latest revision is 30000, compact on revision 29000).
- Add [`--grpc-keepalive-min-time`, `--grpc-keepalive-interval`, `--grpc-keepalive-timeout`](https://github.com/coreos/etcd/pull/8535) flags to configure server-side keepalive policies.
- Serve [`/health` endpoint as unhealthy](https://github.com/coreos/etcd/pull/8272) when [alarm (e.g. `NOSPACE`) is raised or there's no leader](https://github.com/coreos/etcd/issues/8207).
  - Define [`etcdhttp.Health`](https://godoc.org/github.com/coreos/etcd/etcdserver/api/etcdhttp#Health) struct with JSON encoder.
  - Note that `"health"` field is [`string` type, not `bool`](https://github.com/coreos/etcd/pull/9143).
    - e.g. `{"health":"false"}`, `{"health":"true"}`
  - [Remove `"errors"` field](https://github.com/coreos/etcd/pull/9162) since `v3.3.0-rc.3` (did exist only in `v3.3.0-rc.0`, `v3.3.0-rc.1`, `v3.3.0-rc.2`).
- Move [logging setup to embed package](https://github.com/coreos/etcd/pull/8810)
  - Disable gRPC server info-level logs by default (can be enabled with `etcd --debug` flag).
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
    - In previous versions(v3.2.10, v3.2.11), client response size was limited to only 4 MiB.
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
- Add [`snapshot restore --wal-dir`](https://github.com/coreos/etcd/pull/9124) flag.
- Add [`defrag --data-dir`](https://github.com/coreos/etcd/pull/8367) flag.
- Add [`move-leader`](https://github.com/coreos/etcd/pull/8153) command.
- Add [`endpoint hashkv`](https://github.com/coreos/etcd/pull/8351) command.
- Add [`endpoint --cluster`](https://github.com/coreos/etcd/pull/8143) flag, equivalent to [v2 `etcdctl cluster-health`](https://github.com/coreos/etcd/issues/8117).
- Make `endpoint health` command terminate with [non-zero exit code on unhealthy status](https://github.com/coreos/etcd/pull/8342).
- Add [`lock --ttl`](https://github.com/coreos/etcd/pull/8370) flag.
- Support [`watch [key] [range_end] -- [exec-command…]`](https://github.com/coreos/etcd/pull/8919), equivalent to [v2 `etcdctl exec-watch`](https://github.com/coreos/etcd/issues/8814).
  - Make `watch -- [exec-command]` set environmental variables [`ETCD_WATCH_REVISION`, `ETCD_WATCH_EVENT_TYPE`, `ETCD_WATCH_KEY`, `ETCD_WATCH_VALUE`](https://github.com/coreos/etcd/pull/9142) for each event.
- Support [`watch` with environmental variables `ETCDCTL_WATCH_KEY` and `ETCDCTL_WATCH_RANGE_END`](https://github.com/coreos/etcd/pull/9142).
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

- Add [`grpc-proxy start --experimental-leasing-prefix`](https://github.com/coreos/etcd/pull/8341) flag.
  - For disconnected linearized reads.
  - Based on [V system leasing](https://github.com/coreos/etcd/issues/6065).
  - See ["Disconnected consistent reads with etcd" blog post](https://coreos.com/blog/coreos-labs-disconnected-consistent-reads-with-etcd).
- Add [`grpc-proxy start --experimental-serializable-ordering`](https://github.com/coreos/etcd/pull/8315) flag.
  - To ensure serializable reads have monotonically increasing store revisions across endpoints.
- Add [`grpc-proxy start --metrics-addr`](https://github.com/coreos/etcd/pull/8242) flag for an additional `/metrics` endpoint.
  - Set `--metrics-addr=http://[HOST]:9379` to serve `/metrics` in insecure port 9379.
- Serve [`/health` endpoint in grpc-proxy](https://github.com/coreos/etcd/pull/8322).
- Add [`grpc-proxy start --debug`](https://github.com/coreos/etcd/pull/8994) flag.
- Add [`grpc-proxy start --max-send-bytes`](https://github.com/coreos/etcd/pull/9250) flag to [configure maximum client request size](https://github.com/coreos/etcd/issues/7923).
- Add [`grpc-proxy start --max-recv-bytes`](https://github.com/coreos/etcd/pull/9250) flag to [configure maximum client request size](https://github.com/coreos/etcd/issues/7923).

### Added(gRPC gateway)

- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint with [`/v3beta`](https://github.com/coreos/etcd/pull/8880).
  - To deprecate [`/v3alpha`](https://github.com/coreos/etcd/issues/8125) in `v3.4`.
- Support ["authorization" token](https://github.com/coreos/etcd/pull/7999).
- Support [websocket for bi-directional streams](https://github.com/coreos/etcd/pull/8257).
  - Fix [`Watch` API with gRPC gateway](https://github.com/coreos/etcd/issues/8237).
- Upgrade gRPC gateway to [v1.3.0](https://github.com/coreos/etcd/issues/8838).

### Package `raft`

- Add [non-voting member](https://github.com/coreos/etcd/pull/8751).
  - To implement [Raft thesis 4.2.1 Catching up new servers](https://github.com/coreos/etcd/issues/8568).
  - `Learner` node does not vote or promote itself.

### Added/Fixed(Security/Auth)

- Add [CRL based connection rejection](https://github.com/coreos/etcd/pull/8124) to manage [revoked certs](https://github.com/coreos/etcd/issues/4034).
- Document [TLS authentication changes](https://github.com/coreos/etcd/pull/8895).
  - [Server accepts connections if IP matches, without checking DNS entries](https://github.com/coreos/etcd/pull/8223). For instance, if peer cert contains IP addresses and DNS names in Subject Alternative Name (SAN) field, and the remote IP address matches one of those IP addresses, server just accepts connection without further checking the DNS names.
  - [Server supports reverse-lookup on wildcard DNS `SAN`](https://github.com/coreos/etcd/pull/8281). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server first reverse-lookups the remote IP address to get a list of names mapping to that address (e.g. `nslookup IPADDR`). Then accepts the connection if those names have a matching name with peer cert's DNS names (either by exact or wildcard match). If none is matched, server forward-lookups each DNS entry in peer cert (e.g. look up `example.default.svc` when the entry is `*.example.default.svc`), and accepts connection only when the host's resolved addresses have the matching IP address with the peer's remote IP address.
- Add [`etcd --peer-cert-allowed-cn`](https://github.com/coreos/etcd/pull/8616) flag.
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

- Fix [range/put/delete operation metrics](https://github.com/coreos/etcd/pull/8054) with transaction.
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
- Fix [`mvcc/backend.defragdb` nil-pointer dereference on create bucket failure](https://github.com/coreos/etcd/pull/9119).
- Fix [server crash](https://github.com/coreos/etcd/pull/8010) on [invalid transaction request from gRPC gateway](https://github.com/coreos/etcd/issues/7889).
- Prevent [server panic from member update/add](https://github.com/coreos/etcd/pull/9174) with [wrong scheme URLs](https://github.com/coreos/etcd/issues/9173).
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
- `v3.3.x` is the last release cycle that supports `ACI`.
  - [AppC was officially suspended](https://github.com/appc/spec#-disclaimer-), as of late 2016.
  - [`acbuild`](https://github.com/containers/build#this-project-is-currently-unmaintained) is not maintained anymore.
  - `*.aci` files won't be available from etcd `v3.4` release.
- Add container registry [`gcr.io/etcd-development/etcd`](https://gcr.io/etcd-development/etcd).
  - [quay.io/coreos/etcd](https://quay.io/coreos/etcd) is still supported as secondary.
