

Previous change logs can be found at [CHANGELOG-3.0](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.0.md).


The minimum recommended etcd versions to run in **production** are 3.1.11+, 3.2.26+, and 3.3.11+.


<hr>

## [v3.1.21](https://github.com/etcd-io/etcd/releases/tag/v3.1.21) (2019-TBD)

### etcdctl v3

- [Strip out insecure endpoints from DNS SRV records when using discovery](https://github.com/etcd-io/etcd/pull/10443) with etcdctl v2
- Add [`etcdctl endpoint health --write-out` support](https://github.com/etcd-io/etcd/pull/9540).
  - Previously, [`etcdctl endpoint health --write-out json` did not work](https://github.com/etcd-io/etcd/issues/9532).
  - The command output is changed. Previously, if endpoint is unreachable, the command output is
  "\<endpoint\> is unhealthy: failed to connect: \<error message\>". This change unified the error message, all error types
  now have the same output "\<endpoint\> is unhealthy: failed to commit proposal: \<error message\>".

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Fix bug where [db_compaction_total_duration_milliseconds metric incorrectly measured duration as 0](https://github.com/etcd-io/etcd/pull/10646).

<hr>

## [v3.1.20](https://github.com/etcd-io/etcd/releases/tag/v3.1.20) (2018-10-10)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.19...v3.1.20) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Improved

- Improve ["became inactive" warning log](https://github.com/etcd-io/etcd/pull/10024), which indicates message send to a peer failed.
- Improve [read index wait timeout warning log](https://github.com/etcd-io/etcd/pull/10026), which indicates that local node might have slow network.
- Add [gRPC interceptor for debugging logs](https://github.com/etcd-io/etcd/pull/9990); enable `etcd --debug` flag to see per-request debug information.
- Add [consistency check in snapshot status](https://github.com/etcd-io/etcd/pull/10109). If consistency check on snapshot file fails, `snapshot status` returns `"snapshot file integrity check failed..."` error.

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Improve [`etcd_network_peer_round_trip_time_seconds`](https://github.com/etcd-io/etcd/pull/10155) Prometheus metric to track leader heartbeats.
  - Previously, it only samples the TCP connection for snapshot messages.
- Display all registered [gRPC metrics at start](https://github.com/etcd-io/etcd/pull/10034).
- Add [`etcd_snap_db_fsync_duration_seconds_count`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_snap_db_save_total_duration_seconds_bucket`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_send_success`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_send_failures`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_send_total_duration_seconds`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_receive_success`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_receive_failures`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_network_snapshot_receive_total_duration_seconds`](https://github.com/etcd-io/etcd/pull/9997) Prometheus metric.
- Add [`etcd_server_id`](https://github.com/etcd-io/etcd/pull/9998) Prometheus metric.
- Add [`etcd_server_health_success`](https://github.com/etcd-io/etcd/pull/10156) Prometheus metric.
- Add [`etcd_server_health_failures`](https://github.com/etcd-io/etcd/pull/10156) Prometheus metric.
- Add [`etcd_server_read_indexes_failed_total`](https://github.com/etcd-io/etcd/pull/10094) Prometheus metric.

### client v3

- Fix logic on [release lock key if cancelled](https://github.com/etcd-io/etcd/pull/10153) in `clientv3/concurrency` package.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.19](https://github.com/etcd-io/etcd/releases/tag/v3.1.19) (2018-07-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.18...v3.1.19) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Improved

- Improve [Raft Read Index timeout warning messages](https://github.com/etcd-io/etcd/pull/9897).

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_go_version`](https://github.com/etcd-io/etcd/pull/9957) Prometheus metric.
- Add [`etcd_server_slow_read_indexes_total`](https://github.com/etcd-io/etcd/pull/9897) Prometheus metric.
- Add [`etcd_server_quota_backend_bytes`](https://github.com/etcd-io/etcd/pull/9820) Prometheus metric.
  - Use it with `etcd_mvcc_db_total_size_in_bytes` and `etcd_mvcc_db_total_size_in_use_in_bytes`.
  - `etcd_server_quota_backend_bytes 2.147483648e+09` means current quota size is 2 GB.
  - `etcd_mvcc_db_total_size_in_bytes 20480` means current physically allocated DB size is 20 KB.
  - `etcd_mvcc_db_total_size_in_use_in_bytes 16384` means future DB size if defragment operation is complete.
  - `etcd_mvcc_db_total_size_in_bytes - etcd_mvcc_db_total_size_in_use_in_bytes` is the number of bytes that can be saved on disk with defragment operation.
- Add [`etcd_mvcc_db_total_size_in_bytes`](https://github.com/etcd-io/etcd/pull/9819) Prometheus metric.
  - In addition to [`etcd_debugging_mvcc_db_total_size_in_bytes`](https://github.com/etcd-io/etcd/pull/9819).
- Add [`etcd_mvcc_db_total_size_in_use_in_bytes`](https://github.com/etcd-io/etcd/pull/9256) Prometheus metric.
  - Use it with `etcd_mvcc_db_total_size_in_bytes` and `etcd_mvcc_db_total_size_in_use_in_bytes`.
  - `etcd_server_quota_backend_bytes 2.147483648e+09` means current quota size is 2 GB.
  - `etcd_mvcc_db_total_size_in_bytes 20480` means current physically allocated DB size is 20 KB.
  - `etcd_mvcc_db_total_size_in_use_in_bytes 16384` means future DB size if defragment operation is complete.
  - `etcd_mvcc_db_total_size_in_bytes - etcd_mvcc_db_total_size_in_use_in_bytes` is the number of bytes that can be saved on disk with defragment operation.

### client v3

- Fix [lease keepalive interval updates when response queue is full](https://github.com/etcd-io/etcd/pull/9952).
  - If `<-chan *clientv3LeaseKeepAliveResponse` from `clientv3.Lease.KeepAlive` was never consumed or channel is full, client was [sending keepalive request every 500ms](https://github.com/etcd-io/etcd/issues/9911) instead of expected rate of every "TTL / 3" duration.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.18](https://github.com/etcd-io/etcd/releases/tag/v3.1.18) (2018-06-15)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.17...v3.1.18) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_version`](https://github.com/etcd-io/etcd/pull/8960) Prometheus metric.
  - To replace [Kubernetes `etcd-version-monitor`](https://github.com/etcd-io/etcd/issues/8948).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.17](https://github.com/etcd-io/etcd/releases/tag/v3.1.17) (2018-06-06)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.16...v3.1.17) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Fix [v3 snapshot recovery](https://github.com/etcd-io/etcd/issues/7628).
  - A follower receives a leader snapshot to be persisted as a `[SNAPSHOT-INDEX].snap.db` file on disk.
  - Now, server [ensures that the incoming snapshot be persisted on disk before loading it](https://github.com/etcd-io/etcd/pull/7876).
  - Otherwise, index mismatch happens and triggers server-side panic (e.g. newer WAL entry with outdated snapshot index).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.16](https://github.com/etcd-io/etcd/releases/tag/v3.1.16) (2018-05-31)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.15...v3.1.16) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Fix [`mvcc` server panic from restore operation](https://github.com/etcd-io/etcd/pull/9775).
  - Let's assume that a watcher had been requested with a future revision X and sent to node A that became network-partitioned thereafter. Meanwhile, cluster makes progress. Then when the partition gets removed, the leader sends a snapshot to node A. Previously if the snapshot's latest revision is still lower than the watch revision X,  **etcd server panicked** during snapshot restore operation.
  - Now, this server-side panic has been fixed.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.15](https://github.com/etcd-io/etcd/releases/tag/v3.1.15) (2018-05-09)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.14...v3.1.15) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Purge old [`*.snap.db` snapshot files](https://github.com/etcd-io/etcd/pull/7967).
  - Previously, etcd did not respect `--max-snapshots` flag to purge old `*.snap.db` files.
  - Now, etcd purges old `*.snap.db` files to keep maximum `--max-snapshots` number of files on disk.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.14](https://github.com/etcd-io/etcd/releases/tag/v3.1.14) (2018-04-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.13...v3.1.14) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_is_leader`](https://github.com/etcd-io/etcd/pull/9587) Prometheus metric.

### etcd server

- Add [`--initial-election-tick-advance`](https://github.com/etcd-io/etcd/pull/9591) flag to configure initial election tick fast-forward.
  - By default, `--initial-election-tick-advance=true`, then local member fast-forwards election ticks to speed up "initial" leader election trigger.
  - This benefits the case of larger election ticks. For instance, cross datacenter deployment may require longer election timeout of 10-second. If true, local node does not need wait up to 10-second. Instead, forwards its election ticks to 8-second, and have only 2-second left before leader election.
  - Major assumptions are that: cluster has no active leader thus advancing ticks enables faster leader election. Or cluster already has an established leader, and rejoining follower is likely to receive heartbeats from the leader after tick advance and before election timeout.
  - However, when network from leader to rejoining follower is congested, and the follower does not receive leader heartbeat within left election ticks, disruptive election has to happen thus affecting cluster availabilities.
  - Now, this can be disabled by setting `--initial-election-tick-advance=false`.
  - Disabling this would slow down initial bootstrap process for cross datacenter deployments. Make tradeoffs by configuring `--initial-election-tick-advance` at the cost of slow initial bootstrap.
  - If single-node, it advances ticks regardless.
  - Address [disruptive rejoining follower node](https://github.com/etcd-io/etcd/issues/9333).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.13](https://github.com/etcd-io/etcd/releases/tag/v3.1.13) (2018-03-29)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.12...v3.1.13) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Improved

- Adjust [election timeout on server restart](https://github.com/etcd-io/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/etcd-io/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.

### Metrics, Monitoring

See [List of metrics](https://github.com/etcd-io/etcd/tree/master/Documentation/metrics) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add missing [`etcd_network_peer_sent_failures_total` count](https://github.com/etcd-io/etcd/pull/9437).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.12](https://github.com/etcd-io/etcd/releases/tag/v3.1.12) (2018-03-08)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.11...v3.1.12) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Fix [`mvcc` "unsynced" watcher restore operation](https://github.com/etcd-io/etcd/pull/9297).
  - "unsynced" watcher is watcher that needs to be in sync with events that have happened.
  - That is, "unsynced" watcher is the slow watcher that was requested on old revision.
  - "unsynced" watcher restore operation was not correctly populating its underlying watcher group.
  - Which possibly causes [missing events from "unsynced" watchers](https://github.com/etcd-io/etcd/issues/9086).
  - A node gets network partitioned with a watcher on a future revision, and falls behind receiving a leader snapshot after partition gets removed. When applying this snapshot, etcd watch storage moves current synced watchers to unsynced since sync watchers might have become stale during network partition. And reset synced watcher group to restart watcher routines. Previously, there was a bug when moving from synced watcher group to unsynced, thus client would miss events when the watcher was requested to the network-partitioned node.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.11](https://github.com/etcd-io/etcd/releases/tag/v3.1.11) (2017-11-28)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.10...v3.1.11) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- [#8411](https://github.com/etcd-io/etcd/issues/8411),[#8806](https://github.com/etcd-io/etcd/pull/8806) backport "mvcc: sending events after restore"
- [#8009](https://github.com/etcd-io/etcd/issues/8009),[#8902](https://github.com/etcd-io/etcd/pull/8902) backport coreos/bbolt v1.3.1-coreos.5

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.1.10](https://github.com/etcd-io/etcd/releases/tag/v3.1.10) (2017-07-14)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.9...v3.1.10) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Added

- Tag docker images with minor versions.
  - e.g. `docker pull quay.io/coreos/etcd:v3.1` to fetch latest v3.1 versions.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).
  - Fix panic on `net/http.CloseNotify`


<hr>


## [v3.1.9](https://github.com/etcd-io/etcd/releases/tag/v3.1.9) (2017-06-09)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.8...v3.1.9) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Allow v2 snapshot over 512MB.

### Go

- Compile with [*Go 1.7.6*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.8](https://github.com/etcd-io/etcd/releases/tag/v3.1.8) (2017-05-19)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.7...v3.1.8) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.7](https://github.com/etcd-io/etcd/releases/tag/v3.1.7) (2017-04-28)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.6...v3.1.7) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.6](https://github.com/etcd-io/etcd/releases/tag/v3.1.6) (2017-04-19)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.5...v3.1.6) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Fill in Auth API response header.
- Remove auth check in Status API.

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.5](https://github.com/etcd-io/etcd/releases/tag/v3.1.5) (2017-03-27)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.4...v3.1.5) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd server

- Fix raft memory leak issue.
- Fix Windows file path issues.

### Other

- Add `/etc/nsswitch.conf` file to alpine-based Docker image.

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.4](https://github.com/etcd-io/etcd/releases/tag/v3.1.4) (2017-03-22)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.3...v3.1.4) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.3](https://github.com/etcd-io/etcd/releases/tag/v3.1.3) (2017-03-10)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.2...v3.1.3) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd gateway

- Fix `etcd gateway` schema handling in DNS discovery.
- Fix sd_notify behaviors in `gateway`, `grpc-proxy`.

### gRPC Proxy

- Fix sd_notify behaviors in `gateway`, `grpc-proxy`.

### Other

- Use machine default host when advertise URLs are default values(`localhost:2379,2380`) AND if listen URL is `0.0.0.0`.

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.2](https://github.com/etcd-io/etcd/releases/tag/v3.1.2) (2017-02-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.1...v3.1.2) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### etcd gateway

- Fix `etcd gateway` with multiple endpoints.

### Other

- Use IPv4 default host, by default (when IPv4 and IPv6 are available).

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.1](https://github.com/etcd-io/etcd/releases/tag/v3.1.1) (2017-02-17)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.0...v3.1.1) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Go

- Compile with [*Go 1.7.5*](https://golang.org/doc/devel/release.html#go1.7).


<hr>


## [v3.1.0](https://github.com/etcd-io/etcd/releases/tag/v3.1.0) (2017-01-20)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.0.0...v3.1.0) and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/) for any breaking changes.

**Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.1 upgrade guide](https://etcd.io/docs/latest/upgrades/upgrade_3_1/).**

### Improved

- Faster linearizable reads (implements Raft [read-index](https://github.com/etcd-io/etcd/pull/6212)).
- v3 authentication API is now stable.

### Breaking Changes

- Deprecated following gRPC metrics in favor of [go-grpc-prometheus](https://github.com/grpc-ecosystem/go-grpc-prometheus).
  - `etcd_grpc_requests_total`
  - `etcd_grpc_requests_failed_total`
  - `etcd_grpc_active_streams`
  - `etcd_grpc_unary_requests_duration_seconds`

### Dependency

- Upgrade [`github.com/ugorji/go/codec`](https://github.com/ugorji/go) to [**`ugorji/go@9c7f9b7`**](https://github.com/ugorji/go/commit/9c7f9b7a2bc3a520f7c7b30b34b7f85f47fe27b6), and [regenerate v2 `client`](https://github.com/etcd-io/etcd/pull/6945).

### Security, Authentication

See [security doc](https://etcd.io/docs/latest/op-guide/security/) for more details.

- SRV records (e.g., infra1.example.com) must match the discovery domain (i.e., example.com) if no custom certificate authority is given.
  - `TLSConfig.ServerName` is ignored with user-provided certificates for backwards compatibility; to be deprecated.
  - For example, `etcd --discovery-srv=example.com` will only authenticate peers/clients when the provided certs have root domain `example.com` as an entry in Subject Alternative Name (SAN) field.

### etcd server

- Automatic leadership transfer when leader steps down.
- etcd flags
  - `--strict-reconfig-check` flag is set by default.
  - Add `--log-output` flag.
  - Add `--metrics` flag.
- etcd uses default route IP if advertise URL is not given.
- Cluster rejects removing members if quorum will be lost.
- Discovery now has upper limit for waiting on retries.
- Warn on binding listeners through domain names; to be deprecated.
- v3.0 and v3.1 with `--auto-compaction-retention=10` run periodic compaction on v3 key-value store for every 10-hour.
  - Compactor only supports periodic compaction.
  - Compactor records latest revisions every 5-minute, until it reaches the first compaction period (e.g. 10-hour).
  - In order to retain key-value history of last compaction period, it uses the last revision that was fetched before compaction period, from the revision records that were collected every 5-minute.
  - When `--auto-compaction-retention=10`, compactor uses revision 100 for compact revision where revision 100 is the latest revision fetched from 10 hours ago.
  - If compaction succeeds or requested revision has already been compacted, it resets period timer and starts over with new historical revision records (e.g. restart revision collect and compact for the next 10-hour period).
  - If compaction fails, it retries in 5 minutes.

### client v3

- Add `SetEndpoints` method; update endpoints at runtime.
- Add `Sync` method; auto-update endpoints at runtime.
- Add `Lease TimeToLive` API; fetch lease information.
- replace Config.Logger field with global logger.
- Get API responses are sorted in ascending order by default.

### etcdctl v3

- Add `lease timetolive` command.
- Add `--print-value-only` flag to get command.
- Add `--dest-prefix` flag to make-mirror command.
- `get` command responses are sorted in ascending order by default.

### gRPC Proxy

- Experimental gRPC proxy feature.

### Other

- `recipes` now conform to sessions defined in `clientv3/concurrency`.
- ACI has symlinks to `/usr/local/bin/etcd*`.

### Go

- Compile with [*Go 1.7.4*](https://golang.org/doc/devel/release.html#go1.7).


<hr>

