

## v3.4.0 (TBD 2018-06-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.3.0...v3.4.0) and [v3.4 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_4.md) for any breaking changes.

### Improved

- Rewrite [client balancer](TODO) with [new gRPC balancer interface](TODO).
- Add [jitter to watch progress notify](https://github.com/coreos/etcd/pull/9278) to prevent [spikes in `etcd_network_client_grpc_sent_bytes_total`](https://github.com/coreos/etcd/issues/9246).
- Improve [slow requests warning logging](https://github.com/coreos/etcd/pull/9288).
  - e.g. `etcdserver: read-only range request "key:\"\\000\" range_end:\"\\000\" " took too long [3.389041388s] to execute`
- Improve [TLS setup error logging](https://github.com/coreos/etcd/pull/9518) to help debug [TLS-enabled cluster configuring issues](https://github.com/coreos/etcd/issues/9400).
- Improve [long-running concurrent read transactions under light write workloads](https://github.com/coreos/etcd/pull/9296).
  - Previously, periodic commit on pending writes blocks incoming read transactions, even if there is no pending write.
  - Now, periodic commit operation does not block concurrent read transactions, thus improves long-running read transaction performance.
- Adjust [election timeout on server restart](https://github.com/coreos/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/coreos/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.
- Add [Raft Pre-Vote feature](https://github.com/coreos/etcd/pull/9352) to reduce [disruptive rejoining servers](https://github.com/coreos/etcd/issues/9333).
  - For instance, a flaky(or rejoining) member may drop in and out, and start campaign. This member will end up with a higher term, and ignore all incoming messages with lower term. In this case, a new leader eventually need to get elected, thus disruptive to cluster availability. Raft implements Pre-Vote phase to prevent this kind of disruptions. If enabled, Raft runs an additional phase of election to check if pre-candidate can get enough votes to win an election.
- Adjust [periodic compaction retention window](https://github.com/coreos/etcd/pull/9485).
  - e.g. `--auto-compaction-mode=revision --auto-compaction-retention=1000` automatically `Compact` on `"latest revision" - 1000` every 5-minute (when latest revision is 30000, compact on revision 29000).
  - e.g. Previously, `--auto-compaction-mode=periodic --auto-compaction-retention=24h` automatically `Compact` with 24-hour retention windown for every 2.4-hour. Now, `Compact` happens for every 1-hour.
  - e.g. Previously, `--auto-compaction-mode=periodic --auto-compaction-retention=30m` automatically `Compact` with 30-minute retention windown for every 3-minute. Now, `Compact` happens for every 30-minute.
  - Periodic compactor keeps recording latest revisions for every compaction period when given period is less than 1-hour, or for every 1-hour when given compaction period is greater than 1-hour (e.g. 1-hour when `--auto-compaction-mode=periodic --auto-compaction-retention=24h`).
  - For every compaction period or 1-hour, compactor uses the last revision that was fetched before compaction period, to discard historical data.
  - The retention window of compaction period moves for every given compaction period or hour.
  - For instance, when hourly writes are 100 and `--auto-compaction-mode=periodic --auto-compaction-retention=24h`, `v3.2.x`, `v3.3.0`, `v3.3.1`, and `v3.3.2` compact revision 2400, 2640, and 2880 for every 2.4-hour, while `v3.3.3` *or later* compacts revision 2400, 2500, 2600 for every 1-hour.
  - Futhermore, when `--auto-compaction-mode=periodic --auto-compaction-retention=30m` and writes per minute are about 1000, `v3.3.0`, `v3.3.1`, and `v3.3.2` compact revision 30000, 33000, and 36000, for every 3-minute, while `v3.3.3` *or later* compacts revision 30000, 60000, and 90000, for every 30-minute.
- Improve [lease expire/revoke operation performance](https://github.com/coreos/etcd/pull/9418), address [lease scalability issue](https://github.com/coreos/etcd/issues/9496).
- Make [Lease `Lookup` non-blocking with concurrent `Grant`/`Revoke`](https://github.com/coreos/etcd/pull/9229).
- Make etcd server return `raft.ErrProposalDropped` on internal Raft proposal drop in [v3 applier](https://github.com/coreos/etcd/pull/9549) and [v2 applier](https://github.com/coreos/etcd/pull/9558).
  - e.g. a node is removed from cluster, or [`raftpb.MsgProp` arrives at current leader while there is an ongoing leadership transfer](https://github.com/coreos/etcd/issues/8975).
- Add [`snapshot`](https://github.com/coreos/etcd/pull/9118) package for easier snapshot workflow (see [`godoc.org/github.com/etcd/snapshot`](https://godoc.org/github.com/coreos/etcd/snapshot) for more).
- Improve [functional tester](https://github.com/coreos/etcd/tree/master/functional) coverage: [proxy layer to run network fault tests in CI](https://github.com/coreos/etcd/pull/9081), [TLS is enabled both for server and client](https://github.com/coreos/etcd/pull/9534), [liveness mode](https://github.com/coreos/etcd/issues/9230), [shuffle test sequence](https://github.com/coreos/etcd/issues/9381), [membership reconfiguration failure cases](https://github.com/coreos/etcd/pull/9564), [disastrous quorum loss and snapshot recover from a seed member](https://github.com/coreos/etcd/pull/9565), [embedded etcd](https://github.com/coreos/etcd/pull/9572).
- Improve [index compaction blocking](https://github.com/coreos/etcd/pull/9511) by using a copy on write clone to avoid holding the lock for the traversal of the entire index.

### Breaking Changes

- **Remove `etcd --ca-file` flag**, instead [use `--trusted-ca-file`](https://github.com/coreos/etcd/pull/9470) (`--ca-file` has been deprecated since v2.1).
- **Remove `etcd --peer-ca-file` flag**, instead [use `--peer-trusted-ca-file`](https://github.com/coreos/etcd/pull/9470) (`--peer-ca-file` has been deprecated since v2.1).
- **Remove `pkg/transport.TLSInfo.CAFile` field**, instead [use `pkg/transport.TLSInfo.TrustedCAFile`](https://github.com/coreos/etcd/pull/9470) (`CAFile` has been deprecated since v2.1).
- Drop [ACIs from official release](https://github.com/coreos/etcd/pull/9059).
  - [AppC was officially suspended](https://github.com/appc/spec#-disclaimer-), as of late 2016.
  - [`acbuild`](https://github.com/containers/build#this-project-is-currently-unmaintained) is not maintained anymore.
  - `*.aci` files are not available from `v3.4` release.
- Exit on [empty hosts in advertise URLs](https://github.com/coreos/etcd/pull/8786).
  - Address [advertise client URLs accepts empty hosts](https://github.com/coreos/etcd/issues/8379).
  - e.g. exit with error on `--advertise-client-urls=http://:2379`.
  - e.g. exit with error on `--initial-advertise-peer-urls=http://:2380`.
- Exit on [shadowed environment variables](https://github.com/coreos/etcd/pull/9382).
  - Address [error on shadowed environment variables](https://github.com/coreos/etcd/issues/8380).
  - e.g. exit with error on `ETCD_NAME=abc etcd --name=def`.
  - e.g. exit with error on `ETCD_INITIAL_CLUSTER_TOKEN=abc etcd --initial-cluster-token=def`.
  - e.g. exit with error on `ETCDCTL_ENDPOINTS=abc.com ETCDCTL_API=3 etcdctl endpoint health --endpoints=def.com`.
- Change [`etcdserverpb.AuthRoleRevokePermissionRequest/key,range_end` fields type from `string` to `bytes`](https://github.com/coreos/etcd/pull/9433).
- Change [`embed.Config.CorsInfo` in `*cors.CORSInfo` type to `embed.Config.CORS` in `map[string]struct{}` type](https://github.com/coreos/etcd/pull/9490).
- Change v3 `etcdctl snapshot` exit codes with [`snapshot` package](https://github.com/coreos/etcd/pull/9118/commits/df689f4280e1cce4b9d61300be13ca604d41670a).
  - Exit on error with exit code 1 (no more exit code 5 or 6 on `snapshot save/restore` commands).
- Migrate dependency management tool from `glide` to [`golang/dep`](https://github.com/coreos/etcd/pull/9155).
  - <= 3.3 puts `vendor` directory under `cmd/vendor` directory to [prevent conflicting transitive dependencies](https://github.com/coreos/etcd/issues/4913).
  - 3.4 moves `cmd/vendor` directory to `vendor` at repository root.
  - Remove recursive symlinks in `cmd` directory.
  - Now `go get/install/build` on `etcd` packages (e.g. `clientv3`, `tools/benchmark`) enforce builds with etcd `vendor` directory.
- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint `/v3beta` with [`/v3`](https://github.com/coreos/etcd/pull/9298).
  - Deprecated [`/v3alpha`](https://github.com/coreos/etcd/pull/9298).
- Change [`wal` package function signatures](https://github.com/coreos/etcd/pull/9572) to support [structured logger and logging to file](https://github.com/coreos/etcd/issues/9438) in server-side.
  - Previously, `Open(dirpath string, snap walpb.Snapshot) (*WAL, error)`, now `Open(lg *zap.Logger, dirpath string, snap walpb.Snapshot) (*WAL, error)`.
  - Previously, `OpenForRead(dirpath string, snap walpb.Snapshot) (*WAL, error)`, now `OpenForRead(lg *zap.Logger, dirpath string, snap walpb.Snapshot) (*WAL, error)`.
  - Previously, `Repair(dirpath string) bool`, now `Repair(lg *zap.Logger, dirpath string) bool`.
  - Previously, `Create(dirpath string, metadata []byte) (*WAL, error)`, now `Create(lg *zap.Logger, dirpath string, metadata []byte) (*WAL, error)`.
- Remove [`embed.Config.SetupLogging`](https://github.com/coreos/etcd/pull/9572).
  - Now logger is set up automatically based on [`embed.Config.Logger`, `embed.Config.LogOutput`, `embed.Config.Debug` fields](https://github.com/coreos/etcd/pull/9572).
- Change [`embed.Config.LogOutput` type from `string` to `[]string`](https://github.com/coreos/etcd/pull/9579) to support multiple log outputs.
  - Now that `log-output` accepts multiple writers, etcd configuration YAML file `log-output` field must be changed to `[]string` type.
  - Previously, `etcd.config.yaml` can have `log-output: default` field, now must be `log-output: [default]`.
- Remove [`pkg/cors` package](https://github.com/coreos/etcd/pull/9490).
- Move `"github.com/coreos/etcd/snap"` to [`"github.com/coreos/etcd/raftsnap"`](https://github.com/coreos/etcd/pull/9211).
- Move `"github.com/coreos/etcd/etcdserver/auth"` to [`"github.com/coreos/etcd/etcdserver/v2auth"`](https://github.com/coreos/etcd/pull/9275).
- Move `"github.com/coreos/etcd/error"` to [`"github.com/coreos/etcd/etcdserver/v2error"`](https://github.com/coreos/etcd/pull/9274).
- Move `"github.com/coreos/etcd/store"` to [`"github.com/coreos/etcd/etcdserver/v2store"`](https://github.com/coreos/etcd/pull/9274).

### Dependency

- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) from [**`v1.7.5`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5) to [**`v1.11.1`**](TODO).
- Upgrade [`github.com/ugorji/go/codec`](https://github.com/ugorji/go) to [**`v1.1.1`**](https://github.com/ugorji/go/releases/tag/v1.1.1), and [regenerate v2 `client`](https://github.com/coreos/etcd/pull/9494).
- Upgrade [`github.com/soheilhy/cmux`](https://github.com/soheilhy/cmux/releases) from [**`v0.1.3`**](https://github.com/soheilhy/cmux/releases/tag/v0.1.3) to [**`v0.1.4`**](https://github.com/soheilhy/cmux/releases/tag/v0.1.4).
- Upgrade [`github.com/google/btree`](https://github.com/google/btree/releases) from [**`google/btree@925471ac9`**](https://github.com/google/btree/commit/925471ac9e2131377a91e1595defec898166fe49) to [**`google/btree@e89373fe6`**](https://github.com/google/btree/commit/e89373fe6b4a7413d7acd6da1725b83ef713e6e4).
- Upgrade [`github.com/spf13/cobra`](https://github.com/spf13/cobra/releases) from [**`spf13/cobra@1c44ec8d3`**](https://github.com/spf13/cobra/commit/1c44ec8d3f1552cac48999f9306da23c4d8a288b) to [**`spf13/cobra@cd30c2a7e`**](https://github.com/spf13/cobra/commit/cd30c2a7e91a1d63fd9a0027accf18a681e9d50b).
- Upgrade [`github.com/spf13/pflag`](https://github.com/spf13/pflag/releases) from [**`v1.0.0`**](https://github.com/spf13/pflag/releases/tag/v1.0.0) to [**`spf13/pflag@1ce0cc6db`**](https://github.com/spf13/pflag/commit/1ce0cc6db4029d97571db82f85092fccedb572ce).

### Metrics, Monitoring

- Add [`etcd_debugging_mvcc_db_total_size_in_use_in_bytes`](https://github.com/coreos/etcd/pull/9256) Prometheus metric.
- Add missing [`etcd_network_peer_sent_failures_total` count](https://github.com/coreos/etcd/pull/9437).
- Fix [`etcd_debugging_server_lease_expired_total`](https://github.com/coreos/etcd/pull/9557) Prometheus metric.
- Fix [race conditions in v2 server stat collecting](https://github.com/coreos/etcd/pull/9562).

### Security, Authentication

See [security doc](https://github.com/coreos/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- Add [`etcd --host-whitelist`](https://github.com/coreos/etcd/pull/9372) flag, [`etcdserver.Config.HostWhitelist`](https://github.com/coreos/etcd/pull/9372), and [`embed.Config.HostWhitelist`](https://github.com/coreos/etcd/pull/9372), to prevent ["DNS Rebinding"](https://en.wikipedia.org/wiki/DNS_rebinding) attack.
  - Any website can simply create an authorized DNS name, and direct DNS to `"localhost"` (or any other address). Then, all HTTP endpoints of etcd server listening on `"localhost"` becomes accessible, thus vulnerable to [DNS rebinding attacks (CVE-2018-5702)](https://bugs.chromium.org/p/project-zero/issues/detail?id=1447#c2).
  - Client origin enforce policy works as follow:
    - If client connection is secure via HTTPS, allow any hostnames..
    - If client connection is not secure and `"HostWhitelist"` is not empty, only allow HTTP requests whose Host field is listed in whitelist.
  - By default, `"HostWhitelist"` is `"*"`, which means insecure server allows all client HTTP requests.
  - Note that the client origin policy is enforced whether authentication is enabled or not, for tighter controls.
  - When specifying hostnames, loopback addresses are not added automatically. To allow loopback interfaces, add them to whitelist manually (e.g. `"localhost"`, `"127.0.0.1"`, etc.).
  - e.g. `etcd --host-whitelist example.com`, then the server will reject all HTTP requests whose Host field is not `example.com` (also rejects requests to `"localhost"`).
- Support [`etcd --cors`](https://github.com/coreos/etcd/pull/9490) in v3 HTTP requests (gRPC gateway).
- Support [TLS cipher suite lists](TODO).
- Support [`ttl` field for `etcd` Authentication JWT token](https://github.com/coreos/etcd/pull/8302).
  - e.g. `etcd --auth-token jwt,pub-key=<pub key path>,priv-key=<priv key path>,sign-method=<sign method>,ttl=5m`.
- Allow empty token provider in [`etcdserver.ServerConfig.AuthToken`](https://github.com/coreos/etcd/pull/9369).
- Fix [TLS reload](https://github.com/coreos/etcd/pull/9570) when [certificate SAN field only includes IP addresses but no domain names](https://github.com/coreos/etcd/issues/9541).
  - In Go, server calls `(*tls.Config).GetCertificate` for TLS reload if and only if server's `(*tls.Config).Certificates` field is not empty, or `(*tls.ClientHelloInfo).ServerName` is not empty with a valid SNI from the client. Previously, etcd always populates `(*tls.Config).Certificates` on the initial client TLS handshake, as non-empty. Thus, client was always expected to supply a matching SNI in order to pass the TLS verification and to trigger `(*tls.Config).GetCertificate` to reload TLS assets.
  - However, a certificate whose SAN field does [not include any domain names but only IP addresses](https://github.com/coreos/etcd/issues/9541) would request `*tls.ClientHelloInfo` with an empty `ServerName` field, thus failing to trigger the TLS reload on initial TLS handshake; this becomes a problem when expired certificates need to be replaced online.
  - Now, `(*tls.Config).Certificates` is created empty on initial TLS client handshake, first to trigger `(*tls.Config).GetCertificate`, and then to populate rest of the certificates on every new TLS connection, even when client SNI is empty (e.g. cert only includes IPs).

### Added: `etcd`

- Add [`--pre-vote`](https://github.com/coreos/etcd/pull/9352) flag to enable to run an additional Raft election phase.
  - For instance, a flaky(or rejoining) member may drop in and out, and start campaign. This member will end up with a higher term, and ignore all incoming messages with lower term. In this case, a new leader eventually need to get elected, thus disruptive to cluster availability. Raft implements Pre-Vote phase to prevent this kind of disruptions. If enabled, Raft runs an additional phase of election to check if pre-candidate can get enough votes to win an election.
  - `--pre-vote=false` by default.
  - v3.5 will enable `--pre-vote=true` by default.
- [`--initial-corrupt-check`](TODO) flag is now stable (`--experimental-initial-corrupt-check` is deprecated).
  - `--initial-corrupt-check=true` by default, to check cluster database hashes before serving client/peer traffic.
- [`--corrupt-check-time`](TODO) flag is now stable (`--experimental-corrupt-check-time` is deprecated).
  - `--corrupt-check-time=12h` by default, to check cluster database hashes for every 12-hour.
- [`--enable-v2v3`](TODO) flag is now stable (`--experimental-enable-v2v3` is deprecated).
  - `--enable-v2=true --enable-v2v3=''` by default, to enable v2 API server that is backed by **v2 store**.
  - `--enable-v2=true --enable-v2v3=/aaa` to enable v2 API server that is backed by **v3 storage**.
  - `--enable-v2=false --enable-v2v3=''` to disable v2 API server.
  - `--enable-v2=false --enable-v2v3=/aaa` to disable v2 API server. TODO: error?
  - v4.0 will configure `--enable-v2=true --enable-v2v3=/aaa` to enable v2 API server that is backed by **v3 storage**.
- Add [`--discovery-srv-name`](https://github.com/coreos/etcd/pull/8690) flag to support custom DNS SRV name with discovery.
  - If not given, etcd queries `_etcd-server-ssl._tcp.[YOUR_HOST]` and `_etcd-server._tcp.[YOUR_HOST]`.
  - If `--discovery-srv-name="foo"`, then query `_etcd-server-ssl-foo._tcp.[YOUR_HOST]` and `_etcd-server-foo._tcp.[YOUR_HOST]`.
  - Useful for operating multiple etcd clusters under the same domain.
- Support [`etcd --cors`](https://github.com/coreos/etcd/pull/9490) in v3 HTTP requests (gRPC gateway).
- Add [`--logger`](https://github.com/coreos/etcd/pull/9572) flag to support [structured logger and logging to file](https://github.com/coreos/etcd/issues/9438) in server-side.
  - e.g. `--logger=capnslog --log-output=default` is the default setting and same as previous etcd server logging format.
  - TODO: `--logger=zap` is experimental, and journald logging may not work when etcd runs as PID 1.
  - e.g. `--logger=zap --log-output=/tmp/test.log` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to the specified file `/tmp/test.log`.
  - e.g. `--logger=zap --log-output=default` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to `os.Stderr` (detect systemd journald TODO).
  - e.g. `--logger=zap --log-output=stderr` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to `os.Stderr` (bypass journald TODO).
  - e.g. `--logger=zap --log-output=stdout` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to `os.Stdout` (bypass journald TODO).
  - e.g. `--logger=zap --log-output=a.log,b.log,c.log,stdout` [writes server logs to multiple files `a.log`, `b.log` and `c.log` at the same time](https://github.com/coreos/etcd/pull/9579) and outputs to `stdout`, in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig).
  - e.g. `--logger=zap --log-output=/dev/null` will discard all server logs.

### Added: `embed`

- Add [`embed.Config.Logger`](https://github.com/coreos/etcd/pull/9518) to support [structured logger `zap`](https://github.com/uber-go/zap) in server-side.
- Define [`embed.CompactorModePeriodic`](https://godoc.org/github.com/coreos/etcd/embed#pkg-variables) for `compactor.ModePeriodic`.
- Define [`embed.CompactorModeRevision`](https://godoc.org/github.com/coreos/etcd/embed#pkg-variables) for `compactor.ModeRevision`.

### Added: API

- Add [`snapshot`](https://github.com/coreos/etcd/pull/9118) package for snapshot restore/save operations (see [`godoc.org/github.com/etcd/snapshot`](https://godoc.org/github.com/coreos/etcd/snapshot) for more).
- Add [`watch_id` field to `etcdserverpb.WatchCreateRequest`](https://github.com/coreos/etcd/pull/9065), allow user-provided watch ID to `mvcc`.
  - Corresponding `watch_id` is returned via `etcdserverpb.WatchResponse`, if any.
- Add [`raftAppliedIndex` field to `etcdserverpb.StatusResponse`](https://github.com/coreos/etcd/pull/9176) for current Raft applied index.
- Add [`errors` field to `etcdserverpb.StatusResponse`](https://github.com/coreos/etcd/pull/9206) for server-side error.
  - e.g. `"etcdserver: no leader", "NOSPACE", "CORRUPT"`
- Add [`dbSizeInUse` field to `etcdserverpb.StatusResponse`](https://github.com/coreos/etcd/pull/9256) for actual DB size after compaction.

### Added: v3 `etcdctl`

- Add [`check datascale`](https://github.com/coreos/etcd/pull/9185) command.
- Add [`check datascale --auto-compact, --auto-defrag`](https://github.com/coreos/etcd/pull/9351) flags.
- Add [`check perf --auto-compact, --auto-defrag`](https://github.com/coreos/etcd/pull/9330) flags.
- Add [`defrag --cluster`](https://github.com/coreos/etcd/pull/9390) flag.
- Add ["raft applied index" field to `endpoint status`](https://github.com/coreos/etcd/pull/9176).
- Add ["errors" field to `endpoint status`](https://github.com/coreos/etcd/pull/9206).
- Add [`endpoint health --write-out` support](https://github.com/coreos/etcd/pull/9540).
  - Previously, [`endpoint health --write-out json` did not work](https://github.com/coreos/etcd/issues/9532).

### Added: gRPC gateway

- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint `/v3beta` with [`/v3`](https://github.com/coreos/etcd/pull/9298).
  - Deprecated [`/v3alpha`](https://github.com/coreos/etcd/pull/9298).
  - To deprecate [`/v3beta`](https://github.com/coreos/etcd/issues/9189) in `v3.5`.
- Add API endpoints [`/{v3beta,v3}/lease/leases, /{v3beta,v3}/lease/revoke, /{v3beta,v3}/lease/timetolive`](https://github.com/coreos/etcd/pull/9450).
  - To deprecate [`/{v3beta,v3}/kv/lease/leases, /{v3beta,v3}/kv/lease/revoke, /{v3beta,v3}/kv/lease/timetolive`](https://github.com/coreos/etcd/issues/9430) in `v3.5`.
- Support [`etcd --cors`](https://github.com/coreos/etcd/pull/9490) in v3 HTTP requests (gRPC gateway).

### Package `raft`

- Fix [deadlock during PreVote migration process](https://github.com/coreos/etcd/pull/8525).
- Add [`raft.ErrProposalDropped`](https://github.com/coreos/etcd/pull/9067).
  - Now [`(r *raft) Step` returns `raft.ErrProposalDropped`](https://github.com/coreos/etcd/pull/9137) if a proposal has been ignored.
  - e.g. a node is removed from cluster, or [`raftpb.MsgProp` arrives at current leader while there is an ongoing leadership transfer](https://github.com/coreos/etcd/issues/8975).
- Improve [Raft `becomeLeader` and `stepLeader`](https://github.com/coreos/etcd/pull/9073) by keeping track of latest `pb.EntryConfChange` index.
  - Previously record `pendingConf` boolean field scanning the entire tail of the log, which can delay hearbeat send.
- Fix [missing learner nodes on `(n *node) ApplyConfChange`](https://github.com/coreos/etcd/pull/9116).

### Fixed: v3

- Fix [`mvcc` "unsynced" watcher restore operation](https://github.com/coreos/etcd/pull/9281).
  - "unsynced" watcher is watcher that needs to be in sync with events that have happened.
  - That is, "unsynced" watcher is the slow watcher that was requested on old revision.
  - "unsynced" watcher restore operation was not correctly populating its underlying watcher group.
  - Which possibly causes [missing events from "unsynced" watchers](https://github.com/coreos/etcd/issues/9086).
- Fix [server panic on invalid Election Proclaim/Resign HTTP(S) requests](https://github.com/coreos/etcd/pull/9379).
  - Previously, wrong-formatted HTTP requests to Election API could trigger panic in etcd server.
  - e.g. `curl -L http://localhost:2379/v3/election/proclaim -X POST -d '{"value":""}'`, `curl -L http://localhost:2379/v3/election/resign -X POST -d '{"value":""}'`.
- Fix [revision-based compaction retention parsing](https://github.com/coreos/etcd/pull/9339).
  - Previously, `etcd --auto-compaction-mode revision --auto-compaction-retention 1` was [translated to revision retention 3600000000000](https://github.com/coreos/etcd/issues/9337).
  - Now, `etcd --auto-compaction-mode revision --auto-compaction-retention 1` is correctly parsed as revision retention 1.
- Prevent [overflow by large `TTL` values for `Lease` `Grant`](https://github.com/coreos/etcd/pull/9399).
  - `TTL` parameter to `Grant` request is unit of second.
  - Leases with too large `TTL` values exceeding `math.MaxInt64` [expire in unexpected ways](https://github.com/coreos/etcd/issues/9374).
  - Server now returns `rpctypes.ErrLeaseTTLTooLarge` to client, when the requested `TTL` is larger than *9,000,000,000 seconds* (which is >285 years).
  - Again, etcd `Lease` is meant for short-periodic keepalives or sessions, in the range of seconds or minutes. Not for hours or days!
- Enable etcd server [`raft.Config.CheckQuorum` when starting with `ForceNewCluster`](https://github.com/coreos/etcd/pull/9347).

### Go

- Require *Go 1.10+*.
- Compile with [*Go 1.10*](https://golang.org/doc/devel/release.html#go1.10).

