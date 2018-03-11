

## v3.4.0 (TBD 2018-05-01)

See [code changes](https://github.com/coreos/etcd/compare/v3.3.0...v3.4.0) and [v3.4 upgrade guide](https://github.com/coreos/etcd/blob/master/Documentation/upgrades/upgrade_3_4.md) for any breaking changes.

### Improved

- Compile with [Go 1.10+](https://groups.google.com/forum/#!topic/golang-announce/IQeqMp44Qok).
- Add [jitter to watch progress notify](https://github.com/coreos/etcd/pull/9278) to prevent [spikes in `etcd_network_client_grpc_sent_bytes_total`](https://github.com/coreos/etcd/issues/9246).
- Add [warnings on requests taking too long](https://github.com/coreos/etcd/pull/9288).
  - e.g. `etcdserver: read-only range request "key:\"\\000\" range_end:\"\\000\" " took too long [3.389041388s] to execute`
- Improve [long-running concurrent read transactions under light write workloads](https://github.com/coreos/etcd/pull/9296).
  - Previously, periodic commit on pending writes blocks incoming read transactions, even if there is no pending write.
  - Now, periodic commit operation does not block concurrent read transactions, thus improves long-running read transaction performance.
- Adjust [election timeout on server restart](https://github.com/coreos/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/coreos/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.
- Add [Raft Pre-Vote feature](https://github.com/coreos/etcd/pull/9352) to reduce [disruptive rejoining servers](https://github.com/coreos/etcd/issues/9333).
  - For instance, a flaky(or rejoining) member may drop in and out, and start campaign. This member will end up with a higher term, and ignore all incoming messages with lower term. In this case, a new leader eventually need to get elected, thus disruptive to cluster availability. Raft implements Pre-Vote phase to prevent this kind of disruptions. If enabled, Raft runs an additional phase of election to check if pre-candidate can get enough votes to win an election.
- Make [Lease `Lookup` non-blocking with concurrent `Grant`/`Revoke`](https://github.com/coreos/etcd/pull/9229).

### Breaking Changes

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
- Move `"github.com/coreos/etcd/snap"` to [`"github.com/coreos/etcd/raftsnap"`](https://github.com/coreos/etcd/pull/9211).
- Move `"github.com/coreos/etcd/etcdserver/auth"` to [`"github.com/coreos/etcd/etcdserver/v2auth"`](https://github.com/coreos/etcd/pull/9275).
- Move `"github.com/coreos/etcd/error"` to [`"github.com/coreos/etcd/etcdserver/v2error"`](https://github.com/coreos/etcd/pull/9274).
- Move `"github.com/coreos/etcd/store"` to [`"github.com/coreos/etcd/etcdserver/v2store"`](https://github.com/coreos/etcd/pull/9274).
- Change v3 `etcdctl snapshot` exit codes with [`snapshot` package](https://github.com/coreos/etcd/pull/9118/commits/df689f4280e1cce4b9d61300be13ca604d41670a).
  - Exit on error with exit code 1 (no more exit code 5 or 6 on `snapshot save/restore` commands).
- Require Go 1.10+.
- Migrate dependency management tool from `glide` to [`golang/dep`](https://github.com/coreos/etcd/pull/9155).
  - <= 3.3 puts `vendor` directory under `cmd/vendor` directory to [prevent conflicting transitive dependencies](https://github.com/coreos/etcd/issues/4913).
  - 3.4 moves `cmd/vendor` directory to `vendor` at repository root.
  - Remove recursive symlinks in `cmd` directory.
  - Now `go get/install/build` on `etcd` packages (e.g. `clientv3`, `tools/benchmark`) enforce builds with etcd `vendor` directory.

### Added: `etcd`

- Add [`--discovery-srv-name`](https://github.com/coreos/etcd/pull/8690) flag to support custom DNS SRV name with discovery.
  - If not given, etcd queries `_etcd-server-ssl._tcp.[YOUR_HOST]` and `_etcd-server._tcp.[YOUR_HOST]`.
  - If `--discovery-srv-name="foo"`, then query `_etcd-server-ssl-foo._tcp.[YOUR_HOST]` and `_etcd-server-foo._tcp.[YOUR_HOST]`.
  - Useful for operating multiple etcd clusters under the same domain.
- Define [`embed.CompactorModePeriodic`](https://godoc.org/github.com/coreos/etcd/embed#pkg-variables) for `compactor.ModePeriodic`.
- Define [`embed.CompactorModeRevision`](https://godoc.org/github.com/coreos/etcd/embed#pkg-variables) for `compactor.ModeRevision`.

### Security, Authentication

- Add [`--host-whitelist`](https://github.com/coreos/etcd/pull/9372) flag, [`etcdserver.Config.HostWhitelist`](https://github.com/coreos/etcd/pull/9372), and [`embed.Config.HostWhitelist`](https://github.com/coreos/etcd/pull/9372), to prevent ["DNS Rebinding"](https://en.wikipedia.org/wiki/DNS_rebinding) attack.
  - Any website can simply create an authorized DNS name, and direct DNS to `"localhost"` (or any other address). Then, all HTTP endpoints of etcd server listening on `"localhost"` becomes accessible, thus vulnerable to [DNS rebinding attacks (CVE-2018-5702)](https://bugs.chromium.org/p/project-zero/issues/detail?id=1447#c2).
  - Client origin enforce policy works as follow:
    - If client connection is secure via HTTPS, allow any hostnames..
    - If client connection is not secure and `"HostWhitelist"` is not empty, only allow HTTP requests whose Host field is listed in whitelist.
  - By default, `"HostWhitelist"` is empty, which means insecure server allows all client HTTP requests.
  - Note that the client origin policy is enforced whether authentication is enabled or not, for tighter controls.
  - When specifying hostnames, loopback addresses are not added automatically. To allow loopback interfaces, add them to whitelist manually (e.g. `"localhost"`, `"127.0.0.1"`, etc.).
  - e.g. `etcd --host-whitelist example.com`, then the server will reject all HTTP requests whose Host field is not `example.com` (also rejects requests to `"localhost"`).
- Support [`ttl` field for `etcd` Authentication JWT token](https://github.com/coreos/etcd/pull/8302).
  - e.g. `etcd --auth-token jwt,pub-key=<pub key path>,priv-key=<priv key path>,sign-method=<sign method>,ttl=5m`.
- Allow empty token provider in [`etcdserver.ServerConfig.AuthToken`](https://github.com/coreos/etcd/pull/9369).

### Added: API

- Add [`snapshot`](https://github.com/coreos/etcd/pull/9118) package for snapshot restore/save operations.
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

### Added: metrics

- Add [`etcd_debugging_mvcc_db_total_size_in_use_in_bytes`](https://github.com/coreos/etcd/pull/9256) Prometheus metric.

### Added: gRPC gateway

- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint with [`/v3`](https://github.com/coreos/etcd/pull/9298).
  - To deprecate [`/v3beta`](https://github.com/coreos/etcd/issues/9189) in `v3.5`.

### Package `raft`

- Fix [deadlock during PreVote migration process](https://github.com/coreos/etcd/pull/8525).
- Add [`raft.ErrProposalDropped`](https://github.com/coreos/etcd/pull/9067).
  - Now `(r *raft) Step` returns `raft.ErrProposalDropped` if a proposal has been ignored.
  - e.g. a node is removed from cluster, [ongoing leadership transfer](https://github.com/coreos/etcd/issues/8975), etc.
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
