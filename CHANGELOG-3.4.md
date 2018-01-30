

## v3.4.0 (TBD)

**v3.4.0 is not yet released.**

### Changed(Breaking Changes)

- Drop [ACIs from official release](https://github.com/coreos/etcd/pull/9059).
  - [AppC was officially suspended](https://github.com/appc/spec#-disclaimer-), as of late 2016.
  - [`acbuild`](https://github.com/containers/build#this-project-is-currently-unmaintained) is not maintained anymore.
  - `*.aci` files are not available from `v3.4` release.
- Exit on [empty hosts in advertise URLs](https://github.com/coreos/etcd/pull/8786).
  - Address [advertise client URLs accepts empty hosts](https://github.com/coreos/etcd/issues/8379).
  - e.g. exit with error on `--advertise-client-urls=http://:2379`.
  - e.g. exit with error on `--initial-advertise-peer-urls=http://:2380`.
- Exit on [shadowed environment variables](TODO).
  - Address [error on shadowed environment variables](https://github.com/coreos/etcd/issues/8380).
  - e.g. exit with error on `ETCD_INITIAL_CLUSTER_TOKEN=abc etcd --initial-cluster-token=def`.
- Migrate dependency management tool from `glide` to [`golang/dep`](https://github.com/coreos/etcd/pull/9155).
  - <= 3.3 puts `vendor` directory under `cmd/vendor` directory to [prevent conflicting transitive dependencies](https://github.com/coreos/etcd/issues/4913).
  - 3.4 moves `cmd/vendor` directory to `vendor` at repository root.
  - Remove recursive symlinks in `cmd` directory.
  - Now `go get/install/build` on `etcd` packages (e.g. `clientv3`, `tools/benchmark`) enforce builds with etcd `vendor` directory.
- Reorganize [internal packages](https://github.com/coreos/etcd/issues/9220).
  - `internal/*` packages [cannot be(are not meant to be) imported by external projects](https://docs.google.com/document/d/1e8kOo3r51b2BWtTs_1uADIA5djfXhPT36s6eHVRIvaU/edit).
  - Move `"github.com/coreos/etcd/alarm"` to [`"github.com/coreos/etcd/internal/alarm"`](https://github.com/coreos/etcd/pull/9234).
  - Move `"github.com/coreos/etcd/auth"` to [`"github.com/coreos/etcd/internal/auth"`](https://github.com/coreos/etcd/pull/9243).
  - Move `"github.com/coreos/etcd/compactor"` to [`"github.com/coreos/etcd/internal/compactor"`](https://github.com/coreos/etcd/pull/9234).
    - [`embed.CompactorModePeriodic`](https://github.com/coreos/etcd/pull/9247) to replace `compactor.ModePeriodic`.
    - [`embed.CompactorModeRevision`](https://github.com/coreos/etcd/pull/9247) to replace `compactor.ModeRevision`.
  - Move `"github.com/coreos/etcd/discovery"` to [`"github.com/coreos/etcd/internal/discovery"`](https://github.com/coreos/etcd/pull/9233).
  - Move `"github.com/coreos/etcd/lease"` to [`"github.com/coreos/etcd/internal/lease"`](https://github.com/coreos/etcd/pull/9238).
  - Move `"github.com/coreos/etcd/mvcc"` to [`"github.com/coreos/etcd/internal/mvcc"`](https://github.com/coreos/etcd/pull/9238).
  - Move `"github.com/coreos/etcd/snap"` to [`"github.com/coreos/etcd/internal/raftsnap"`](https://github.com/coreos/etcd/pull/9211).
  - Move `"github.com/coreos/etcd/store"` to [`"github.com/coreos/etcd/internal/store"`](https://github.com/coreos/etcd/pull/9238).
  - Move `"github.com/coreos/etcd/version"` to [`"github.com/coreos/etcd/internal/version"`](https://github.com/coreos/etcd/pull/9244).

### Added(`etcd`)

- Add [`--discovery-srv-name`](https://github.com/coreos/etcd/pull/8690) flag to support custom DNS SRV name with discovery.
  - If not given, etcd queries `_etcd-server-ssl._tcp.[YOUR_HOST]` and `_etcd-server._tcp.[YOUR_HOST]`.
  - If `--discovery-srv-name="foo"`, then query `_etcd-server-ssl-foo._tcp.[YOUR_HOST]` and `_etcd-server-foo._tcp.[YOUR_HOST]`.
  - Useful for operating multiple etcd clusters under the same domain.

### Added(API)

- Add [`snapshot`](https://github.com/coreos/etcd/pull/9118) package for snapshot restore/save operations.
- Add [`watch_id` field to `etcdserverpb.WatchCreateRequest`](https://github.com/coreos/etcd/pull/9065), allow user-provided watch ID to `mvcc`.
  - Corresponding `watch_id` is returned via `etcdserverpb.WatchResponse`, if any.
- Add [`raftAppliedIndex` field to `etcdserverpb.StatusResponse`](https://github.com/coreos/etcd/pull/9176) for current Raft applied index.
- Add [`errors` field to `etcdserverpb.StatusResponse`](https://github.com/coreos/etcd/pull/9206) for server-side error.
  - e.g. `"etcdserver: no leader", "NOSPACE", "CORRUPT"`
- Add [`dbSizeInUse` field to `etcdserverpb.StatusResponse`](https://github.com/coreos/etcd/pull/9256) for actual DB size after compaction.
  - Also exposed as metric `etcd_debugging_mvcc_db_total_size_in_use_in_bytes`

### Added(v3 `etcdctl`)

- Add [`check datascale`](https://github.com/coreos/etcd/pull/9185) command.
- Add ["raft applied index" to `endpoint status`](https://github.com/coreos/etcd/pull/9176).
- Add ["errors" to `endpoint status`](https://github.com/coreos/etcd/pull/9206).

### Package `raft`

- Improve [Raft `becomeLeader` and `stepLeader`](https://github.com/coreos/etcd/pull/9073) by keeping track of latest `pb.EntryConfChange` index.
  - Previously record `pendingConf` boolean field scanning the entire tail of the log, which can delay hearbeat send.
- Add [`raft.ErrProposalDropped`](https://github.com/coreos/etcd/pull/9067).
  - Now `(r *raft) Step` returns `raft.ErrProposalDropped` if a proposal has been ignored.
  - e.g. a node is removed from cluster, [ongoing leadership transfer](https://github.com/coreos/etcd/issues/8975), etc.
- Fix [missing learner nodes on `(n *node) ApplyConfChange`](https://github.com/coreos/etcd/pull/9116).

