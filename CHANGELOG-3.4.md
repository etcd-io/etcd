

Previous change logs can be found at [CHANGELOG-3.3](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.3.md).


<hr>


## v3.4.0 (TBD 2019)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.0...v3.4.0) and [v3.4 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_4.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.4 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_4.md).**

### Improved

- Add Raft learner: [#10725](https://github.com/etcd-io/etcd/pull/10725), [#10727](https://github.com/etcd-io/etcd/pull/10727), [#10730](https://github.com/etcd-io/etcd/pull/10730).
  - User guide: [runtime-configuration document](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/runtime-configuration.md#add-a-new-member-as-learner).
  - API change: [API reference document](https://github.com/etcd-io/etcd/blob/master/Documentation/dev-guide/api_reference_v3.md).
  - More details on implementation: [learner design document](https://github.com/etcd-io/etcd/blob/master/docs/server-learner.rst) and [implementation task list](https://github.com/etcd-io/etcd/issues/10537).
- Rewrite [client balancer](https://github.com/etcd-io/etcd/pull/9860) with [new gRPC balancer interface](https://github.com/etcd-io/etcd/issues/9106).
- Add [backoff on watch retries on transient errors](https://github.com/etcd-io/etcd/pull/9840).
- Add [jitter to watch progress notify](https://github.com/etcd-io/etcd/pull/9278) to prevent [spikes in `etcd_network_client_grpc_sent_bytes_total`](https://github.com/etcd-io/etcd/issues/9246).
- Improve [read index wait timeout warning log](https://github.com/etcd-io/etcd/pull/10026), which indicates that local node might have slow network.
- Improve [slow request apply warning log](https://github.com/etcd-io/etcd/pull/9288).
  - e.g. `read-only range request "key:\"/a\" range_end:\"/b\" " with result "range_response_count:3 size:96" took too long (97.966µs) to execute`.
  - Redact [request value field](https://github.com/etcd-io/etcd/pull/9822).
  - Provide [response size](https://github.com/etcd-io/etcd/pull/9826).
- Improve ["became inactive" warning log](https://github.com/etcd-io/etcd/pull/10024), which indicates message send to a peer failed.
- Improve [TLS setup error logging](https://github.com/etcd-io/etcd/pull/9518) to help debug [TLS-enabled cluster configuring issues](https://github.com/etcd-io/etcd/issues/9400).
- Improve [long-running concurrent read transactions under light write workloads](https://github.com/etcd-io/etcd/pull/9296).
  - Previously, periodic commit on pending writes blocks incoming read transactions, even if there is no pending write.
  - Now, periodic commit operation does not block concurrent read transactions, thus improves long-running read transaction performance.
- Improve [Raft Read Index timeout warning messages](https://github.com/etcd-io/etcd/pull/9897).
- Adjust [election timeout on server restart](https://github.com/etcd-io/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/etcd-io/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.
- Add [Raft Pre-Vote feature](https://github.com/etcd-io/etcd/pull/9352) to reduce [disruptive rejoining servers](https://github.com/etcd-io/etcd/issues/9333).
  - For instance, a flaky(or rejoining) member may drop in and out, and start campaign. This member will end up with a higher term, and ignore all incoming messages with lower term. In this case, a new leader eventually need to get elected, thus disruptive to cluster availability. Raft implements Pre-Vote phase to prevent this kind of disruptions. If enabled, Raft runs an additional phase of election to check if pre-candidate can get enough votes to win an election.
- Adjust [periodic compaction retention window](https://github.com/etcd-io/etcd/pull/9485).
  - e.g. `etcd --auto-compaction-mode=revision --auto-compaction-retention=1000` automatically `Compact` on `"latest revision" - 1000` every 5-minute (when latest revision is 30000, compact on revision 29000).
  - e.g. Previously, `etcd --auto-compaction-mode=periodic --auto-compaction-retention=24h` automatically `Compact` with 24-hour retention windown for every 2.4-hour. Now, `Compact` happens for every 1-hour.
  - e.g. Previously, `etcd --auto-compaction-mode=periodic --auto-compaction-retention=30m` automatically `Compact` with 30-minute retention windown for every 3-minute. Now, `Compact` happens for every 30-minute.
  - Periodic compactor keeps recording latest revisions for every compaction period when given period is less than 1-hour, or for every 1-hour when given compaction period is greater than 1-hour (e.g. 1-hour when `etcd --auto-compaction-mode=periodic --auto-compaction-retention=24h`).
  - For every compaction period or 1-hour, compactor uses the last revision that was fetched before compaction period, to discard historical data.
  - The retention window of compaction period moves for every given compaction period or hour.
  - For instance, when hourly writes are 100 and `etcd --auto-compaction-mode=periodic --auto-compaction-retention=24h`, `v3.2.x`, `v3.3.0`, `v3.3.1`, and `v3.3.2` compact revision 2400, 2640, and 2880 for every 2.4-hour, while `v3.3.3` *or later* compacts revision 2400, 2500, 2600 for every 1-hour.
  - Futhermore, when `etcd --auto-compaction-mode=periodic --auto-compaction-retention=30m` and writes per minute are about 1000, `v3.3.0`, `v3.3.1`, and `v3.3.2` compact revision 30000, 33000, and 36000, for every 3-minute, while `v3.3.3` *or later* compacts revision 30000, 60000, and 90000, for every 30-minute.
- Improve [lease expire/revoke operation performance](https://github.com/etcd-io/etcd/pull/9418), address [lease scalability issue](https://github.com/etcd-io/etcd/issues/9496).
- Make [Lease `Lookup` non-blocking with concurrent `Grant`/`Revoke`](https://github.com/etcd-io/etcd/pull/9229).
- Make etcd server return `raft.ErrProposalDropped` on internal Raft proposal drop in [v3 applier](https://github.com/etcd-io/etcd/pull/9549) and [v2 applier](https://github.com/etcd-io/etcd/pull/9558).
  - e.g. a node is removed from cluster, or [`raftpb.MsgProp` arrives at current leader while there is an ongoing leadership transfer](https://github.com/etcd-io/etcd/issues/8975).
- Add [`snapshot`](https://github.com/etcd-io/etcd/pull/9118) package for easier snapshot workflow (see [`godoc.org/github.com/etcd/clientv3/snapshot`](https://godoc.org/github.com/etcd-io/etcd/clientv3/snapshot) for more).
- Improve [functional tester](https://github.com/etcd-io/etcd/tree/master/functional) coverage: [proxy layer to run network fault tests in CI](https://github.com/etcd-io/etcd/pull/9081), [TLS is enabled both for server and client](https://github.com/etcd-io/etcd/pull/9534), [liveness mode](https://github.com/etcd-io/etcd/issues/9230), [shuffle test sequence](https://github.com/etcd-io/etcd/issues/9381), [membership reconfiguration failure cases](https://github.com/etcd-io/etcd/pull/9564), [disastrous quorum loss and snapshot recover from a seed member](https://github.com/etcd-io/etcd/pull/9565), [embedded etcd](https://github.com/etcd-io/etcd/pull/9572).
- Improve [index compaction blocking](https://github.com/etcd-io/etcd/pull/9511) by using a copy on write clone to avoid holding the lock for the traversal of the entire index.
- Update [JWT methods](https://github.com/etcd-io/etcd/pull/9883) to allow for use of any supported signature method/algorithm.
- Add [Lease checkpointing](https://github.com/etcd-io/etcd/pull/9924) to persist remaining TTLs to the consensus log periodically so that long lived leases progress toward expiry in the presence of leader elections and server restarts.
  - Enabled by experimental flag "--experimental-enable-lease-checkpoint".
- Add [gRPC interceptor for debugging logs](https://github.com/etcd-io/etcd/pull/9990); enable `etcd --debug` flag to see per-request debug information.
- Add [consistency check in snapshot status](https://github.com/etcd-io/etcd/pull/10109). If consistency check on snapshot file fails, `snapshot status` returns `"snapshot file integrity check failed..."` error.
- Add [`Verify` function to perform corruption check on WAL contents](https://github.com/etcd-io/etcd/pull/10603).
- Improve [heartbeat send failure logging](https://github.com/etcd-io/etcd/pull/10663).
- Support [users with no password](https://github.com/etcd-io/etcd/pull/9817) for reducing security risk introduced by leaked password. The users can only be authenticated with CommonName based auth.

### Breaking Changes

- Require [*Go 1.11+*](https://github.com/etcd-io/etcd/pull/10045).
- Use [Go module](https://github.com/etcd-io/etcd/pull/10063) for dependency management.
- Move [`"github.com/coreos/etcd"`](https://github.com/etcd-io/etcd/issues/9965) to [`"github.com/etcd-io/etcd"`](https://github.com/etcd-io/etcd/issues/9965).
  - Change import path to `"go.etcd.io/etcd"`.
  - e.g. `import "go.etcd.io/etcd/raft"`.
- Make [`ETCDCTL_API=3 etcdctl` default](https://github.com/etcd-io/etcd/issues/9600).
  - Now, `etcdctl set foo bar` must be `ETCDCTL_API=2 etcdctl set foo bar`.
  - Now, `ETCDCTL_API=3 etcdctl put foo bar` could be just `etcdctl put foo bar`.
- **Remove `etcd --ca-file` flag**, instead [use `etcd --trusted-ca-file`](https://github.com/etcd-io/etcd/pull/9470) (`etcd --ca-file` flag has been marked deprecated since v2.1).
- **Remove `etcd --peer-ca-file` flag**, instead [use `etcd --peer-trusted-ca-file`](https://github.com/etcd-io/etcd/pull/9470) (`etcd --peer-ca-file` flag has been marked deprecated since v2.1).
- **Remove `pkg/transport.TLSInfo.CAFile` field**, instead [use `pkg/transport.TLSInfo.TrustedCAFile`](https://github.com/etcd-io/etcd/pull/9470) (`CAFile` field has been marked deprecated since v2.1).
- Deprecate `latest` [release container](https://console.cloud.google.com/gcr/images/etcd-development/GLOBAL/etcd) tag.
  - **`docker pull gcr.io/etcd-development/etcd:latest` would not be up-to-date**.
- Deprecate [minor](https://semver.org/) version [release container](https://console.cloud.google.com/gcr/images/etcd-development/GLOBAL/etcd) tags.
  - `docker pull gcr.io/etcd-development/etcd:v3.3` would still work.
  - **`docker pull gcr.io/etcd-development/etcd:v3.4` would not work**.
  - Use **`docker pull gcr.io/etcd-development/etcd:v3.4.x`** instead, with the exact patch version.
- Drop [ACIs from official release](https://github.com/etcd-io/etcd/pull/9059).
  - [AppC was officially suspended](https://github.com/appc/spec#-disclaimer-), as of late 2016.
  - [`acbuild`](https://github.com/containers/build#this-project-is-currently-unmaintained) is not maintained anymore.
  - `*.aci` files are not available from `v3.4` release.
- Exit on [empty hosts in advertise URLs](https://github.com/etcd-io/etcd/pull/8786).
  - Address [advertise client URLs accepts empty hosts](https://github.com/etcd-io/etcd/issues/8379).
  - e.g. exit with error on `--advertise-client-urls=http://:2379`.
  - e.g. exit with error on `--initial-advertise-peer-urls=http://:2380`.
- Exit on [shadowed environment variables](https://github.com/etcd-io/etcd/pull/9382).
  - Address [error on shadowed environment variables](https://github.com/etcd-io/etcd/issues/8380).
  - e.g. exit with error on `ETCD_NAME=abc etcd --name=def`.
  - e.g. exit with error on `ETCD_INITIAL_CLUSTER_TOKEN=abc etcd --initial-cluster-token=def`.
  - e.g. exit with error on `ETCDCTL_ENDPOINTS=abc.com ETCDCTL_API=3 etcdctl endpoint health --endpoints=def.com`.
- Change [`etcdserverpb.AuthRoleRevokePermissionRequest/key,range_end` fields type from `string` to `bytes`](https://github.com/etcd-io/etcd/pull/9433).
- Rename [`etcd_debugging_mvcc_db_total_size_in_bytes` Prometheus metric to `etcd_mvcc_db_total_size_in_bytes`](https://github.com/etcd-io/etcd/pull/9819).
- Rename `etcdserver.ServerConfig.SnapCount` field to `etcdserver.ServerConfig.SnapshotCount`, to be consistent with the flag name `etcd --snapshot-count`.
- Rename `embed.Config.SnapCount` field to [`embed.Config.SnapshotCount`](https://github.com/etcd-io/etcd/pull/9745), to be consistent with the flag name `etcd --snapshot-count`.
- Change [`embed.Config.CorsInfo` in `*cors.CORSInfo` type to `embed.Config.CORS` in `map[string]struct{}` type](https://github.com/etcd-io/etcd/pull/9490).
- Remove [`embed.Config.SetupLogging`](https://github.com/etcd-io/etcd/pull/9572).
  - Now logger is set up automatically based on [`embed.Config.Logger`, `embed.Config.LogOutputs`, `embed.Config.Debug` fields](https://github.com/etcd-io/etcd/pull/9572).
- Rename [`etcd --log-output` to `etcd --log-outputs`](https://github.com/etcd-io/etcd/pull/9624) to support multiple log outputs.
  - **`etcd --log-output`** will be deprecated in v3.5.
- Rename [**`embed.Config.LogOutput`** to **`embed.Config.LogOutputs`**](https://github.com/etcd-io/etcd/pull/9624) to support multiple log outputs.
- Change [**`embed.Config.LogOutputs`** type from `string` to `[]string`](https://github.com/etcd-io/etcd/pull/9579) to support multiple log outputs.
  - Now that `etcd --log-outputs` accepts multiple writers, etcd configuration YAML file `log-outputs` field must be changed to `[]string` type.
  - Previously, `etcd --config-file etcd.config.yaml` can have `log-outputs: default` field, now must be `log-outputs: [default]`.
- Change v3 `etcdctl snapshot` exit codes with [`snapshot` package](https://github.com/etcd-io/etcd/pull/9118/commits/df689f4280e1cce4b9d61300be13ca604d41670a).
  - Exit on error with exit code 1 (no more exit code 5 or 6 on `snapshot save/restore` commands).
- Migrate dependency management tool from `glide` to [`golang/dep`](https://github.com/etcd-io/etcd/pull/9155).
  - <= 3.3 puts `vendor` directory under `cmd/vendor` directory to [prevent conflicting transitive dependencies](https://github.com/etcd-io/etcd/issues/4913).
  - 3.4 moves `cmd/vendor` directory to `vendor` at repository root.
  - Remove recursive symlinks in `cmd` directory.
  - Now `go get/install/build` on `etcd` packages (e.g. `clientv3`, `tools/benchmark`) enforce builds with etcd `vendor` directory.
- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint `/v3beta` with [`/v3`](https://github.com/etcd-io/etcd/pull/9298).
  - Deprecated [`/v3alpha`](https://github.com/etcd-io/etcd/pull/9298).
  - To deprecate [`/v3beta`](https://github.com/etcd-io/etcd/issues/9189) in v3.5.
  - In v3.4, `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` still works as a fallback to `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'`, but `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` won't work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
- Change [`wal` package function signatures](https://github.com/etcd-io/etcd/pull/9572) to support [structured logger and logging to file](https://github.com/etcd-io/etcd/issues/9438) in server-side.
  - Previously, `Open(dirpath string, snap walpb.Snapshot) (*WAL, error)`, now `Open(lg *zap.Logger, dirpath string, snap walpb.Snapshot) (*WAL, error)`.
  - Previously, `OpenForRead(dirpath string, snap walpb.Snapshot) (*WAL, error)`, now `OpenForRead(lg *zap.Logger, dirpath string, snap walpb.Snapshot) (*WAL, error)`.
  - Previously, `Repair(dirpath string) bool`, now `Repair(lg *zap.Logger, dirpath string) bool`.
  - Previously, `Create(dirpath string, metadata []byte) (*WAL, error)`, now `Create(lg *zap.Logger, dirpath string, metadata []byte) (*WAL, error)`.
- Remove [`pkg/cors` package](https://github.com/etcd-io/etcd/pull/9490).
- Change [`etcd --experimental-enable-v2v3`](TODO) flag to `etcd --enable-v2v3`; v2 storage emulation is now stable.
- Move internal packages to `etcdserver`.
  - `"github.com/coreos/etcd/alarm"` to `"go.etcd.io/etcd/etcdserver/api/v3alarm"`.
  - `"github.com/coreos/etcd/compactor"` to `"go.etcd.io/etcd/etcdserver/api/v3compactor"`.
  - `"github.com/coreos/etcd/discovery"` to `"go.etcd.io/etcd/etcdserver/api/v2discovery"`.
  - `"github.com/coreos/etcd/etcdserver/auth"` to `"go.etcd.io/etcd/etcdserver/api/v2auth"`.
  - `"github.com/coreos/etcd/etcdserver/membership"` to `"go.etcd.io/etcd/etcdserver/api/membership"`.
  - `"github.com/coreos/etcd/etcdserver/stats"` to `"go.etcd.io/etcd/etcdserver/api/v2stats"`.
  - `"github.com/coreos/etcd/error"` to `"go.etcd.io/etcd/etcdserver/api/v2error"`.
  - `"github.com/coreos/etcd/rafthttp"` to `"go.etcd.io/etcd/etcdserver/api/rafthttp"`.
  - `"github.com/coreos/etcd/snap"` to `"go.etcd.io/etcd/etcdserver/api/snap"`.
  - `"github.com/coreos/etcd/store"` to `"go.etcd.io/etcd/etcdserver/api/v2store"`.
- Change [snapshot file permissions](https://github.com/etcd-io/etcd/pull/9977): On Linux, the snapshot file changes from readable by all (mode 0644) to readable by the user only (mode 0600).

### Dependency

- Upgrade [`github.com/coreos/bbolt`](https://github.com/etcd-io/bbolt/releases) from [**`v1.3.1-coreos.6`**](https://github.com/etcd-io/bbolt/releases/tag/v1.3.1-coreos.6) to [`go.etcd.io/bbolt`](https://github.com/etcd-io/bbolt/releases) [**`v1.3.3`**](https://github.com/etcd-io/bbolt/releases/tag/v1.3.3).
- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) from [**`v1.7.5`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5) to [**`v1.13.0`**](https://github.com/grpc/grpc-go/releases/tag/v1.13.0).
- Upgrade [`github.com/golang/protobuf`](https://github.com/golang/protobuf/releases) from [**`golang/protobuf@1e59b77b5`**](https://github.com/golang/protobuf/commit/1e59b77b52bf8e4b449a57e6f79f21226d571845) to [**`v1.1.0`**](https://github.com/golang/protobuf/releases/tag/v1.1.0).
- Migrate [`github.com/ugorji/go/codec`](https://github.com/ugorji/go/releases) to [**`github.com/json-iterator/go`**](https://github.com/json-iterator/go), to [regenerate v2 `client`](https://github.com/etcd-io/etcd/pull/9494) (See [#10667](https://github.com/etcd-io/etcd/pull/10667) for more).
- Migrate [`github.com/ghodss/yaml`](https://github.com/ghodss/yaml/releases) to [**`sigs.k8s.io/yaml`**](https://github.com/kubernetes-sigs/yaml) (See [#10687](https://github.com/etcd-io/etcd/pull/10687) for more).
- Upgrade [`golang.org/x/crypto`](https://github.com/golang/crypto) from [**`crypto@9419663f5`**](https://github.com/golang/crypto/commit/9419663f5a44be8b34ca85f08abc5fe1be11f8a3) to [**`crypto@8ac0e0d97`**](https://github.com/golang/crypto/commit/8ac0e0d97ce45cd83d1d7243c060cb8461dda5e9).
- Upgrade [`golang.org/x/net`](https://github.com/golang/net) from [**`net@66aacef3d`**](https://github.com/golang/net/commit/66aacef3dd8a676686c7ae3716979581e8b03c47) to [**`net@db08ff08e`**](https://github.com/golang/net/commit/db08ff08e8622530d9ed3a0e8ac279f6d4c02196).
- Upgrade [`golang.org/x/sys`](https://github.com/golang/sys) from [**`sys@ebfc5b463`**](https://github.com/golang/sys/commit/ebfc5b4631820b793c9010c87fd8fef0f39eb082) to [**`sys@56ede360e`**](https://github.com/golang/sys/commit/56ede360ec1c541828fb88741b3f1049406d28f5).
- Upgrade [`golang.org/x/text`](https://github.com/golang/text) from [**`text@b19bf474d`**](https://github.com/golang/text/commit/b19bf474d317b857955b12035d2c5acb57ce8b01) to [**`text@f21a4dfb5`**](https://github.com/golang/text/commit/f21a4dfb5e38f5895301dc265a8def02365cc3d0).
- Upgrade [`golang.org/x/time`](https://github.com/golang/time) from [**`time@c06e80d93`**](https://github.com/golang/time/commit/c06e80d9300e4443158a03817b8a8cb37d230320) to [**`time@fbb02b229`**](https://github.com/golang/time/commit/fbb02b2291d28baffd63558aa44b4b56f178d650).
- Upgrade [`github.com/golang/protobuf`](https://github.com/golang/protobuf/releases) from [**`golang/protobuf@1e59b77b5`**](https://github.com/golang/protobuf/commit/1e59b77b52bf8e4b449a57e6f79f21226d571845) to [**`v1.1.0`**](https://github.com/golang/protobuf/releases/tag/v1.1.0).
- Upgrade [`gopkg.in/yaml.v2`](https://github.com/go-yaml/yaml/releases) from [**`yaml@cd8b52f82`**](https://github.com/go-yaml/yaml/commit/cd8b52f8269e0feb286dfeef29f8fe4d5b397e0b) to [**`yaml@5420a8b67`**](https://github.com/go-yaml/yaml/commit/5420a8b6744d3b0345ab293f6fcba19c978f1183).
- Upgrade [`github.com/dgrijalva/jwt-go`](https://github.com/dgrijalva/jwt-go/releases) from [**`v3.0.0`**](https://github.com/dgrijalva/jwt-go/releases/tag/v3.0.0) to [**`v3.2.0`**](https://github.com/dgrijalva/jwt-go/releases/tag/v3.2.0).
- Upgrade [`github.com/soheilhy/cmux`](https://github.com/soheilhy/cmux/releases) from [**`v0.1.3`**](https://github.com/soheilhy/cmux/releases/tag/v0.1.3) to [**`v0.1.4`**](https://github.com/soheilhy/cmux/releases/tag/v0.1.4).
- Upgrade [`github.com/google/btree`](https://github.com/google/btree/releases) from [**`google/btree@925471ac9`**](https://github.com/google/btree/commit/925471ac9e2131377a91e1595defec898166fe49) to [**`google/btree@e89373fe6`**](https://github.com/google/btree/commit/e89373fe6b4a7413d7acd6da1725b83ef713e6e4).
- Upgrade [`github.com/spf13/cobra`](https://github.com/spf13/cobra/releases) from [**`spf13/cobra@1c44ec8d3`**](https://github.com/spf13/cobra/commit/1c44ec8d3f1552cac48999f9306da23c4d8a288b) to [**`v0.0.3`**](https://github.com/spf13/cobra/releases/tag/v0.0.3).
- Upgrade [`github.com/spf13/pflag`](https://github.com/spf13/pflag/releases) from [**`v1.0.0`**](https://github.com/spf13/pflag/releases/tag/v1.0.0) to [**`spf13/pflag@1ce0cc6db`**](https://github.com/spf13/pflag/commit/1ce0cc6db4029d97571db82f85092fccedb572ce).
- Upgrade [`github.com/coreos/go-systemd`](https://github.com/coreos/go-systemd/releases) from [**`v15`**](https://github.com/coreos/go-systemd/releases/tag/v15) to [**`v17`**](https://github.com/coreos/go-systemd/releases/tag/v17).
- Upgrade [`github.com/prometheus/client_golang`](https://github.com/prometheus/client_golang/releases) from [**``prometheus/client_golang@5cec1d042``**](https://github.com/prometheus/client_golang/commit/5cec1d0429b02e4323e042eb04dafdb079ddf568) to [**`v0.8.0`**](https://github.com/prometheus/client_golang/releases/tag/v0.8.0).
- Upgrade [`github.com/grpc-ecosystem/go-grpc-prometheus`](https://github.com/grpc-ecosystem/go-grpc-prometheus/releases) from [**``grpc-ecosystem/go-grpc-prometheus@0dafe0d49``**](https://github.com/grpc-ecosystem/go-grpc-prometheus/commit/0dafe0d496ea71181bf2dd039e7e3f44b6bd11a7) to [**`v1.2.0`**](https://github.com/grpc-ecosystem/go-grpc-prometheus/releases/tag/v1.2.0).
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) from [**`v1.3.1`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3.1) to [**`v1.4.1`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.4.1).

### Metrics, Monitoring

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_snap_db_fsync_duration_seconds_count`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_snap_db_save_total_duration_seconds_bucket`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_send_success`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_send_failures`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_send_total_duration_seconds`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_receive_success`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_receive_failures`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_receive_total_duration_seconds`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_active_peers`](https://github.com/etcd-io/etcd/pull/9762) Prometheus metric.
  - Let's say `"7339c4e5e833c029"` server `/metrics` returns `etcd_network_active_peers{Local="7339c4e5e833c029",Remote="729934363faa4a24"} 1` and `etcd_network_active_peers{Local="7339c4e5e833c029",Remote="b548c2511513015"} 1`. This indicates that the local node `"7339c4e5e833c029"` currently has two active remote peers `"729934363faa4a24"` and `"b548c2511513015"` in a 3-node cluster. If the node `"b548c2511513015"` is down, the local node `"7339c4e5e833c029"` will show `etcd_network_active_peers{Local="7339c4e5e833c029",Remote="729934363faa4a24"} 1` and `etcd_network_active_peers{Local="7339c4e5e833c029",Remote="b548c2511513015"} 0`.
- Add [`etcd_network_disconnected_peers_total`](https://github.com/etcd-io/etcd/pull/9762) Prometheus metric.
  - If a remote peer `"b548c2511513015"` is down, the local node `"7339c4e5e833c029"` server `/metrics` would return `etcd_network_disconnected_peers_total{Local="7339c4e5e833c029",Remote="b548c2511513015"} 1`, while active peer metrics will show `etcd_network_active_peers{Local="7339c4e5e833c029",Remote="729934363faa4a24"} 1` and `etcd_network_active_peers{Local="7339c4e5e833c029",Remote="b548c2511513015"} 0`.
- Add [`etcd_network_server_stream_failures_total`](https://github.com/etcd-io/etcd/pull/9760) Prometheus metric.
  - e.g. `etcd_network_server_stream_failures_total{API="lease-keepalive",Type="receive"} 1`
  - e.g. `etcd_network_server_stream_failures_total{API="watch",Type="receive"} 1`
- Improve [`etcd_network_peer_round_trip_time_seconds`](https://github.com/etcd-io/etcd/pull/10155) Prometheus metric to track leader heartbeats.
  - Previously, it only samples the TCP connection for snapshot messages.
- Increase [`etcd_network_peer_round_trip_time_seconds`](https://github.com/etcd-io/etcd/pull/9762) Prometheus metric histogram upper-bound.
  - Previously, highest bucket only collects requests taking 0.8192 seconds or more.
  - Now, highest buckets collect 0.8192 seconds, 1.6384 seconds, and 3.2768 seconds or more.
- Add [`etcd_server_is_leader`](https://github.com/etcd-io/etcd/pull/9587) Prometheus metric.
- Add [`etcd_server_id`](https://github.com/etcd-io/etcd/pull/9998) Prometheus metric.
- Add [`etcd_cluster_version`](https://github.com/etcd-io/etcd/pull/10257) Prometheus metric.
- Add [`etcd_server_version`](https://github.com/etcd-io/etcd/pull/8960) Prometheus metric.
  - To replace [Kubernetes `etcd-version-monitor`](https://github.com/etcd-io/etcd/issues/8948).
- Add [`etcd_server_go_version`](https://github.com/etcd-io/etcd/pull/9957) Prometheus metric.
- Add [`etcd_server_health_success`](https://github.com/etcd-io/etcd/pull/10156) Prometheus metric.
- Add [`etcd_server_health_failures`](https://github.com/etcd-io/etcd/pull/10156) Prometheus metric.
- Add [`etcd_server_read_indexes_failed_total`](https://github.com/etcd-io/etcd/pull/10094) Prometheus metric.
- Add [`etcd_server_heartbeat_send_failures_total`](https://github.com/etcd-io/etcd/pull/9761) Prometheus metric.
- Add [`etcd_server_slow_apply_total`](https://github.com/etcd-io/etcd/pull/9761) Prometheus metric.
- Add [`etcd_server_slow_read_indexes_total`](https://github.com/etcd-io/etcd/pull/9897) Prometheus metric.
- Add [`etcd_server_quota_backend_bytes`](https://github.com/etcd-io/etcd/pull/9820) Prometheus metric.
  - Use it with `etcd_mvcc_db_total_size_in_bytes` and `etcd_mvcc_db_total_size_in_use_in_bytes`.
  - `etcd_server_quota_backend_bytes 2.147483648e+09` means current quota size is 2 GB.
  - `etcd_mvcc_db_total_size_in_bytes 20480` means current physically allocated DB size is 20 KB.
  - `etcd_mvcc_db_total_size_in_use_in_bytes 16384` means future DB size if defragment operation is complete.
  - `etcd_mvcc_db_total_size_in_bytes - etcd_mvcc_db_total_size_in_use_in_bytes` is the number of bytes that can be saved on disk with defragment operation.
- Add [`etcd_mvcc_db_total_size_in_bytes`](https://github.com/etcd-io/etcd/pull/9819) Prometheus metric.
  - Renamed from [`etcd_debugging_mvcc_db_total_size_in_bytes`](https://github.com/etcd-io/etcd/pull/9819).
- Add [`etcd_mvcc_db_total_size_in_use_in_bytes`](https://github.com/etcd-io/etcd/pull/9256) Prometheus metric.
  - Use it with `etcd_mvcc_db_total_size_in_bytes` and `etcd_mvcc_db_total_size_in_use_in_bytes`.
  - `etcd_server_quota_backend_bytes 2.147483648e+09` means current quota size is 2 GB.
  - `etcd_mvcc_db_total_size_in_bytes 20480` means current physically allocated DB size is 20 KB.
  - `etcd_mvcc_db_total_size_in_use_in_bytes 16384` means future DB size if defragment operation is complete.
  - `etcd_mvcc_db_total_size_in_bytes - etcd_mvcc_db_total_size_in_use_in_bytes` is the number of bytes that can be saved on disk with defragment operation.
- Add [`etcd_snap_fsync_duration_seconds`](https://github.com/etcd-io/etcd/pull/9762) Prometheus metric.
- Add [`etcd_disk_backend_defrag_duration_seconds`](https://github.com/etcd-io/etcd/pull/9761) Prometheus metric.
- Add [`etcd_mvcc_hash_duration_seconds`](https://github.com/etcd-io/etcd/pull/9761) Prometheus metric.
- Add [`etcd_mvcc_hash_rev_duration_seconds`](https://github.com/etcd-io/etcd/pull/9761) Prometheus metric.
- Add [`etcd_debugging_disk_backend_commit_rebalance_duration_seconds`](https://github.com/etcd-io/etcd/pull/9834) Prometheus metric.
- Add [`etcd_debugging_disk_backend_commit_spill_duration_seconds`](https://github.com/etcd-io/etcd/pull/9834) Prometheus metric.
- Add [`etcd_debugging_disk_backend_commit_write_duration_seconds`](https://github.com/etcd-io/etcd/pull/9834) Prometheus metric.
- Add [`etcd_debugging_lease_granted_total`](https://github.com/etcd-io/etcd/pull/9778) Prometheus metric.
- Add [`etcd_debugging_lease_revoked_total`](https://github.com/etcd-io/etcd/pull/9778) Prometheus metric.
- Add [`etcd_debugging_lease_renewed_total`](https://github.com/etcd-io/etcd/pull/9778) Prometheus metric.
- Add [`etcd_debugging_lease_ttl_total`](https://github.com/etcd-io/etcd/pull/9778) Prometheus metric.
- Add [`etcd_server_is_learner`](https://github.com/etcd-io/etcd/pull/10731) Prometheus metric.
- Add [`etcd_server_learner_promote_failures`](https://github.com/etcd-io/etcd/pull/10731) Prometheus metric.
- Add [`etcd_server_learner_promote_successes`](https://github.com/etcd-io/etcd/pull/10731) Prometheus metric.
- Increase [`etcd_debugging_mvcc_index_compaction_pause_duration_milliseconds`](https://github.com/etcd-io/etcd/pull/9762) Prometheus metric histogram upper-bound.
  - Previously, highest bucket only collects requests taking 1.024 seconds or more.
  - Now, highest buckets collect 1.024 seconds, 2.048 seconds, and 4.096 seconds or more.
- Fix missing [`etcd_network_peer_sent_failures_total`](https://github.com/etcd-io/etcd/pull/9437) Prometheus metric count.
- Fix [`etcd_debugging_server_lease_expired_total`](https://github.com/etcd-io/etcd/pull/9557) Prometheus metric.
- Fix [race conditions in v2 server stat collecting](https://github.com/etcd-io/etcd/pull/9562).
- Change [gRPC proxy to expose etcd server endpoint /metrics](https://github.com/etcd-io/etcd/pull/10618).
  - The metrics that were exposed via the proxy were not etcd server members but instead the proxy itself.
- Fix bug where [db_compaction_total_duration_milliseconds metric incorrectly measured duration as 0](https://github.com/etcd-io/etcd/pull/10646).

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- Support TLS cipher suite whitelisting.
  - To block [weak cipher suites](https://github.com/etcd-io/etcd/issues/8320).
  - TLS handshake fails when client hello is requested with invalid cipher suites.
  - Add [`etcd --cipher-suites`](https://github.com/etcd-io/etcd/pull/9801) flag.
  - If empty, Go auto-populates the list.
- Add [`etcd --host-whitelist`](https://github.com/etcd-io/etcd/pull/9372) flag, [`etcdserver.Config.HostWhitelist`](https://github.com/etcd-io/etcd/pull/9372), and [`embed.Config.HostWhitelist`](https://github.com/etcd-io/etcd/pull/9372), to prevent ["DNS Rebinding"](https://en.wikipedia.org/wiki/DNS_rebinding) attack.
  - Any website can simply create an authorized DNS name, and direct DNS to `"localhost"` (or any other address). Then, all HTTP endpoints of etcd server listening on `"localhost"` becomes accessible, thus vulnerable to [DNS rebinding attacks (CVE-2018-5702)](https://bugs.chromium.org/p/project-zero/issues/detail?id=1447#c2).
  - Client origin enforce policy works as follow:
    - If client connection is secure via HTTPS, allow any hostnames..
    - If client connection is not secure and `"HostWhitelist"` is not empty, only allow HTTP requests whose Host field is listed in whitelist.
  - By default, `"HostWhitelist"` is `"*"`, which means insecure server allows all client HTTP requests.
  - Note that the client origin policy is enforced whether authentication is enabled or not, for tighter controls.
  - When specifying hostnames, loopback addresses are not added automatically. To allow loopback interfaces, add them to whitelist manually (e.g. `"localhost"`, `"127.0.0.1"`, etc.).
  - e.g. `etcd --host-whitelist example.com`, then the server will reject all HTTP requests whose Host field is not `example.com` (also rejects requests to `"localhost"`).
- Support [`etcd --cors`](https://github.com/etcd-io/etcd/pull/9490) in v3 HTTP requests (gRPC gateway).
- Support [`ttl` field for `etcd` Authentication JWT token](https://github.com/etcd-io/etcd/pull/8302).
  - e.g. `etcd --auth-token jwt,pub-key=<pub key path>,priv-key=<priv key path>,sign-method=<sign method>,ttl=5m`.
- Allow empty token provider in [`etcdserver.ServerConfig.AuthToken`](https://github.com/etcd-io/etcd/pull/9369).
- Fix [TLS reload](https://github.com/etcd-io/etcd/pull/9570) when [certificate SAN field only includes IP addresses but no domain names](https://github.com/etcd-io/etcd/issues/9541).
  - In Go, server calls `(*tls.Config).GetCertificate` for TLS reload if and only if server's `(*tls.Config).Certificates` field is not empty, or `(*tls.ClientHelloInfo).ServerName` is not empty with a valid SNI from the client. Previously, etcd always populates `(*tls.Config).Certificates` on the initial client TLS handshake, as non-empty. Thus, client was always expected to supply a matching SNI in order to pass the TLS verification and to trigger `(*tls.Config).GetCertificate` to reload TLS assets.
  - However, a certificate whose SAN field does [not include any domain names but only IP addresses](https://github.com/etcd-io/etcd/issues/9541) would request `*tls.ClientHelloInfo` with an empty `ServerName` field, thus failing to trigger the TLS reload on initial TLS handshake; this becomes a problem when expired certificates need to be replaced online.
  - Now, `(*tls.Config).Certificates` is created empty on initial TLS client handshake, first to trigger `(*tls.Config).GetCertificate`, and then to populate rest of the certificates on every new TLS connection, even when client SNI is empty (e.g. cert only includes IPs).

### etcd server

- Add [`rpctypes.ErrLeaderChanged`](https://github.com/etcd-io/etcd/pull/10094).
  - Now linearizable requests with read index would fail fast when there is a leadership change, instead of waiting until context timeout.
- Add [`etcd --initial-election-tick-advance`](https://github.com/etcd-io/etcd/pull/9591) flag to configure initial election tick fast-forward.
  - By default, `etcd --initial-election-tick-advance=true`, then local member fast-forwards election ticks to speed up "initial" leader election trigger.
  - This benefits the case of larger election ticks. For instance, cross datacenter deployment may require longer election timeout of 10-second. If true, local node does not need wait up to 10-second. Instead, forwards its election ticks to 8-second, and have only 2-second left before leader election.
  - Major assumptions are that: cluster has no active leader thus advancing ticks enables faster leader election. Or cluster already has an established leader, and rejoining follower is likely to receive heartbeats from the leader after tick advance and before election timeout.
  - However, when network from leader to rejoining follower is congested, and the follower does not receive leader heartbeat within left election ticks, disruptive election has to happen thus affecting cluster availabilities.
  - Now, this can be disabled by setting `etcd --initial-election-tick-advance=false`.
  - Disabling this would slow down initial bootstrap process for cross datacenter deployments. Make tradeoffs by configuring `etcd --initial-election-tick-advance` at the cost of slow initial bootstrap.
  - If single-node, it advances ticks regardless.
  - Address [disruptive rejoining follower node](https://github.com/etcd-io/etcd/issues/9333).
- Add [`etcd --pre-vote`](https://github.com/etcd-io/etcd/pull/9352) flag to enable to run an additional Raft election phase.
  - For instance, a flaky(or rejoining) member may drop in and out, and start campaign. This member will end up with a higher term, and ignore all incoming messages with lower term. In this case, a new leader eventually need to get elected, thus disruptive to cluster availability. Raft implements Pre-Vote phase to prevent this kind of disruptions. If enabled, Raft runs an additional phase of election to check if pre-candidate can get enough votes to win an election.
  - `etcd --pre-vote=false` by default.
  - v3.5 will enable `etcd --pre-vote=true` by default.
- [`etcd --initial-corrupt-check`](TODO) flag is now stable (`etcd --experimental-initial-corrupt-check`haisbeen  deprecated).
  - `etcd --initial-corrupt-check=true` by default, to check cluster database hashes before serving client/peer traffic.
- [`etcd --corrupt-check-time`](TODO) flag is now stable (`etcd --experimental-corrupt-check-time`haisbeen  deprecated).
  - `etcd --corrupt-check-time=12h` by default, to check cluster database hashes for every 12-hour.
- [`etcd --enable-v2v3`](TODO) flag is now stable.
  - `etcd --experimental-enable-v2v3` has been deprecated.
  - Added [more v2v3 integration tests](https://github.com/etcd-io/etcd/pull/9634).
  - `etcd --enable-v2=true --enable-v2v3=''` by default, to enable v2 API server that is backed by **v2 store**.
  - `etcd --enable-v2=true --enable-v2v3=/aaa` to enable v2 API server that is backed by **v3 storage**.
  - `etcd --enable-v2=false --enable-v2v3=''` to disable v2 API server.
  - `etcd --enable-v2=false --enable-v2v3=/aaa` to disable v2 API server. TODO: error?
  - Automatically [create parent directory if it does not exist](https://github.com/etcd-io/etcd/pull/9626) (fix [issue#9609](https://github.com/etcd-io/etcd/issues/9609)).
  - v4.0 will configure `etcd --enable-v2=true --enable-v2v3=/aaa` to enable v2 API server that is backed by **v3 storage**.
- Add [`etcd --discovery-srv-name`](https://github.com/etcd-io/etcd/pull/8690) flag to support custom DNS SRV name with discovery.
  - If not given, etcd queries `_etcd-server-ssl._tcp.[YOUR_HOST]` and `_etcd-server._tcp.[YOUR_HOST]`.
  - If `etcd --discovery-srv-name="foo"`, then query `_etcd-server-ssl-foo._tcp.[YOUR_HOST]` and `_etcd-server-foo._tcp.[YOUR_HOST]`.
  - Useful for operating multiple etcd clusters under the same domain.
- Support TLS cipher suite whitelisting.
  - To block [weak cipher suites](https://github.com/etcd-io/etcd/issues/8320).
  - TLS handshake fails when client hello is requested with invalid cipher suites.
  - Add [`etcd --cipher-suites`](https://github.com/etcd-io/etcd/pull/9801) flag.
  - If empty, Go auto-populates the list.
- Support [`etcd --cors`](https://github.com/etcd-io/etcd/pull/9490) in v3 HTTP requests (gRPC gateway).
- Rename [`etcd --log-output` to `etcd --log-outputs`](https://github.com/etcd-io/etcd/pull/9624) to support multiple log outputs.
  - **`etcd --log-output` will be deprecated in v3.5**.
- Add [`etcd --logger`](https://github.com/etcd-io/etcd/pull/9572) flag to support [structured logger and multiple log outputs](https://github.com/etcd-io/etcd/issues/9438) in server-side.
  - **`etcd --logger=capnslog` will be deprecated in v3.5**.
  - Main motivation is to promote automated etcd monitoring, rather than looking back server logs when it starts breaking. Future development will make etcd log as few as possible, and make etcd easier to monitor with metrics and alerts.
  - `etcd --logger=capnslog --log-outputs=default` is the default setting and same as previous etcd server logging format.
  - `etcd --logger=zap --log-outputs=default` is not supported when `etcd --logger=zap`.
    - Instead, use `etcd --logger=zap --log-outputs=stderr`.
    - Or, use `etcd --logger=zap --log-outputs=systemd/journal` to send logs to the local systemd journal.
    - Previously, if etcd parent process ID (PPID) is 1 (e.g. run with systemd), `etcd --logger=capnslog --log-outputs=default` redirects server logs to local systemd journal. And if write to journald fails, it writes to `os.Stderr` as a fallback.
    - However, even with PPID 1, it can fail to dial systemd journal (e.g. run embedded etcd with Docker container). Then, [every single log write will fail](https://github.com/etcd-io/etcd/pull/9729) and fall back to `os.Stderr`, which is inefficient.
    - To avoid this problem, systemd journal logging must be configured manually.
  - `etcd --logger=zap --log-outputs=stderr` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to `os.Stderr`. Use this to override journald log redirects.
  - `etcd --logger=zap --log-outputs=stdout` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to `os.Stdout` Use this to override journald log redirects.
  - `etcd --logger=zap --log-outputs=a.log` will log server operations in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig) and writes logs to the specified file `a.log`.
  - `etcd --logger=zap --log-outputs=a.log,b.log,c.log,stdout` [writes server logs to multiple files `a.log`, `b.log` and `c.log` at the same time](https://github.com/etcd-io/etcd/pull/9579) and outputs to `os.Stderr`, in [JSON-encoded format](https://godoc.org/go.uber.org/zap#NewProductionEncoderConfig).
  - `etcd --logger=zap --log-outputs=/dev/null` will discard all server logs.
- Add [`etcd --backend-batch-limit`](https://github.com/etcd-io/etcd/pull/10283) flag.
- Add [`etcd --backend-batch-interval`](https://github.com/etcd-io/etcd/pull/10283) flag.
- Fix [`mvcc` "unsynced" watcher restore operation](https://github.com/etcd-io/etcd/pull/9281).
  - "unsynced" watcher is watcher that needs to be in sync with events that have happened.
  - That is, "unsynced" watcher is the slow watcher that was requested on old revision.
  - "unsynced" watcher restore operation was not correctly populating its underlying watcher group.
  - Which possibly causes [missing events from "unsynced" watchers](https://github.com/etcd-io/etcd/issues/9086).
  - A node gets network partitioned with a watcher on a future revision, and falls behind receiving a leader snapshot after partition gets removed. When applying this snapshot, etcd watch storage moves current synced watchers to unsynced since sync watchers might have become stale during network partition. And reset synced watcher group to restart watcher routines. Previously, there was a bug when moving from synced watcher group to unsynced, thus client would miss events when the watcher was requested to the network-partitioned node.
- Fix [`mvcc` server panic from restore operation](https://github.com/etcd-io/etcd/pull/9775).
  - Let's assume that a watcher had been requested with a future revision X and sent to node A that became network-partitioned thereafter. Meanwhile, cluster makes progress. Then when the partition gets removed, the leader sends a snapshot to node A. Previously if the snapshot's latest revision is still lower than the watch revision X,  **etcd server panicked** during snapshot restore operation.
  - Now, this server-side panic has been fixed.
- Fix [server panic on invalid Election Proclaim/Resign HTTP(S) requests](https://github.com/etcd-io/etcd/pull/9379).
  - Previously, wrong-formatted HTTP requests to Election API could trigger panic in etcd server.
  - e.g. `curl -L http://localhost:2379/v3/election/proclaim -X POST -d '{"value":""}'`, `curl -L http://localhost:2379/v3/election/resign -X POST -d '{"value":""}'`.
- Fix [revision-based compaction retention parsing](https://github.com/etcd-io/etcd/pull/9339).
  - Previously, `etcd --auto-compaction-mode revision --auto-compaction-retention 1` was [translated to revision retention 3600000000000](https://github.com/etcd-io/etcd/issues/9337).
  - Now, `etcd --auto-compaction-mode revision --auto-compaction-retention 1` is correctly parsed as revision retention 1.
- Prevent [overflow by large `TTL` values for `Lease` `Grant`](https://github.com/etcd-io/etcd/pull/9399).
  - `TTL` parameter to `Grant` request is unit of second.
  - Leases with too large `TTL` values exceeding `math.MaxInt64` [expire in unexpected ways](https://github.com/etcd-io/etcd/issues/9374).
  - Server now returns `rpctypes.ErrLeaseTTLTooLarge` to client, when the requested `TTL` is larger than *9,000,000,000 seconds* (which is >285 years).
  - Again, etcd `Lease` is meant for short-periodic keepalives or sessions, in the range of seconds or minutes. Not for hours or days!
- Enable etcd server [`raft.Config.CheckQuorum` when starting with `ForceNewCluster`](https://github.com/etcd-io/etcd/pull/9347).
- Allow [non-WAL files in `etcd --wal-dir` directory](https://github.com/etcd-io/etcd/pull/9743).
  - Previously, existing files such as [`lost+found`](https://github.com/etcd-io/etcd/issues/7287) in WAL directory prevent etcd server boot.
  - Now, WAL directory that contains only `lost+found` or a file that's not suffixed with `.wal` is considered non-initialized.

### API

- Add `isLearner` field to `etcdserverpb.Member`, `etcdserverpb.MemberAddRequest` and `etcdserverpb.StatusResponse` as part of [raft learner implementation](https://github.com/etcd-io/etcd/pull/10725).
- Add `MemberPromote` rpc to `etcdserverpb.Cluster` interface and the corresponding `MemberPromoteRequest` and `MemberPromoteResponse` as part of [raft learner implementation](https://github.com/etcd-io/etcd/pull/10725).
- Add [`snapshot`](https://github.com/etcd-io/etcd/pull/9118) package for snapshot restore/save operations (see [`godoc.org/github.com/etcd/clientv3/snapshot`](https://godoc.org/github.com/coreos/etcd/clientv3/snapshot) for more).
- Add [`watch_id` field to `etcdserverpb.WatchCreateRequest`](https://github.com/etcd-io/etcd/pull/9065) to allow user-provided watch ID to `mvcc`.
  - Corresponding `watch_id` is returned via `etcdserverpb.WatchResponse`, if any.
- Add [`fragment` field to `etcdserverpb.WatchCreateRequest`](https://github.com/etcd-io/etcd/pull/9291) to request etcd server to [split watch events](https://github.com/etcd-io/etcd/issues/9294) when the total size of events exceeds `etcd --max-request-bytes` flag value plus gRPC-overhead 512 bytes.
  - The default server-side request bytes limit is `embed.DefaultMaxRequestBytes` which is 1.5 MiB plus gRPC-overhead 512 bytes.
  - If watch response events exceed this server-side request limit and watch request is created with `fragment` field `true`, the server will split watch events into a set of chunks, each of which is a subset of watch events below server-side request limit.
  - Useful when client-side has limited bandwidths.
  - For example, watch response contains 10 events, where each event is 1 MiB. And server `etcd --max-request-bytes` flag value is 1 MiB. Then, server will send 10 separate fragmented events to the client.
  - For example, watch response contains 5 events, where each event is 2 MiB. And server `etcd --max-request-bytes` flag value is 1 MiB and `clientv3.Config.MaxCallRecvMsgSize` is 1 MiB. Then, server will try to send 5 separate fragmented events to the client, and the client will error with `"code = ResourceExhausted desc = grpc: received message larger than max (...)"`.
  - Client must implement fragmented watch event merge (which `clientv3` does in etcd v3.4).
- Add [`raftAppliedIndex` field to `etcdserverpb.StatusResponse`](https://github.com/etcd-io/etcd/pull/9176) for current Raft applied index.
- Add [`errors` field to `etcdserverpb.StatusResponse`](https://github.com/etcd-io/etcd/pull/9206) for server-side error.
  - e.g. `"etcdserver: no leader", "NOSPACE", "CORRUPT"`
- Add [`dbSizeInUse` field to `etcdserverpb.StatusResponse`](https://github.com/etcd-io/etcd/pull/9256) for actual DB size after compaction.
- Add [`WatchRequest.WatchProgressRequest`](https://github.com/etcd-io/etcd/pull/9869).
  - To manually trigger broadcasting watch progress event (empty watch response with latest header) to all associated watch streams.
  - Think of it as `WithProgressNotify` that can be triggered manually.

Note: **v3.5 will deprecate `etcd --log-package-levels` flag for `capnslog`**; `etcd --logger=zap --log-outputs=stderr` will the default. **v3.5 will deprecate `[CLIENT-URL]/config/local/log` endpoint.**

### Package `embed`

- Add [`embed.Config.CipherSuites`](https://github.com/etcd-io/etcd/pull/9801) to specify a list of supported cipher suites for TLS handshake between client/server  and peers.
  - If empty, Go auto-populates the list.
  - Both `embed.Config.ClientTLSInfo.CipherSuites` and `embed.Config.CipherSuites` cannot be non-empty at the same time.
  - If not empty, specify either `embed.Config.ClientTLSInfo.CipherSuites` or `embed.Config.CipherSuites`.
- Add [`embed.Config.InitialElectionTickAdvance`](https://github.com/etcd-io/etcd/pull/9591) to enable/disable initial election tick fast-forward.
  - `embed.NewConfig()` would return `*embed.Config` with `InitialElectionTickAdvance` as true by default.
- Define [`embed.CompactorModePeriodic`](https://godoc.org/github.com/etcd-io/etcd/embed#pkg-variables) for `compactor.ModePeriodic`.
- Define [`embed.CompactorModeRevision`](https://godoc.org/github.com/etcd-io/etcd/embed#pkg-variables) for `compactor.ModeRevision`.
- Change [`embed.Config.CorsInfo` in `*cors.CORSInfo` type to `embed.Config.CORS` in `map[string]struct{}` type](https://github.com/etcd-io/etcd/pull/9490).
- Remove [`embed.Config.SetupLogging`](https://github.com/etcd-io/etcd/pull/9572).
  - Now logger is set up automatically based on [`embed.Config.Logger`, `embed.Config.LogOutputs`, `embed.Config.Debug` fields](https://github.com/etcd-io/etcd/pull/9572).
- Add [`embed.Config.Logger`](https://github.com/etcd-io/etcd/pull/9518) to support [structured logger `zap`](https://github.com/uber-go/zap) in server-side.
- Rename `embed.Config.SnapCount` field to [`embed.Config.SnapshotCount`](https://github.com/etcd-io/etcd/pull/9745), to be consistent with the flag name `etcd --snapshot-count`.
- Rename [**`embed.Config.LogOutput`** to **`embed.Config.LogOutputs`**](https://github.com/etcd-io/etcd/pull/9624) to support multiple log outputs.
- Change [**`embed.Config.LogOutputs`** type from `string` to `[]string`](https://github.com/etcd-io/etcd/pull/9579) to support multiple log outputs.
- Add [`embed.Config.BackendBatchLimit`](https://github.com/etcd-io/etcd/pull/10283) field.
- Add [`embed.Config.BackendBatchInterval`](https://github.com/etcd-io/etcd/pull/10283) field.

### Package `integration`

- Add [`CLUSTER_DEBUG` to enable test cluster logging](https://github.com/etcd-io/etcd/pull/9678).
  - Deprecated `capnslog` in integration tests.

### client v3

- Add [`MemberAddAsLearner`](https://github.com/etcd-io/etcd/pull/10725) to `Clientv3.Cluster` interface. This API is used to add a learner member to etcd cluster.
- Add [`MemberPromote`](https://github.com/etcd-io/etcd/pull/10727) to `Clientv3.Cluster` interface. This API is used to promote a learner member in etcd cluster.
- Client may receive [`rpctypes.ErrLeaderChanged`](https://github.com/etcd-io/etcd/pull/10094) from server.
  - Now linearizable requests with read index would fail fast when there is a leadership change, instead of waiting until context timeout.
- Add [`WithFragment` `OpOption`](https://github.com/etcd-io/etcd/pull/9291) to support [watch events fragmentation](https://github.com/etcd-io/etcd/issues/9294) when the total size of events exceeds `etcd --max-request-bytes` flag value plus gRPC-overhead 512 bytes.
  - Watch fragmentation is disabled by default.
  - The default server-side request bytes limit is `embed.DefaultMaxRequestBytes` which is 1.5 MiB plus gRPC-overhead 512 bytes.
  - If watch response events exceed this server-side request limit and watch request is created with `fragment` field `true`, the server will split watch events into a set of chunks, each of which is a subset of watch events below server-side request limit.
  - Useful when client-side has limited bandwidths.
  - For example, watch response contains 10 events, where each event is 1 MiB. And server `etcd --max-request-bytes` flag value is 1 MiB. Then, server will send 10 separate fragmented events to the client.
  - For example, watch response contains 5 events, where each event is 2 MiB. And server `etcd --max-request-bytes` flag value is 1 MiB and `clientv3.Config.MaxCallRecvMsgSize` is 1 MiB. Then, server will try to send 5 separate fragmented events to the client, and the client will error with `"code = ResourceExhausted desc = grpc: received message larger than max (...)"`.
- Add [`Watcher.RequestProgress` method](https://github.com/etcd-io/etcd/pull/9869).
  - To manually trigger broadcasting watch progress event (empty watch response with latest header) to all associated watch streams.
  - Think of it as `WithProgressNotify` that can be triggered manually.
- Fix [lease keepalive interval updates when response queue is full](https://github.com/etcd-io/etcd/pull/9952).
  - If `<-chan *clientv3LeaseKeepAliveResponse` from `clientv3.Lease.KeepAlive` was never consumed or channel is full, client was [sending keepalive request every 500ms](https://github.com/etcd-io/etcd/issues/9911) instead of expected rate of every "TTL / 3" duration.
- Change [snapshot file permissions](https://github.com/etcd-io/etcd/pull/9977): On Linux, the snapshot file changes from readable by all (mode 0644) to readable by the user only (mode 0600).
- Client may choose to send keepalive pings to server using [`PermitWithoutStream`](https://github.com/etcd-io/etcd/pull/10146).
  - By setting `PermitWithoutStream` to true, client can send keepalive pings to server without any active streams(RPCs). In other words, it allows sending keepalive pings with unary or simple RPC calls.
  - `PermitWithoutStream` is set to false by default.
- Fix logic on [release lock key if cancelled](https://github.com/etcd-io/etcd/pull/10153) in `clientv3/concurrency` package.
- Fix [`(*Client).Endpoints()` method race condition](https://github.com/etcd-io/etcd/pull/10595).

### etcdctl v3

- Make [`ETCDCTL_API=3 etcdctl` default](https://github.com/etcd-io/etcd/issues/9600).
  - Now, `etcdctl set foo bar` must be `ETCDCTL_API=2 etcdctl set foo bar`.
  - Now, `ETCDCTL_API=3 etcdctl put foo bar` could be just `etcdctl put foo bar`.
- Add [`etcdctl member add --learner` and `etcdctl member promote`](https://github.com/etcd-io/etcd/pull/10725) to add and promote raft learner member in etcd cluster.
- Add [`etcdctl --password`](https://github.com/etcd-io/etcd/pull/9730) flag.
  - To support [`:` character in user name](https://github.com/etcd-io/etcd/issues/9691).
  - e.g. `etcdctl --user user --password password get foo`
- Add [`etcdctl user add --new-user-password`](https://github.com/etcd-io/etcd/pull/9730) flag.
- Add [`etcdctl check datascale`](https://github.com/etcd-io/etcd/pull/9185) command.
- Add [`etcdctl check datascale --auto-compact, --auto-defrag`](https://github.com/etcd-io/etcd/pull/9351) flags.
- Add [`etcdctl check perf --auto-compact, --auto-defrag`](https://github.com/etcd-io/etcd/pull/9330) flags.
- Add [`etcdctl defrag --cluster`](https://github.com/etcd-io/etcd/pull/9390) flag.
- Add ["raft applied index" field to `endpoint status`](https://github.com/etcd-io/etcd/pull/9176).
- Add ["errors" field to `endpoint status`](https://github.com/etcd-io/etcd/pull/9206).
- Add [`etcdctl endpoint health --write-out` support](https://github.com/etcd-io/etcd/pull/9540).
  - Previously, [`etcdctl endpoint health --write-out json` did not work](https://github.com/etcd-io/etcd/issues/9532).
- Fix [`etcdctl watch [key] [range_end] -- [exec-command…]`](https://github.com/etcd-io/etcd/pull/9688) parsing.
  - Previously,  `ETCDCTL_API=3 etcdctl watch foo -- echo watch event received` panicked.
- Fix [`etcdctl move-leader` command for TLS-enabled endpoints](https://github.com/etcd-io/etcd/pull/9807).
- Add [`progress` command to `etcdctl watch --interactive`](https://github.com/etcd-io/etcd/pull/9869).
  - To manually trigger broadcasting watch progress event (empty watch response with latest header) to all associated watch streams.
  - Think of it as `WithProgressNotify` that can be triggered manually.
- Add [timeout](https://github.com/etcd-io/etcd/pull/10301) to `etcdctl snapshot
  save`.
  - User can specify timeout of `etcdctl snapshot save` command using flag
    `--command-timeout`.
  - Fix etcdctl to [strip out insecure endpoints from DNS SRV records when using discovery](https://github.com/etcd-io/etcd/pull/10443)

### gRPC proxy

- Fix [etcd server panic from restore operation](https://github.com/etcd-io/etcd/pull/9775).
  - Let's assume that a watcher had been requested with a future revision X and sent to node A that became network-partitioned thereafter. Meanwhile, cluster makes progress. Then when the partition gets removed, the leader sends a snapshot to node A. Previously if the snapshot's latest revision is still lower than the watch revision X,  **etcd server panicked** during snapshot restore operation.
  - Especially, gRPC proxy was affected, since it detects a leader loss with a key `"proxy-namespace__lostleader"` and a watch revision `"int64(math.MaxInt64 - 2)"`.
  - Now, this server-side panic has been fixed.
- Fix [memory leak in cache layer](https://github.com/etcd-io/etcd/pull/10327).
- Change [gRPC proxy to expose etcd server endpoint /metrics](https://github.com/etcd-io/etcd/pull/10618).
  - The metrics that were exposed via the proxy were not etcd server members but instead the proxy itself.

### gRPC gateway

- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint `/v3beta` with [`/v3`](https://github.com/etcd-io/etcd/pull/9298).
  - Deprecated [`/v3alpha`](https://github.com/etcd-io/etcd/pull/9298).
  - To deprecate [`/v3beta`](https://github.com/etcd-io/etcd/issues/9189) in v3.5.
  - In v3.4, `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` still works as a fallback to `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'`, but `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` won't work in v3.5. Use `curl -L http://localhost:2379/v3/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
- Add API endpoints [`/{v3beta,v3}/lease/leases, /{v3beta,v3}/lease/revoke, /{v3beta,v3}/lease/timetolive`](https://github.com/etcd-io/etcd/pull/9450).
  - To deprecate [`/{v3beta,v3}/kv/lease/leases, /{v3beta,v3}/kv/lease/revoke, /{v3beta,v3}/kv/lease/timetolive`](https://github.com/etcd-io/etcd/issues/9430) in v3.5.
- Support [`etcd --cors`](https://github.com/etcd-io/etcd/pull/9490) in v3 HTTP requests (gRPC gateway).

### Package `raft`

- Fix [deadlock during PreVote migration process](https://github.com/etcd-io/etcd/pull/8525).
- Add [`raft.ErrProposalDropped`](https://github.com/etcd-io/etcd/pull/9067).
  - Now [`(r *raft) Step` returns `raft.ErrProposalDropped`](https://github.com/etcd-io/etcd/pull/9137) if a proposal has been ignored.
  - e.g. a node is removed from cluster, or [`raftpb.MsgProp` arrives at current leader while there is an ongoing leadership transfer](https://github.com/etcd-io/etcd/issues/8975).
- Improve [Raft `becomeLeader` and `stepLeader`](https://github.com/etcd-io/etcd/pull/9073) by keeping track of latest `pb.EntryConfChange` index.
  - Previously record `pendingConf` boolean field scanning the entire tail of the log, which can delay hearbeat send.
- Fix [missing learner nodes on `(n *node) ApplyConfChange`](https://github.com/etcd-io/etcd/pull/9116).
- Add [`raft.Config.MaxUncommittedEntriesSize`](https://github.com/etcd-io/etcd/pull/10167) to limit the total size of the uncommitted entries in bytes.
  - Once exceeded, raft returns `raft.ErrProposalDropped` error.
  - Prevent [unbounded Raft log growth](https://github.com/cockroachdb/cockroach/issues/27772).
  - There was a bug in [PR#10167](https://github.com/etcd-io/etcd/pull/10167) but fixed via [PR#10199](https://github.com/etcd-io/etcd/pull/10199).
- Add [`raft.Ready.CommittedEntries` pagination using `raft.Config.MaxSizePerMsg`](https://github.com/etcd-io/etcd/pull/9982).
  - This prevents out-of-memory errors if the raft log has become very large and commits all at once.
  - Fix [correctness bug in CommittedEntries pagination](https://github.com/etcd-io/etcd/pull/10063).
- Optimize [message send flow control](https://github.com/etcd-io/etcd/pull/9985).
  - Leader now sends more append entries if it has more non-empty entries to send after updating flow control information.
  - Now, Raft allows multiple in-flight append messages.
- Optimize [memory allocation when boxing slice in `maybeCommit`](https://github.com/etcd-io/etcd/pull/10679).
  - By boxing a heap-allocated slice header instead of the slice header on the stack, we can avoid an allocation when passing through the sort.Interface interface.
- Avoid [memory allocation in Raft entry `String` method](https://github.com/etcd-io/etcd/pull/10680).
- Avoid [multiple memory allocations when merging stable and unstable log](https://github.com/etcd-io/etcd/pull/10684).
- Extract [progress tracking into own component](https://github.com/etcd-io/etcd/pull/10683).

### Package `wal`

- Add [`Verify` function to perform corruption check on WAL contents](https://github.com/etcd-io/etcd/pull/10603).

### Tooling

- Add [`etcd-dump-logs --entry-type`](https://github.com/etcd-io/etcd/pull/9628) flag to support WAL log filtering by entry type.
- Add [`etcd-dump-logs --stream-decoder`](https://github.com/etcd-io/etcd/pull/9790) flag to support custom decoder.

### Go

- Require [*Go 1.11+*](https://github.com/etcd-io/etcd/pull/10045).
- Compile with [*Go 1.11*](https://golang.org/doc/devel/release.html#go1.11).


<hr>

