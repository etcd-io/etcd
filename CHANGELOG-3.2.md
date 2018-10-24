

Previous change logs can be found at [CHANGELOG-3.1](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.1.md).


The [minimum recommended etcd versions to run in **production**](https://groups.google.com/d/msg/etcd-dev/nZQl17RjxHQ/FkC_rZ_4AwAJT) is 3.1.11+, 3.2.10+, and 3.3.0+.


<hr>


## [v3.2.25](https://github.com/etcd-io/etcd/releases/tag/v3.2.25) (2018-10-10)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.24...v3.2.25) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Improve ["became inactive" warning log](https://github.com/etcd-io/etcd/pull/10024), which indicates message send to a peer failed.
- Improve [read index wait timeout warning log](https://github.com/etcd-io/etcd/pull/10026), which indicates that local node might have slow network.
- Add [gRPC interceptor for debugging logs](https://github.com/etcd-io/etcd/pull/9990); enable `etcd --debug` flag to see per-request debug information.
- Add [consistency check in snapshot status](https://github.com/etcd-io/etcd/pull/10109). If consistency check on snapshot file fails, `snapshot status` returns `"snapshot file integrity check failed..."` error.

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Improve [`etcd_network_peer_round_trip_time_seconds`](https://github.com/etcd-io/etcd/pull/10155) Prometheus metric to track leader heartbeats.
  - Previously, it only samples the TCP connection for snapshot messages.
- Display all registered [gRPC metrics at start](https://github.com/etcd-io/etcd/pull/10032).
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


## [v3.2.24](https://github.com/etcd-io/etcd/releases/tag/v3.2.24) (2018-07-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.23...v3.2.24) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Improve [Raft Read Index timeout warning messages](https://github.com/etcd-io/etcd/pull/9897).

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_go_version`](https://github.com/etcd-io/etcd/pull/9957) Prometheus metric.
- Add [`etcd_server_heartbeat_send_failures_total`](https://github.com/etcd-io/etcd/pull/9942) Prometheus metric.
- Add [`etcd_server_slow_apply_total`](https://github.com/etcd-io/etcd/pull/9942) Prometheus metric.
- Add [`etcd_disk_backend_defrag_duration_seconds`](https://github.com/etcd-io/etcd/pull/9942) Prometheus metric.
- Add [`etcd_mvcc_hash_duration_seconds`](https://github.com/etcd-io/etcd/pull/9942) Prometheus metric.
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
  - Use it with `etcd_mvcc_db_total_size_in_bytes` and `etcd_server_quota_backend_bytes`.
  - `etcd_server_quota_backend_bytes 2.147483648e+09` means current quota size is 2 GB.
  - `etcd_mvcc_db_total_size_in_bytes 20480` means current physically allocated DB size is 20 KB.
  - `etcd_mvcc_db_total_size_in_use_in_bytes 16384` means future DB size if defragment operation is complete.
  - `etcd_mvcc_db_total_size_in_bytes - etcd_mvcc_db_total_size_in_use_in_bytes` is the number of bytes that can be saved on disk with defragment operation.

### gRPC Proxy

- Add [flags for specifying TLS for connecting to proxy](https://github.com/etcd-io/etcd/pull/9894):
  - Add `grpc-proxy start --cert-file`, `grpc-proxy start --key-file` and `grpc-proxy start --trusted-ca-file` flags.
- Add [`grpc-proxy start --metrics-addr` flag for specifying a separate metrics listen address](https://github.com/etcd-io/etcd/pull/9894).

### client v3

- Fix [lease keepalive interval updates when response queue is full](https://github.com/etcd-io/etcd/pull/9952).
  - If `<-chan *clientv3LeaseKeepAliveResponse` from `clientv3.Lease.KeepAlive` was never consumed or channel is full, client was [sending keepalive request every 500ms](https://github.com/etcd-io/etcd/issues/9911) instead of expected rate of every "TTL / 3" duration.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.23](https://github.com/etcd-io/etcd/releases/tag/v3.2.23) (2018-06-15)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.22...v3.2.23) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Improve [slow request apply warning log](https://github.com/etcd-io/etcd/pull/9288).
  - e.g. `read-only range request "key:\"/a\" range_end:\"/b\" " with result "range_response_count:3 size:96" took too long (97.966µs) to execute`.
  - Redact [request value field](https://github.com/etcd-io/etcd/pull/9822).
  - Provide [response size](https://github.com/etcd-io/etcd/pull/9826).
- Add [backoff on watch retries on transient errors](https://github.com/etcd-io/etcd/pull/9840).

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_version`](https://github.com/etcd-io/etcd/pull/8960) Prometheus metric.
  - To replace [Kubernetes `etcd-version-monitor`](https://github.com/etcd-io/etcd/issues/8948).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.22](https://github.com/etcd-io/etcd/releases/tag/v3.2.22) (2018-06-06)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.21...v3.2.22) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Security, Authentication

- Support TLS cipher suite whitelisting.
  - To block [weak cipher suites](https://github.com/etcd-io/etcd/issues/8320).
  - TLS handshake fails when client hello is requested with invalid cipher suites.
  - Add [`etcd --cipher-suites`](https://github.com/etcd-io/etcd/pull/9801) flag.
  - If empty, Go auto-populates the list.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.21](https://github.com/etcd-io/etcd/releases/tag/v3.2.21) (2018-05-31)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.20...v3.2.21) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Fix [auth storage panic when simple token provider is disabled](https://github.com/etcd-io/etcd/pull/8695).
- Fix [`mvcc` server panic from restore operation](https://github.com/etcd-io/etcd/pull/9775).
  - Let's assume that a watcher had been requested with a future revision X and sent to node A that became network-partitioned thereafter. Meanwhile, cluster makes progress. Then when the partition gets removed, the leader sends a snapshot to node A. Previously if the snapshot's latest revision is still lower than the watch revision X,  **etcd server panicked** during snapshot restore operation.
  - Now, this server-side panic has been fixed.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.20](https://github.com/etcd-io/etcd/releases/tag/v3.2.20) (2018-05-09)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.19...v3.2.20) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Purge old [`*.snap.db` snapshot files](https://github.com/etcd-io/etcd/pull/7967).
  - Previously, etcd did not respect `--max-snapshots` flag to purge old `*.snap.db` files.
  - Now, etcd purges old `*.snap.db` files to keep maximum `--max-snapshots` number of files on disk.

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.19](https://github.com/etcd-io/etcd/releases/tag/v3.2.19) (2018-04-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.18...v3.2.19) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Fix [`etcd_debugging_server_lease_expired_total`](https://github.com/etcd-io/etcd/pull/9557) Prometheus metric.
- Fix [race conditions in v2 server stat collecting](https://github.com/etcd-io/etcd/pull/9562).
- Add [`etcd_server_is_leader`](https://github.com/etcd-io/etcd/pull/9587) Prometheus metric.

### Security, Authentication

- Fix [TLS reload](https://github.com/etcd-io/etcd/pull/9570) when [certificate SAN field only includes IP addresses but no domain names](https://github.com/etcd-io/etcd/issues/9541).
  - In Go, server calls `(*tls.Config).GetCertificate` for TLS reload if and only if server's `(*tls.Config).Certificates` field is not empty, or `(*tls.ClientHelloInfo).ServerName` is not empty with a valid SNI from the client. Previously, etcd always populates `(*tls.Config).Certificates` on the initial client TLS handshake, as non-empty. Thus, client was always expected to supply a matching SNI in order to pass the TLS verification and to trigger `(*tls.Config).GetCertificate` to reload TLS assets.
  - However, a certificate whose SAN field does [not include any domain names but only IP addresses](https://github.com/etcd-io/etcd/issues/9541) would request `*tls.ClientHelloInfo` with an empty `ServerName` field, thus failing to trigger the TLS reload on initial TLS handshake; this becomes a problem when expired certificates need to be replaced online.
  - Now, `(*tls.Config).Certificates` is created empty on initial TLS client handshake, first to trigger `(*tls.Config).GetCertificate`, and then to populate rest of the certificates on every new TLS connection, even when client SNI is empty (e.g. cert only includes IPs).

### etcd server

- Add [`etcd --initial-election-tick-advance`](https://github.com/etcd-io/etcd/pull/9591) flag to configure initial election tick fast-forward.
  - By default, `etcd --initial-election-tick-advance=true`, then local member fast-forwards election ticks to speed up "initial" leader election trigger.
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


## [v3.2.18](https://github.com/etcd-io/etcd/releases/tag/v3.2.18) (2018-03-29)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.17...v3.2.18) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Adjust [election timeout on server restart](https://github.com/etcd-io/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/etcd-io/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add missing [`etcd_network_peer_sent_failures_total` count](https://github.com/etcd-io/etcd/pull/9437).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.17](https://github.com/etcd-io/etcd/releases/tag/v3.2.17) (2018-03-08)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.16...v3.2.17) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Fix [server panic on invalid Election Proclaim/Resign HTTP(S) requests](https://github.com/etcd-io/etcd/pull/9379).
  - Previously, wrong-formatted HTTP requests to Election API could trigger panic in etcd server.
  - e.g. `curl -L http://localhost:2379/v3/election/proclaim -X POST -d '{"value":""}'`, `curl -L http://localhost:2379/v3/election/resign -X POST -d '{"value":""}'`.
- Prevent [overflow by large `TTL` values for `Lease` `Grant`](https://github.com/etcd-io/etcd/pull/9399).
  - `TTL` parameter to `Grant` request is unit of second.
  - Leases with too large `TTL` values exceeding `math.MaxInt64` [expire in unexpected ways](https://github.com/etcd-io/etcd/issues/9374).
  - Server now returns `rpctypes.ErrLeaseTTLTooLarge` to client, when the requested `TTL` is larger than *9,000,000,000 seconds* (which is >285 years).
  - Again, etcd `Lease` is meant for short-periodic keepalives or sessions, in the range of seconds or minutes. Not for hours or days!
- Enable etcd server [`raft.Config.CheckQuorum` when starting with `ForceNewCluster`](https://github.com/etcd-io/etcd/pull/9347).

### Proxy v2

- Fix [v2 proxy leaky HTTP requests](https://github.com/etcd-io/etcd/pull/9336).

### Go

- Compile with [*Go 1.8.7*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.16](https://github.com/etcd-io/etcd/releases/tag/v3.2.16) (2018-02-12)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.15...v3.2.16) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Fix [`mvcc` "unsynced" watcher restore operation](https://github.com/etcd-io/etcd/pull/9297).
  - "unsynced" watcher is watcher that needs to be in sync with events that have happened.
  - That is, "unsynced" watcher is the slow watcher that was requested on old revision.
  - "unsynced" watcher restore operation was not correctly populating its underlying watcher group.
  - Which possibly causes [missing events from "unsynced" watchers](https://github.com/etcd-io/etcd/issues/9086).
  - A node gets network partitioned with a watcher on a future revision, and falls behind receiving a leader snapshot after partition gets removed. When applying this snapshot, etcd watch storage moves current synced watchers to unsynced since sync watchers might have become stale during network partition. And reset synced watcher group to restart watcher routines. Previously, there was a bug when moving from synced watcher group to unsynced, thus client would miss events when the watcher was requested to the network-partitioned node.

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.15](https://github.com/etcd-io/etcd/releases/tag/v3.2.15) (2018-01-22)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.14...v3.2.15) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Prevent [server panic from member update/add](https://github.com/etcd-io/etcd/pull/9174) with [wrong scheme URLs](https://github.com/etcd-io/etcd/issues/9173).
- Log [user context cancel errors on stream APIs in debug level with TLS](https://github.com/etcd-io/etcd/pull/9178).

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.14](https://github.com/etcd-io/etcd/releases/tag/v3.2.14) (2018-01-11)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.13...v3.2.14) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Log [user context cancel errors on stream APIs in debug level](https://github.com/etcd-io/etcd/pull/9105).

### etcd server

- Fix [`mvcc/backend.defragdb` nil-pointer dereference on create bucket failure](https://github.com/etcd-io/etcd/pull/9119).

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.13](https://github.com/etcd-io/etcd/releases/tag/v3.2.13) (2018-01-02)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.12...v3.2.13) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Remove [verbose error messages on stream cancel and gRPC info-level logs](https://github.com/etcd-io/etcd/pull/9080) in server-side.
- Fix [gRPC server panic on `GracefulStop` TLS-enabled server](https://github.com/etcd-io/etcd/pull/8987).

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.12](https://github.com/etcd-io/etcd/releases/tag/v3.2.12) (2017-12-20)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.11...v3.2.12) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Dependency

- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases/tag) from [**`v1.7.4`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.4) to [**`v1.7.5`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5).
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) from [**`v1.3`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3) to [**`v1.3.0`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3.0).

### etcd server

- Fix [error message of `Revision` compactor](https://github.com/etcd-io/etcd/pull/8999) in server-side.

### client v3

- Add [`MaxCallSendMsgSize` and `MaxCallRecvMsgSize`](https://github.com/etcd-io/etcd/pull/9047) fields to [`clientv3.Config`](https://godoc.org/github.com/etcd-io/etcd/clientv3#Config).
  - Fix [exceeded response size limit error in client-side](https://github.com/etcd-io/etcd/issues/9043).
  - Address [kubernetes#51099](https://github.com/kubernetes/kubernetes/issues/51099).
    - In previous versions(v3.2.10, v3.2.11), client response size was limited to only 4 MiB.
  - `MaxCallSendMsgSize` default value is 2 MiB, if not configured.
  - `MaxCallRecvMsgSize` default value is `math.MaxInt32`, if not configured.

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.11](https://github.com/etcd-io/etcd/releases/tag/v3.2.11) (2017-12-05)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.10...v3.2.11) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Dependency

- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases/tag) from [**`v1.7.3`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.3) to [**`v1.7.4`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.4).

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- Log [more details on TLS handshake failures](https://github.com/etcd-io/etcd/pull/8952/files).

### client v3

- Fix racey grpc-go's server handler transport `WriteStatus` call to prevent [TLS-enabled etcd server crash](https://github.com/etcd-io/etcd/issues/8904).
- Add [gRPC RPC failure warnings](https://github.com/etcd-io/etcd/pull/8939) to help debug such issues in the future.

### Documentation

- Remove `--listen-metrics-urls` flag in monitoring document (non-released in `v3.2.x`, planned for `v3.3.x`).

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.10](https://github.com/etcd-io/etcd/releases/tag/v3.2.10) (2017-11-16)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.9...v3.2.10) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Dependency

- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases/tag) from [**`v1.2.1`**](https://github.com/grpc/grpc-go/releases/tag/v1.2.1) to [**`v1.7.3`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.3).
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) from [**`v1.2.0`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.2.0) to [**`v1.3`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3).

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- Revert [discovery SRV auth `ServerName` with `*.{ROOT_DOMAIN}`](https://github.com/etcd-io/etcd/pull/8651) to support non-wildcard subject alternative names in the certs (see [issue #8445](https://github.com/etcd-io/etcd/issues/8445) for more contexts).
  - For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `etcd.local` (**not `*.etcd.local`**) as an entry in Subject Alternative Name (SAN) field.

### etcd server

- Replace backend key-value database `boltdb/bolt` with [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to address [backend database size issue](https://github.com/etcd-io/etcd/issues/8009).

### client v3

- Rewrite balancer to handle [network partitions](https://github.com/etcd-io/etcd/issues/8711).

### Go

- Compile with [*Go 1.8.5*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.9](https://github.com/etcd-io/etcd/releases/tag/v3.2.9) (2017-10-06)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.8...v3.2.9) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- Update `golang.org/x/crypto/bcrypt` (see [golang/crypto@6c586e1](https://github.com/golang/crypto/commit/6c586e17d90a7d08bbbc4069984180dce3b04117)).
- Fix discovery SRV bootstrapping to [authenticate `ServerName` with `*.{ROOT_DOMAIN}`](https://github.com/etcd-io/etcd/pull/8651), in order to support sub-domain wildcard matching (see [issue #8445](https://github.com/etcd-io/etcd/issues/8445) for more contexts).
  - For instance, `etcd --discovery-srv=etcd.local` will only authenticate peers/clients when the provided certs have root domain `*.etcd.local` as an entry in Subject Alternative Name (SAN) field.

### Go

- Compile with [*Go 1.8.4*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.8](https://github.com/etcd-io/etcd/releases/tag/v3.2.8) (2017-09-29)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.7...v3.2.8) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### client v2

- Fix v2 client failover to next endpoint on mutable operation.

### gRPC Proxy

- Handle [`KeysOnly` flag](https://github.com/etcd-io/etcd/pull/8552).

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.7](https://github.com/etcd-io/etcd/releases/tag/v3.2.7) (2017-09-01)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.6...v3.2.7) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Security, Authentication

- Fix [server-side auth so concurrent auth operations do not return old revision error](https://github.com/etcd-io/etcd/pull/8306).

### client v3

- Fix [`concurrency/stm` Put with serializable snapshot](https://github.com/etcd-io/etcd/pull/8439).
  - Use store revision from first fetch to resolve write conflicts instead of modified revision.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.6](https://github.com/etcd-io/etcd/releases/tag/v3.2.6) (2017-08-21)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.5...v3.2.6) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Fix watch restore from snapshot.
- Fix multiple URLs for `--listen-peer-urls` flag.
- Add `--enable-pprof` flag to etcd configuration file format.

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Fix `etcd_debugging_mvcc_keys_total` inconsistency.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.5](https://github.com/etcd-io/etcd/releases/tag/v3.2.5) (2017-08-04)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.4...v3.2.5) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcdctl v3

- Return non-zero exit code on unhealthy `endpoint health`.

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- [Server supports reverse-lookup on wildcard DNS `SAN`](https://github.com/etcd-io/etcd/pull/8281). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server first reverse-lookups the remote IP address to get a list of names mapping to that address (e.g. `nslookup IPADDR`). Then accepts the connection if those names have a matching name with peer cert's DNS names (either by exact or wildcard match). If none is matched, server forward-lookups each DNS entry in peer cert (e.g. look up `example.default.svc` when the entry is `*.example.default.svc`), and accepts connection only when the host's resolved addresses have the matching IP address with the peer's remote IP address. For example, peer B's CSR (with `cfssl`) SAN field is `["*.example.default.svc", "*.example.default.svc.cluster.local"]` when peer B's remote IP address is `10.138.0.2`. When peer B tries to join the cluster, peer A reverse-lookup the IP `10.138.0.2` to get the list of host names. And either exact or wildcard match the host names with peer B's cert DNS names in Subject Alternative Name (SAN) field. If none of reverse/forward lookups worked, it returns an error `"tls: "10.138.0.2" does not match any of DNSNames ["*.example.default.svc","*.example.default.svc.cluster.local"]`. See [issue#8268](https://github.com/etcd-io/etcd/issues/8268) for more detail.

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Fix unreachable `/metrics` endpoint when `--enable-v2=false`.

### gRPC Proxy

- Handle [`PrevKv` flag](https://github.com/etcd-io/etcd/pull/8366).

### Other

- Add container registry `gcr.io/etcd-development/etcd`.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.4](https://github.com/etcd-io/etcd/releases/tag/v3.2.4) (2017-07-19)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.3...v3.2.4) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Do not block on active client stream when stopping server

### gRPC proxy

- Fix gRPC proxy Snapshot RPC error handling

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.3](https://github.com/etcd-io/etcd/releases/tag/v3.2.3) (2017-07-14)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.2...v3.2.3) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### client v3

- Let clients establish unlimited streams

### Other

- Tag docker images with minor versions
  - e.g. `docker pull quay.io/coreos/etcd:v3.2` to fetch latest v3.2 versions

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.2](https://github.com/etcd-io/etcd/releases/tag/v3.2.2) (2017-07-07)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.1...v3.2.2) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Rate-limit lease revoke on expiration.
- Extend leases on promote to avoid queueing effect on lease expiration.

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- [Server accepts connections if IP matches, without checking DNS entries](https://github.com/etcd-io/etcd/pull/8223). For instance, if peer cert contains IP addresses and DNS names in Subject Alternative Name (SAN) field, and the remote IP address matches one of those IP addresses, server just accepts connection without further checking the DNS names. For example, peer B's CSR (with `cfssl`) SAN field is `["invalid.domain", "10.138.0.2"]` when peer B's remote IP address is `10.138.0.2` and `invalid.domain` is a invalid host. When peer B tries to join the cluster, peer A successfully authenticates B, since Subject Alternative Name (SAN) field has a valid matching IP address. See [issue#8206](https://github.com/etcd-io/etcd/issues/8206) for more detail.

### etcd server

- Accept connection with matched IP SAN but no DNS match.
  - Don't check DNS entries in certs if there's a matching IP.

### gRPC gateway

- Use user-provided listen address to connect to gRPC gateway.
  - `net.Listener` rewrites IPv4 0.0.0.0 to IPv6 [::], breaking IPv6 disabled hosts.
  - Only v3.2.0, v3.2.1 are affected.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.1](https://github.com/etcd-io/etcd/releases/tag/v3.2.1) (2017-06-23)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.0...v3.2.1) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### etcd server

- Fix backend database in-memory index corruption issue on restore (only 3.2.0 is affected).

### gRPC gateway

- Fix Txn marshaling.

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Fix backend database size debugging metrics.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>


## [v3.2.0](https://github.com/etcd-io/etcd/releases/tag/v3.2.0) (2017-06-09)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.1.0...v3.2.0) and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.2 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_2.md).**

### Improved

- Improve backend read concurrency.

### Breaking Changes

- Increased [`--snapshot-count` default value from 10,000 to 100,000](https://github.com/etcd-io/etcd/pull/7160).
  - Higher snapshot count means it holds Raft entries in memory for longer before discarding old entries.
  - It is a trade-off between less frequent snapshotting and [higher memory usage](https://github.com/kubernetes/kubernetes/issues/60589#issuecomment-371977156).
  - User lower `--snapshot-count` value for lower memory usage.
  - User higher `--snapshot-count` value for better availabilities of slow followers (less frequent snapshots from leader).
- `clientv3.Lease.TimeToLive` returns `LeaseTimeToLiveResponse.TTL == -1` on lease not found.
- `clientv3.NewFromConfigFile` is moved to `clientv3/yaml.NewConfig`.
- `embed.Etcd.Peers` field is now `[]*peerListener`.
- Rejects domains names for `--listen-peer-urls` and `--listen-client-urls` (3.1 only prints out warnings), since [domain name is invalid for network interface binding](https://github.com/etcd-io/etcd/issues/6336).

### Dependency

- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) from [**`v1.0.4`**](https://github.com/grpc/grpc-go/releases/tag/v1.0.4) to [**`v1.2.1`**](https://github.com/grpc/grpc-go/releases/tag/v1.2.1).
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) to [**`v1.2.0`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.2.0).

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-2) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_disk_backend_snapshot_duration_seconds`](https://github.com/etcd-io/etcd/pull/7892)
- Add `etcd_debugging_server_lease_expired_total` metrics.

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- [TLS certificates get reloaded on every client connection](https://github.com/etcd-io/etcd/pull/7829). This is useful when replacing expiry certs without stopping etcd servers; it can be done by overwriting old certs with new ones. Refreshing certs for every connection should not have too much overhead, but can be improved in the future, with caching layer. Example tests can be found [here](https://github.com/etcd-io/etcd/blob/b041ce5d514a4b4aaeefbffb008f0c7570a18986/integration/v3_grpc_test.go#L1601-L1757).
- [Server denies incoming peer certs with wrong IP `SAN`](https://github.com/etcd-io/etcd/pull/7687). For instance, if peer cert contains any IP addresses in Subject Alternative Name (SAN) field, server authenticates a peer only when the remote IP address matches one of those IP addresses. This is to prevent unauthorized endpoints from joining the cluster.  For example, peer B's CSR (with `cfssl`) SAN field is `["*.example.default.svc", "*.example.default.svc.cluster.local", "10.138.0.27"]` when peer B's actual IP address is `10.138.0.2`, not `10.138.0.27`. When peer B tries to join the cluster, peer A will reject B with the error `x509: certificate is valid for 10.138.0.27, not 10.138.0.2`, because B's remote IP address does not match the one in Subject Alternative Name (SAN) field.
- [Server resolves TLS `DNSNames` when checking `SAN`](https://github.com/etcd-io/etcd/pull/7767). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server authenticates a peer only when forward-lookups (`dig b.com`) on those DNS names have matching IP with the remote IP address.  For example, peer B's CSR (with `cfssl`) SAN field is `["b.com"]` when peer B's remote IP address is `10.138.0.2`. When peer B tries to join the cluster, peer A looks up the incoming host `b.com` to get the list of IP addresses (e.g. `dig b.com`). And rejects B if the list does not contain the IP `10.138.0.2`, with the error `tls: 10.138.0.2 does not match any of DNSNames ["b.com"]`.
- Auth support JWT token.

### etcd server

- RPCs
  - Add Election, Lock service.
- Native client `etcdserver/api/v3client`
  - client "embedded" in the server.
- Logging, monitoring
  - Server warns large snapshot operations.
- Add `etcd --enable-v2` flag to enable v2 API server.
  - `etcd --enable-v2=true` by default.
- Add `etcd --auth-token` flag.
- v3.2 compactor runs [every hour](https://github.com/etcd-io/etcd/pull/7875).
  - Compactor only supports periodic compaction.
  - Compactor continues to record latest revisions every 5-minute.
  - For every hour, it uses the last revision that was fetched before compaction period, from the revision records that were collected every 5-minute.
  - That is, for every hour, compactor discards historical data created before compaction period.
  - The retention window of compaction period moves to next hour.
  - For instance, when hourly writes are 100 and `--auto-compaction-retention=10`, v3.1 compacts revision 1000, 2000, and 3000 for every 10-hour, while v3.2 compacts revision 1000, 1100, and 1200 for every 1-hour.
  - If compaction succeeds or requested revision has already been compacted, it resets period timer and removes used compacted revision from historical revision records (e.g. start next revision collect and compaction from previously collected revisions).
  - If compaction fails, it retries in 5 minutes.
- Allow snapshot over 512MB.

### client v3

- STM prefetching.
- Add namespace feature.
- Add `ErrOldCluster` with server version checking.
- Translate `WithPrefix()` into `WithFromKey()` for empty key.

### etcdctl v3

- Add `check perf` command.
- Add `etcdctl --from-key` flag to role grant-permission command.
- `lock` command takes an optional command to execute.

### gRPC Proxy

- Proxy endpoint discovery.
- Namespaces.
- Coalesce lease requests.

### etcd gateway

- Support [DNS SRV priority](https://github.com/etcd-io/etcd/pull/7882) for [smart proxy routing](https://github.com/etcd-io/etcd/issues/4378).

### Other

- v3 client
  - concurrency package's elections updated to match RPC interfaces.
  - let client dial endpoints not in the balancer.
- Release
  - Annotate acbuild with supports-systemd-notify.
  - Add `nsswitch.conf` to Docker container image.
  - Add ppc64le, arm64(experimental) builds.

### Go

- Compile with [*Go 1.8.3*](https://golang.org/doc/devel/release.html#go1.8).


<hr>

