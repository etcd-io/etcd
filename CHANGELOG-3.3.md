

Previous change logs can be found at [CHANGELOG-3.2](https://github.com/etcd-io/etcd/blob/master/CHANGELOG-3.2.md).


The minimum recommended etcd versions to run in **production** are 3.1.11+, 3.2.26+, and 3.3.11+.


<hr>


## [v3.3.14](https://github.com/etcd-io/etcd/releases/tag/v3.3.14) (2019-TBD)

### etcdctl

- Add [`etcdctl endpoint health --write-out` support](https://github.com/etcd-io/etcd/pull/9540).
  - Previously, [`etcdctl endpoint health --write-out json` did not work](https://github.com/etcd-io/etcd/issues/9532).
  - The command output is changed. Previously, if endpoint is unreachable, the command output is
  "\<endpoint\> is unhealthy: failed to connect: \<error message\>". This change unified the error message, all error types
  now have the same output "\<endpoint\> is unhealthy: failed to commit proposal: \<error message\>".


<hr>


## [v3.3.13](https://github.com/etcd-io/etcd/releases/tag/v3.3.13) (2019-05-02)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.12...v3.3.13) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Improved

- Improve [heartbeat send failure logging](https://github.com/etcd-io/etcd/pull/10663).
- Add [`Verify` function to perform corruption check on WAL contents](https://github.com/etcd-io/etcd/pull/10603).

### Metrics, Monitoring

See [List of metrics](https://etcd.readthedocs.io/en/latest/operate.html#v3-3) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Fix bug where [db_compaction_total_duration_milliseconds metric incorrectly measured duration as 0](https://github.com/etcd-io/etcd/pull/10646).

### client v3

- Fix [`(*Client).Endpoints()` method race condition](https://github.com/etcd-io/etcd/pull/10595).

### Package `wal`

- Add [`Verify` function to perform corruption check on WAL contents](https://github.com/etcd-io/etcd/pull/10603).

### Dependency

- Migrate [`github.com/ugorji/go/codec`](https://github.com/ugorji/go/releases) to [**`github.com/json-iterator/go`**](https://github.com/json-iterator/go) (See [#10667](https://github.com/etcd-io/etcd/pull/10667) for more).
- Migrate [`github.com/ghodss/yaml`](https://github.com/ghodss/yaml/releases) to [**`sigs.k8s.io/yaml`**](https://github.com/kubernetes-sigs/yaml) (See [#10718](https://github.com/etcd-io/etcd/pull/10718) for more).

### Go

- Compile with [*Go 1.10.8*](https://golang.org/doc/devel/release.html#go1.10).


<hr>


## [v3.3.12](https://github.com/etcd-io/etcd/releases/tag/v3.3.12) (2019-02-07)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.11...v3.3.12) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### etcdctl

- [Strip out insecure endpoints from DNS SRV records when using discovery](https://github.com/etcd-io/etcd/pull/10443) with etcdctl v2

### Go

- Compile with [*Go 1.10.8*](https://golang.org/doc/devel/release.html#go1.10).


<hr>


## [v3.3.11](https://github.com/etcd-io/etcd/releases/tag/v3.3.11) (2019-1-11)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.10...v3.3.11) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### gRPC Proxy

- Fix [memory leak in cache layer](https://github.com/etcd-io/etcd/pull/10327).

### Security, Authentication

- Disable [CommonName authentication for gRPC-gateway](https://github.com/etcd-io/etcd/pull/10366) gRPC-gateway proxy requests to etcd server use the etcd client server TLS certificate. If that certificate contains CommonName we do not want to use that for authentication as it could lead to permission escalation.

### Go

- Compile with [*Go 1.10.7*](https://golang.org/doc/devel/release.html#go1.10).


<hr>


## [v3.3.10](https://github.com/etcd-io/etcd/releases/tag/v3.3.10) (2018-10-10)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.9...v3.3.10) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Improved

- Improve ["became inactive" warning log](https://github.com/etcd-io/etcd/pull/10024), which indicates message send to a peer failed.
- Improve [read index wait timeout warning log](https://github.com/etcd-io/etcd/pull/10026), which indicates that local node might have slow network.
- Add [gRPC interceptor for debugging logs](https://github.com/etcd-io/etcd/pull/9990); enable `etcd --debug` flag to see per-request debug information.
- Add [consistency check in snapshot status](https://github.com/etcd-io/etcd/pull/10109). If consistency check on snapshot file fails, `snapshot status` returns `"snapshot file integrity check failed..."` error.

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/v3.3.12/metrics/) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Improve [`etcd_network_peer_round_trip_time_seconds`](https://github.com/etcd-io/etcd/pull/10155) Prometheus metric to track leader heartbeats.
  - Previously, it only samples the TCP connection for snapshot messages.
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

- Compile with [*Go 1.10.4*](https://golang.org/doc/devel/release.html#go1.10).


<hr>


## [v3.3.9](https://github.com/etcd-io/etcd/releases/tag/v3.3.9) (2018-07-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.8...v3.3.9) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Improved

- Improve [Raft Read Index timeout warning messages](https://github.com/etcd-io/etcd/pull/9897).

### Security, Authentication

- Compile with [*Go 1.10.3*](https://golang.org/doc/devel/release.html#go1.10) to support [crypto/x509 "Name Constraints"](https://github.com/etcd-io/etcd/issues/9912).

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/v3.3.12/metrics/) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_go_version`](https://github.com/etcd-io/etcd/pull/9957) Prometheus metric.
- Add [`etcd_server_heartbeat_send_failures_total`](https://github.com/etcd-io/etcd/pull/9940) Prometheus metric.
- Add [`etcd_server_slow_apply_total`](https://github.com/etcd-io/etcd/pull/9940) Prometheus metric.
- Add [`etcd_disk_backend_defrag_duration_seconds`](https://github.com/etcd-io/etcd/pull/9940) Prometheus metric.
- Add [`etcd_mvcc_hash_duration_seconds`](https://github.com/etcd-io/etcd/pull/9940) Prometheus metric.
- Add [`etcd_mvcc_hash_rev_duration_seconds`](https://github.com/etcd-io/etcd/pull/9940) Prometheus metric.
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

- Compile with [*Go 1.10.3*](https://golang.org/doc/devel/release.html#go1.10).


<hr>


## [v3.3.8](https://github.com/etcd-io/etcd/releases/tag/v3.3.8) (2018-06-15)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.7...v3.3.8) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Improved

- Improve [slow request apply warning log](https://github.com/etcd-io/etcd/pull/9288).
  - e.g. `read-only range request "key:\"/a\" range_end:\"/b\" " with result "range_response_count:3 size:96" took too long (97.966µs) to execute`.
  - Redact [request value field](https://github.com/etcd-io/etcd/pull/9822).
  - Provide [response size](https://github.com/etcd-io/etcd/pull/9826).
- Add [backoff on watch retries on transient errors](https://github.com/etcd-io/etcd/pull/9840).

### Go

- Compile with [*Go 1.9.7*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.7](https://github.com/etcd-io/etcd/releases/tag/v3.3.7) (2018-06-06)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.6...v3.3.7) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Security, Authentication

- Support TLS cipher suite whitelisting.
  - To block [weak cipher suites](https://github.com/etcd-io/etcd/issues/8320).
  - TLS handshake fails when client hello is requested with invalid cipher suites.
  - Add [`etcd --cipher-suites`](https://github.com/etcd-io/etcd/pull/9801) flag.
  - If empty, Go auto-populates the list.

### etcdctl v3

- Fix [`etcdctl move-leader` command for TLS-enabled endpoints](https://github.com/etcd-io/etcd/pull/9807).

### Go

- Compile with [*Go 1.9.6*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.6](https://github.com/etcd-io/etcd/releases/tag/v3.3.6) (2018-05-31)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.5...v3.3.6) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### etcd server

- Allow [empty auth token](https://github.com/etcd-io/etcd/pull/9369).
  - Previously, when auth token is an empty string, it returns [`failed to initialize the etcd server: auth: invalid auth options` error](https://github.com/etcd-io/etcd/issues/9349).
- Fix [auth storage panic on server lease revoke routine with JWT token](https://github.com/etcd-io/etcd/issues/9695).
- Fix [`mvcc` server panic from restore operation](https://github.com/etcd-io/etcd/pull/9775).
  - Let's assume that a watcher had been requested with a future revision X and sent to node A that became network-partitioned thereafter. Meanwhile, cluster makes progress. Then when the partition gets removed, the leader sends a snapshot to node A. Previously if the snapshot's latest revision is still lower than the watch revision X,  **etcd server panicked** during snapshot restore operation.
  - Now, this server-side panic has been fixed.

### Go

- Compile with [*Go 1.9.6*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.5](https://github.com/etcd-io/etcd/releases/tag/v3.3.5) (2018-05-09)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.4...v3.3.5) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### etcdctl v3

- Fix [`etcdctl watch [key] [range_end] -- [exec-command…]`](https://github.com/etcd-io/etcd/pull/9688) parsing.
  - Previously,  `ETCDCTL_API=3 ./bin/etcdctl watch foo -- echo watch event received` panicked.

### Go

- Compile with [*Go 1.9.6*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.4](https://github.com/etcd-io/etcd/releases/tag/v3.3.4) (2018-04-24)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.3...v3.3.4) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/v3.3.12/metrics/) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd_server_is_leader`](https://github.com/etcd-io/etcd/pull/9587) Prometheus metric.
- Fix [`etcd_debugging_server_lease_expired_total`](https://github.com/etcd-io/etcd/pull/9557) Prometheus metric.
- Fix [race conditions in v2 server stat collecting](https://github.com/etcd-io/etcd/pull/9562).

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
  - Disabling this would slow down initial bootstrap process for cross datacenter deployments. Make tradeoffs by configuring `etcd --initial-election-tick-advance` at the cost of slow initial bootstrap.
  - If single-node, it advances ticks regardless.
  - Address [disruptive rejoining follower node](https://github.com/etcd-io/etcd/issues/9333).

### Package `embed`

- Add [`embed.Config.InitialElectionTickAdvance`](https://github.com/etcd-io/etcd/pull/9591) to enable/disable initial election tick fast-forward.
  - `embed.NewConfig()` would return `*embed.Config` with `InitialElectionTickAdvance` as true by default.

### Go

- Compile with [*Go 1.9.5*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.3](https://github.com/etcd-io/etcd/releases/tag/v3.3.3) (2018-03-29)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.2...v3.3.3) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Improved

- Adjust [election timeout on server restart](https://github.com/etcd-io/etcd/pull/9415) to reduce [disruptive rejoining servers](https://github.com/etcd-io/etcd/issues/9333).
  - Previously, etcd fast-forwards election ticks on server start, with only one tick left for leader election. This is to speed up start phase, without having to wait until all election ticks elapse. Advancing election ticks is useful for cross datacenter deployments with larger election timeouts. However, it was affecting cluster availability if the last tick elapses before leader contacts the restarted node.
  - Now, when etcd restarts, it adjusts election ticks with more than one tick left, thus more time for leader to prevent disruptive restart.
- Adjust [periodic compaction retention window](https://github.com/etcd-io/etcd/pull/9485).
  - e.g. `etcd --auto-compaction-mode=revision --auto-compaction-retention=1000` automatically `Compact` on `"latest revision" - 1000` every 5-minute (when latest revision is 30000, compact on revision 29000).
  - e.g. Previously, `etcd --auto-compaction-mode=periodic --auto-compaction-retention=72h` automatically `Compact` with 72-hour retention windown for every 7.2-hour. **Now, `Compact` happens, for every 1-hour but still with 72-hour retention window.**
  - e.g. Previously, `etcd --auto-compaction-mode=periodic --auto-compaction-retention=30m` automatically `Compact` with 30-minute retention windown for every 3-minute. **Now, `Compact` happens, for every 30-minute but still with 30-minute retention window.**
  - Periodic compactor keeps recording latest revisions for every compaction period when given period is less than 1-hour, or for every 1-hour when given compaction period is greater than 1-hour (e.g. 1-hour when `etcd --auto-compaction-mode=periodic --auto-compaction-retention=24h`).
  - For every compaction period or 1-hour, compactor uses the last revision that was fetched before compaction period, to discard historical data.
  - The retention window of compaction period moves for every given compaction period or hour.
  - For instance, when hourly writes are 100 and `etcd --auto-compaction-mode=periodic --auto-compaction-retention=24h`, `v3.2.x`, `v3.3.0`, `v3.3.1`, and `v3.3.2` compact revision 2400, 2640, and 2880 for every 2.4-hour, while `v3.3.3` *or later* compacts revision 2400, 2500, 2600 for every 1-hour.
  - Futhermore, when `etcd --auto-compaction-mode=periodic --auto-compaction-retention=30m` and writes per minute are about 1000, `v3.3.0`, `v3.3.1`, and `v3.3.2` compact revision 30000, 33000, and 36000, for every 3-minute, while `v3.3.3` *or later* compacts revision 30000, 60000, and 90000, for every 30-minute.

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/v3.3.12/metrics/) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add missing [`etcd_network_peer_sent_failures_total` count](https://github.com/etcd-io/etcd/pull/9437).

### Go

- Compile with [*Go 1.9.5*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.2](https://github.com/etcd-io/etcd/releases/tag/v3.3.2) (2018-03-08)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.1...v3.3.2) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### etcd server

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

### Proxy v2

- Fix [v2 proxy leaky HTTP requests](https://github.com/etcd-io/etcd/pull/9336).

### Go

- Compile with [*Go 1.9.4*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.1](https://github.com/etcd-io/etcd/releases/tag/v3.3.1) (2018-02-12)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.3.0...v3.3.1) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

### Improved

- Add [warnings on requests taking too long](https://github.com/etcd-io/etcd/pull/9288).
  - e.g. `etcdserver: read-only range request "key:\"\\000\" range_end:\"\\000\" " took too long [3.389041388s] to execute`

### etcd server

- Fix [`mvcc` "unsynced" watcher restore operation](https://github.com/etcd-io/etcd/pull/9281).
  - "unsynced" watcher is watcher that needs to be in sync with events that have happened.
  - That is, "unsynced" watcher is the slow watcher that was requested on old revision.
  - "unsynced" watcher restore operation was not correctly populating its underlying watcher group.
  - Which possibly causes [missing events from "unsynced" watchers](https://github.com/etcd-io/etcd/issues/9086).
  - A node gets network partitioned with a watcher on a future revision, and falls behind receiving a leader snapshot after partition gets removed. When applying this snapshot, etcd watch storage moves current synced watchers to unsynced since sync watchers might have become stale during network partition. And reset synced watcher group to restart watcher routines. Previously, there was a bug when moving from synced watcher group to unsynced, thus client would miss events when the watcher was requested to the network-partitioned node.

### Go

- Compile with [*Go 1.9.4*](https://golang.org/doc/devel/release.html#go1.9).


<hr>


## [v3.3.0](https://github.com/etcd-io/etcd/releases/tag/v3.3.0) (2018-02-01)

See [code changes](https://github.com/etcd-io/etcd/compare/v3.2.0...v3.3.0) and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md) for any breaking changes. **Again, before running upgrades from any previous release, please make sure to read change logs below and [v3.3 upgrade guide](https://github.com/etcd-io/etcd/blob/master/Documentation/upgrades/upgrade_3_3.md).**

- [v3.3.0-rc.4](https://github.com/etcd-io/etcd/releases/tag/v3.3.0-rc.4) (2018-01-22), see [code changes](https://github.com/etcd-io/etcd/compare/v3.3.0-rc.3...v3.3.0-rc.4).
- [v3.3.0-rc.3](https://github.com/etcd-io/etcd/releases/tag/v3.3.0-rc.3) (2018-01-17), see [code changes](https://github.com/etcd-io/etcd/compare/v3.3.0-rc.2...v3.3.0-rc.3).
- [v3.3.0-rc.2](https://github.com/etcd-io/etcd/releases/tag/v3.3.0-rc.2) (2018-01-11), see [code changes](https://github.com/etcd-io/etcd/compare/v3.3.0-rc.1...v3.3.0-rc.2).
- [v3.3.0-rc.1](https://github.com/etcd-io/etcd/releases/tag/v3.3.0-rc.1) (2018-01-02), see [code changes](https://github.com/etcd-io/etcd/compare/v3.3.0-rc.0...v3.3.0-rc.1).
- [v3.3.0-rc.0](https://github.com/etcd-io/etcd/releases/tag/v3.3.0-rc.0) (2017-12-20), see [code changes](https://github.com/etcd-io/etcd/compare/v3.2.0...v3.3.0-rc.0).

### Improved

- Use [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) to replace [`boltdb/bolt`](https://github.com/boltdb/bolt#project-status).
  - Fix [etcd database size grows until `mvcc: database space exceeded`](https://github.com/etcd-io/etcd/issues/8009).
- [Support database size larger than 8GiB](https://github.com/etcd-io/etcd/pull/7525) (8GiB is now a suggested maximum size for normal environments)
- [Reduce memory allocation](https://github.com/etcd-io/etcd/pull/8428) on [Range operations](https://github.com/etcd-io/etcd/pull/8475).
- [Rate limit](https://github.com/etcd-io/etcd/pull/8099) and [randomize](https://github.com/etcd-io/etcd/pull/8101) lease revoke on restart or leader elections.
  - Prevent [spikes in Raft proposal rate](https://github.com/etcd-io/etcd/issues/8096).
- Support `clientv3` balancer failover under [network faults/partitions](https://github.com/etcd-io/etcd/issues/8711).
- Better warning on [mismatched `etcd --initial-cluster`](https://github.com/etcd-io/etcd/pull/8083) flag.
  - etcd compares `etcd --initial-advertise-peer-urls` against corresponding `etcd --initial-cluster` URLs with forward-lookup.
  - If resolved IP addresses of `etcd --initial-advertise-peer-urls` and `etcd --initial-cluster` do not match (e.g. [due to DNS error](https://github.com/etcd-io/etcd/pull/9210)), etcd will exit with errors.
    - v3.2 error: `etcd --initial-cluster must include s1=https://s1.test:2380 given --initial-advertise-peer-urls=https://s1.test:2380`.
    - v3.3 error: `failed to resolve https://s1.test:2380 to match --initial-cluster=s1=https://s1.test:2380 (failed to resolve "https://s1.test:2380" (error ...))`.

### Breaking Changes

- Require [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) [**`v1.7.4`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.4) or [**`v1.7.5`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5).
  - Deprecate [`metadata.Incoming/OutgoingContext`](https://github.com/etcd-io/etcd/pull/7896).
  - Deprecate `grpclog.Logger`, upgrade to [`grpclog.LoggerV2`](https://github.com/etcd-io/etcd/pull/8533).
  - Deprecate [`grpc.ErrClientConnTimeout`](https://github.com/etcd-io/etcd/pull/8505) errors in `clientv3`.
  - Use [`MaxRecvMsgSize` and `MaxSendMsgSize`](https://github.com/etcd-io/etcd/pull/8437) to limit message size, in etcd server.
- Translate [gRPC status error in v3 client `Snapshot` API](https://github.com/etcd-io/etcd/pull/9038).
- v3 `etcdctl` [`lease timetolive LEASE_ID`](https://github.com/etcd-io/etcd/issues/9028) on expired lease now prints [`"lease LEASE_ID already expired"`](https://github.com/etcd-io/etcd/pull/9047).
  - <=3.2 prints `"lease LEASE_ID granted with TTL(0s), remaining(-1s)"`.
- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint `/v3alpha` with [`/v3beta`](https://github.com/etcd-io/etcd/pull/8880).
  - To deprecate [`/v3alpha`](https://github.com/etcd-io/etcd/issues/8125) in v3.4.
  - In v3.3, `curl -L http://localhost:2379/v3alpha/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` still works as a fallback to `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'`, but `curl -L http://localhost:2379/v3alpha/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` won't work in v3.4. Use `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
- Change `etcd --auto-compaction-retention` flag to [accept string values](https://github.com/etcd-io/etcd/pull/8563) with [finer granularity](https://github.com/etcd-io/etcd/issues/8503).
  - Now that `etcd --auto-compaction-retention` accepts string values, etcd configuration YAML file `auto-compaction-retention` field must be changed to `string` type.
  - Previously, `--config-file etcd.config.yaml` can have `auto-compaction-retention: 24` field, now must be `auto-compaction-retention: "24"` or `auto-compaction-retention: "24h"`.
  - If configured as `etcd --auto-compaction-mode periodic --auto-compaction-retention "24h"`, the time duration value for `etcd --auto-compaction-retention` flag must be valid for [`time.ParseDuration`](https://golang.org/pkg/time/#ParseDuration) function in Go.

### Dependency

- Upgrade [`boltdb/bolt`](https://github.com/boltdb/bolt#project-status) from [**`v1.3.0`**](https://github.com/boltdb/bolt/releases/tag/v1.3.0) to [`coreos/bbolt`](https://github.com/coreos/bbolt/releases) [**`v1.3.1-coreos.6`**](https://github.com/coreos/bbolt/releases/tag/v1.3.1-coreos.6).
- Upgrade [`google.golang.org/grpc`](https://github.com/grpc/grpc-go/releases) from [**`v1.2.1`**](https://github.com/grpc/grpc-go/releases/tag/v1.2.1) to [**`v1.7.5`**](https://github.com/grpc/grpc-go/releases/tag/v1.7.5).
- Upgrade [`github.com/ugorji/go/codec`](https://github.com/ugorji/go) to [**`v1.1`**](https://github.com/ugorji/go/releases/tag/v1.1), and [regenerate v2 `client`](https://github.com/etcd-io/etcd/pull/8721).
- Upgrade [`github.com/ugorji/go/codec`](https://github.com/ugorji/go) to [**`ugorji/go@54210f4e0`**](https://github.com/ugorji/go/commit/54210f4e076c57f351166f0ed60e67d3fca57a36), and [regenerate v2 `client`](https://github.com/etcd-io/etcd/pull/8574).
- Upgrade [`github.com/grpc-ecosystem/grpc-gateway`](https://github.com/grpc-ecosystem/grpc-gateway/releases) from [**`v1.2.2`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.2.2) to [**`v1.3.0`**](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v1.3.0).
- Upgrade [`golang.org/x/crypto/bcrypt`](https://github.com/golang/crypto) to [**`golang/crypto@6c586e17d`**](https://github.com/golang/crypto/commit/6c586e17d90a7d08bbbc4069984180dce3b04117).

### Metrics, Monitoring

See [List of metrics](https://etcd.io/docs/v3.3.12/metrics/) for all metrics per release.

Note that any `etcd_debugging_*` metrics are experimental and subject to change.

- Add [`etcd --listen-metrics-urls`](https://github.com/etcd-io/etcd/pull/8242) flag for additional `/metrics` and `/health` endpoints.
  - Useful for [bypassing critical APIs when monitoring etcd](https://github.com/etcd-io/etcd/issues/8060).
- Add [`etcd_server_version`](https://github.com/etcd-io/etcd/pull/8960) Prometheus metric.
  - To replace [Kubernetes `etcd-version-monitor`](https://github.com/etcd-io/etcd/issues/8948).
- Add [`etcd_debugging_mvcc_db_compaction_keys_total`](https://github.com/etcd-io/etcd/pull/8280) Prometheus metric.
- Add [`etcd_debugging_server_lease_expired_total`](https://github.com/etcd-io/etcd/pull/8064) Prometheus metric.
  - To improve [lease revoke monitoring](https://github.com/etcd-io/etcd/issues/8050).
- Document [Prometheus 2.0 rules](https://github.com/etcd-io/etcd/pull/8879).
- Initialize gRPC server [metrics with zero values](https://github.com/etcd-io/etcd/pull/8878).
- Fix [range/put/delete operation metrics](https://github.com/etcd-io/etcd/pull/8054) with transaction.
  - `etcd_debugging_mvcc_range_total`
  - `etcd_debugging_mvcc_put_total`
  - `etcd_debugging_mvcc_delete_total`
  - `etcd_debugging_mvcc_txn_total`
- Fix [`etcd_debugging_mvcc_keys_total`](https://github.com/etcd-io/etcd/pull/8390) on restore.
- Fix [`etcd_debugging_mvcc_db_total_size_in_bytes`](https://github.com/etcd-io/etcd/pull/8120) on restore.
  - Also change to [`prometheus.NewGaugeFunc`](https://github.com/etcd-io/etcd/pull/8150).

### Security, Authentication

See [security doc](https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/security.md) for more details.

- Add [CRL based connection rejection](https://github.com/etcd-io/etcd/pull/8124) to manage [revoked certs](https://github.com/etcd-io/etcd/issues/4034).
- Document [TLS authentication changes](https://github.com/etcd-io/etcd/pull/8895).
  - [Server accepts connections if IP matches, without checking DNS entries](https://github.com/etcd-io/etcd/pull/8223). For instance, if peer cert contains IP addresses and DNS names in Subject Alternative Name (SAN) field, and the remote IP address matches one of those IP addresses, server just accepts connection without further checking the DNS names.
  - [Server supports reverse-lookup on wildcard DNS `SAN`](https://github.com/etcd-io/etcd/pull/8281). For instance, if peer cert contains only DNS names (no IP addresses) in Subject Alternative Name (SAN) field, server first reverse-lookups the remote IP address to get a list of names mapping to that address (e.g. `nslookup IPADDR`). Then accepts the connection if those names have a matching name with peer cert's DNS names (either by exact or wildcard match). If none is matched, server forward-lookups each DNS entry in peer cert (e.g. look up `example.default.svc` when the entry is `*.example.default.svc`), and accepts connection only when the host's resolved addresses have the matching IP address with the peer's remote IP address.
- Add [`etcd --peer-cert-allowed-cn`](https://github.com/etcd-io/etcd/pull/8616) flag.
  - To support [CommonName(CN) based auth](https://github.com/etcd-io/etcd/issues/8262) for inter peer connection.
- [Swap priority](https://github.com/etcd-io/etcd/pull/8594) of cert CommonName(CN) and username + password.
  - To address ["username and password specified in the request should take priority over CN in the cert"](https://github.com/etcd-io/etcd/issues/8584).
- Protect [lease revoke with auth](https://github.com/etcd-io/etcd/pull/8031).
- Provide user's role on [auth permission error](https://github.com/etcd-io/etcd/pull/8164).
- Fix [auth store panic with disabled token](https://github.com/etcd-io/etcd/pull/8695).

### etcd server

- Add [`etcd --experimental-initial-corrupt-check`](https://github.com/etcd-io/etcd/pull/8554) flag to [check cluster database hashes before serving client/peer traffic](https://github.com/etcd-io/etcd/issues/8313).
  - `etcd --experimental-initial-corrupt-check=false` by default.
  - v3.4 will enable `--initial-corrupt-check=true` by default.
- Add [`etcd --experimental-corrupt-check-time`](https://github.com/etcd-io/etcd/pull/8420) flag to [raise corrupt alarm monitoring](https://github.com/etcd-io/etcd/issues/7125).
  - `etcd --experimental-corrupt-check-time=0s` disabled by default.
- Add [`etcd --experimental-enable-v2v3`](https://github.com/etcd-io/etcd/pull/8407) flag to [emulate v2 API with v3](https://github.com/etcd-io/etcd/issues/6925).
  - `etcd --experimental-enable-v2v3=false` by default.
- Add [`etcd --max-txn-ops`](https://github.com/etcd-io/etcd/pull/7976) flag to [configure maximum number operations in transaction](https://github.com/etcd-io/etcd/issues/7826).
- Add [`etcd --max-request-bytes`](https://github.com/etcd-io/etcd/pull/7968) flag to [configure maximum client request size](https://github.com/etcd-io/etcd/issues/7923).
  - If not configured, it defaults to 1.5 MiB.
- Add [`etcd --client-crl-file`, `--peer-crl-file`](https://github.com/etcd-io/etcd/pull/8124) flags for [Certificate revocation list](https://github.com/etcd-io/etcd/issues/4034).
- Add [`etcd --peer-cert-allowed-cn`](https://github.com/etcd-io/etcd/pull/8616) flag to support [CN-based auth for inter-peer connection](https://github.com/etcd-io/etcd/issues/8262).
- Add [`etcd --listen-metrics-urls`](https://github.com/etcd-io/etcd/pull/8242) flag for additional `/metrics` and `/health` endpoints.
  - Support [additional (non) TLS `/metrics` endpoints for a TLS-enabled cluster](https://github.com/etcd-io/etcd/pull/8282).
  - e.g. `etcd --listen-metrics-urls=https://localhost:2378,http://localhost:9379` to serve `/metrics` and `/health` on secure port 2378 and insecure port 9379.
  - Useful for [bypassing critical APIs when monitoring etcd](https://github.com/etcd-io/etcd/issues/8060).
- Add [`etcd --auto-compaction-mode`](https://github.com/etcd-io/etcd/pull/8123) flag to [support revision-based compaction](https://github.com/etcd-io/etcd/issues/8098).
- Change `etcd --auto-compaction-retention` flag to [accept string values](https://github.com/etcd-io/etcd/pull/8563) with [finer granularity](https://github.com/etcd-io/etcd/issues/8503).
  - Now that `etcd --auto-compaction-retention` accepts string values, etcd configuration YAML file `auto-compaction-retention` field must be changed to `string` type.
  - Previously, `etcd --config-file etcd.config.yaml` can have `auto-compaction-retention: 24` field, now must be `auto-compaction-retention: "24"` or `auto-compaction-retention: "24h"`.
  - If configured as `--auto-compaction-mode periodic --auto-compaction-retention "24h"`, the time duration value for `etcd --auto-compaction-retention` flag must be valid for [`time.ParseDuration`](https://golang.org/pkg/time/#ParseDuration) function in Go.
  - e.g. `etcd --auto-compaction-mode=revision --auto-compaction-retention=1000` automatically `Compact` on `"latest revision" - 1000` every 5-minute (when latest revision is 30000, compact on revision 29000).
  - e.g. `etcd --auto-compaction-mode=periodic --auto-compaction-retention=72h` automatically `Compact` with 72-hour retention windown, for every 7.2-hour.
  - e.g. `etcd --auto-compaction-mode=periodic --auto-compaction-retention=30m` automatically `Compact` with 30-minute retention windown, for every 3-minute.
  - Periodic compactor continues to record latest revisions for every 1/10 of given compaction period (e.g. 1-hour when `etcd --auto-compaction-mode=periodic --auto-compaction-retention=10h`).
  - For every 1/10 of given compaction period, compactor uses the last revision that was fetched before compaction period, to discard historical data.
  - The retention window of compaction period moves for every 1/10 of given compaction period.
  - For instance, when hourly writes are 100 and `--auto-compaction-retention=10`, v3.1 compacts revision 1000, 2000, and 3000 for every 10-hour, while v3.2.x, v3.3.0, v3.3.1, and v3.3.2 compact revision 1000, 1100, and 1200 for every 1-hour. Futhermore, when writes per minute are 1000, v3.3.0, v3.3.1, and v3.3.2 with `--auto-compaction-mode=periodic --auto-compaction-retention=30m` compact revision 30000, 33000, and 36000, for every 3-minute with more finer granularity.
  - Whether compaction succeeds or not, this process repeats for every 1/10 of given compaction period. If compaction succeeds, it just removes compacted revision from historical revision records.
- Add [`etcd --grpc-keepalive-min-time`, `etcd --grpc-keepalive-interval`, `etcd --grpc-keepalive-timeout`](https://github.com/etcd-io/etcd/pull/8535) flags to configure server-side keepalive policies.
- Serve [`/health` endpoint as unhealthy](https://github.com/etcd-io/etcd/pull/8272) when [alarm (e.g. `NOSPACE`) is raised or there's no leader](https://github.com/etcd-io/etcd/issues/8207).
  - Define [`etcdhttp.Health`](https://godoc.org/github.com/coreos/etcd/etcdserver/api/etcdhttp#Health) struct with JSON encoder.
  - Note that `"health"` field is [`string` type, not `bool`](https://github.com/etcd-io/etcd/pull/9143).
    - e.g. `{"health":"false"}`, `{"health":"true"}`
  - [Remove `"errors"` field](https://github.com/etcd-io/etcd/pull/9162) since `v3.3.0-rc.3` (did exist only in `v3.3.0-rc.0`, `v3.3.0-rc.1`, `v3.3.0-rc.2`).
- Move [logging setup to embed package](https://github.com/etcd-io/etcd/pull/8810)
  - Disable gRPC server info-level logs by default (can be enabled with `etcd --debug` flag).
- Use [monotonic time in Go 1.9](https://github.com/etcd-io/etcd/pull/8507) for `lease` package.
- Warn on [empty hosts in advertise URLs](https://github.com/etcd-io/etcd/pull/8384).
  - Address [advertise client URLs accepts empty hosts](https://github.com/etcd-io/etcd/issues/8379).
  - etcd v3.4 will exit on this error.
    - e.g. `etcd --advertise-client-urls=http://:2379`.
- Warn on [shadowed environment variables](https://github.com/etcd-io/etcd/pull/8385).
  - Address [error on shadowed environment variables](https://github.com/etcd-io/etcd/issues/8380).
  - etcd v3.4 will exit on this error.

### API

- Support [ranges in transaction comparisons](https://github.com/etcd-io/etcd/pull/8025) for [disconnected linearized reads](https://github.com/etcd-io/etcd/issues/7924).
- Add [nested transactions](https://github.com/etcd-io/etcd/pull/8102) to extend [proxy use cases](https://github.com/etcd-io/etcd/issues/7857).
- Add [lease comparison target in transaction](https://github.com/etcd-io/etcd/pull/8324).
- Add [lease list](https://github.com/etcd-io/etcd/pull/8358).
- Add [hash by revision](https://github.com/etcd-io/etcd/pull/8263) for [better corruption checking against boltdb](https://github.com/etcd-io/etcd/issues/8016).

### client v3

- Add [health balancer](https://github.com/etcd-io/etcd/pull/8545) to fix [watch API hangs](https://github.com/etcd-io/etcd/issues/7247), improve [endpoint switch under network faults](https://github.com/etcd-io/etcd/issues/7941).
- [Refactor balancer](https://github.com/etcd-io/etcd/pull/8840) and add [client-side keepalive pings](https://github.com/etcd-io/etcd/pull/8199) to handle [network partitions](https://github.com/etcd-io/etcd/issues/8711).
- Add [`MaxCallSendMsgSize` and `MaxCallRecvMsgSize`](https://github.com/etcd-io/etcd/pull/9047) fields to [`clientv3.Config`](https://godoc.org/github.com/coreos/etcd/clientv3#Config).
  - Fix [exceeded response size limit error in client-side](https://github.com/etcd-io/etcd/issues/9043).
  - Address [kubernetes#51099](https://github.com/kubernetes/kubernetes/issues/51099).
    - In previous versions(v3.2.10, v3.2.11), client response size was limited to only 4 MiB.
  - `MaxCallSendMsgSize` default value is 2 MiB, if not configured.
  - `MaxCallRecvMsgSize` default value is `math.MaxInt32`, if not configured.
- Accept [`Compare_LEASE`](https://github.com/etcd-io/etcd/pull/8324) in [`clientv3.Compare`](https://godoc.org/github.com/coreos/etcd/clientv3#Compare).
- Add [`LeaseValue` helper](https://github.com/etcd-io/etcd/pull/8488) to `Cmp` `LeaseID` values in `Txn`.
- Add [`MoveLeader`](https://github.com/etcd-io/etcd/pull/8153) to `Maintenance`.
- Add [`HashKV`](https://github.com/etcd-io/etcd/pull/8351) to `Maintenance`.
- Add [`Leases`](https://github.com/etcd-io/etcd/pull/8358) to `Lease`.
- Add [`clientv3/ordering`](https://github.com/etcd-io/etcd/pull/8092) for enforce [ordering in serialized requests](https://github.com/etcd-io/etcd/issues/7623).
- Fix ["put at-most-once" violation](https://github.com/etcd-io/etcd/pull/8335).
- Fix [`WatchResponse.Canceled`](https://github.com/etcd-io/etcd/pull/8283) on [compacted watch request](https://github.com/etcd-io/etcd/issues/8231).
- Fix [`concurrency/stm` `Put` with serializable snapshot](https://github.com/etcd-io/etcd/pull/8439).
  - Use store revision from first fetch to resolve write conflicts instead of modified revision.

### etcdctl v3

- Add [`etcdctl --discovery-srv`](https://github.com/etcd-io/etcd/pull/8462) flag.
- Add [`etcdctl --keepalive-time`, `--keepalive-timeout`](https://github.com/etcd-io/etcd/pull/8663) flags.
- Add [`etcdctl lease list`](https://github.com/etcd-io/etcd/pull/8358) command.
- Add [`etcdctl lease keep-alive --once`](https://github.com/etcd-io/etcd/pull/8775) flag.
- Make [`lease timetolive LEASE_ID`](https://github.com/etcd-io/etcd/issues/9028) on expired lease print [`lease LEASE_ID already expired`](https://github.com/etcd-io/etcd/pull/9047).
  - <=3.2 prints `lease LEASE_ID granted with TTL(0s), remaining(-1s)`.
- Add [`etcdctl snapshot restore --wal-dir`](https://github.com/etcd-io/etcd/pull/9124) flag.
- Add [`etcdctl defrag --data-dir`](https://github.com/etcd-io/etcd/pull/8367) flag.
- Add [`etcdctl move-leader`](https://github.com/etcd-io/etcd/pull/8153) command.
- Add [`etcdctl endpoint hashkv`](https://github.com/etcd-io/etcd/pull/8351) command.
- Add [`etcdctl endpoint --cluster`](https://github.com/etcd-io/etcd/pull/8143) flag, equivalent to [v2 `etcdctl cluster-health`](https://github.com/etcd-io/etcd/issues/8117).
- Make `etcdctl endpoint health` command terminate with [non-zero exit code on unhealthy status](https://github.com/etcd-io/etcd/pull/8342).
- Add [`etcdctl lock --ttl`](https://github.com/etcd-io/etcd/pull/8370) flag.
- Support [`etcdctl watch [key] [range_end] -- [exec-command…]`](https://github.com/etcd-io/etcd/pull/8919), equivalent to [v2 `etcdctl exec-watch`](https://github.com/etcd-io/etcd/issues/8814).
  - Make `etcdctl watch -- [exec-command]` set environmental variables [`ETCD_WATCH_REVISION`, `ETCD_WATCH_EVENT_TYPE`, `ETCD_WATCH_KEY`, `ETCD_WATCH_VALUE`](https://github.com/etcd-io/etcd/pull/9142) for each event.
- Support [`etcdctl watch` with environmental variables `ETCDCTL_WATCH_KEY` and `ETCDCTL_WATCH_RANGE_END`](https://github.com/etcd-io/etcd/pull/9142).
- Enable [`clientv3.WithRequireLeader(context.Context)` for `watch`](https://github.com/etcd-io/etcd/pull/8672) command.
- Print [`"del"` instead of `"delete"`](https://github.com/etcd-io/etcd/pull/8297) in `txn` interactive mode.
- Print [`ETCD_INITIAL_ADVERTISE_PEER_URLS` in `member add`](https://github.com/etcd-io/etcd/pull/8332).

### etcdctl v3

- Handle [empty key permission](https://github.com/etcd-io/etcd/pull/8514) in `etcdctl`.

### etcdctl v2

- Add [`etcdctl backup --with-v3`](https://github.com/etcd-io/etcd/pull/8479) flag.

### gRPC Proxy

- Add [`grpc-proxy start --experimental-leasing-prefix`](https://github.com/etcd-io/etcd/pull/8341) flag.
  - For disconnected linearized reads.
  - Based on [V system leasing](https://github.com/etcd-io/etcd/issues/6065).
  - See ["Disconnected consistent reads with etcd" blog post](https://coreos.com/blog/coreos-labs-disconnected-consistent-reads-with-etcd).
- Add [`grpc-proxy start --experimental-serializable-ordering`](https://github.com/etcd-io/etcd/pull/8315) flag.
  - To ensure serializable reads have monotonically increasing store revisions across endpoints.
- Add [`grpc-proxy start --metrics-addr`](https://github.com/etcd-io/etcd/pull/8242) flag for an additional `/metrics` endpoint.
  - Set `--metrics-addr=http://[HOST]:9379` to serve `/metrics` in insecure port 9379.
- Serve [`/health` endpoint in grpc-proxy](https://github.com/etcd-io/etcd/pull/8322).
- Add [`grpc-proxy start --debug`](https://github.com/etcd-io/etcd/pull/8994) flag.
- Add [`grpc-proxy start --max-send-bytes`](https://github.com/etcd-io/etcd/pull/9250) flag to [configure maximum client request size](https://github.com/etcd-io/etcd/issues/7923).
- Add [`grpc-proxy start --max-recv-bytes`](https://github.com/etcd-io/etcd/pull/9250) flag to [configure maximum client request size](https://github.com/etcd-io/etcd/issues/7923).
- Fix [Snapshot API error handling](https://github.com/etcd-io/etcd/commit/dbd16d52fbf81e5fd806d21ff5e9148d5bf203ab).
- Fix [KV API `PrevKv` flag handling](https://github.com/etcd-io/etcd/pull/8366).
- Fix [KV API `KeysOnly` flag handling](https://github.com/etcd-io/etcd/pull/8552).

### gRPC gateway

- Replace [gRPC gateway](https://github.com/grpc-ecosystem/grpc-gateway) endpoint `/v3alpha` with [`/v3beta`](https://github.com/etcd-io/etcd/pull/8880).
  - To deprecate [`/v3alpha`](https://github.com/etcd-io/etcd/issues/8125) in v3.4.
  - In v3.3, `curl -L http://localhost:2379/v3alpha/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` still works as a fallback to `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'`, but `curl -L http://localhost:2379/v3alpha/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` won't work in v3.4. Use `curl -L http://localhost:2379/v3beta/kv/put -X POST -d '{"key": "Zm9v", "value": "YmFy"}'` instead.
- Support ["authorization" token](https://github.com/etcd-io/etcd/pull/7999).
- Support [websocket for bi-directional streams](https://github.com/etcd-io/etcd/pull/8257).
  - Fix [`Watch` API with gRPC gateway](https://github.com/etcd-io/etcd/issues/8237).
- Upgrade gRPC gateway to [v1.3.0](https://github.com/etcd-io/etcd/issues/8838).

### etcd server

- Fix [backend database in-memory index corruption](https://github.com/etcd-io/etcd/pull/8127) issue on restore (only 3.2.0 is affected).
- Fix [watch restore from snapshot](https://github.com/etcd-io/etcd/pull/8427).
- Fix [`mvcc/backend.defragdb` nil-pointer dereference on create bucket failure](https://github.com/etcd-io/etcd/pull/9119).
- Fix [server crash](https://github.com/etcd-io/etcd/pull/8010) on [invalid transaction request from gRPC gateway](https://github.com/etcd-io/etcd/issues/7889).
- Prevent [server panic from member update/add](https://github.com/etcd-io/etcd/pull/9174) with [wrong scheme URLs](https://github.com/etcd-io/etcd/issues/9173).
- Make [peer dial timeout longer](https://github.com/etcd-io/etcd/pull/8599).
  - See [coreos/etcd-operator#1300](https://github.com/etcd-io/etcd-operator/issues/1300) for more detail.
- Make server [wait up to request time-out](https://github.com/etcd-io/etcd/pull/8267) with [pending RPCs](https://github.com/etcd-io/etcd/issues/8224).
- Fix [`grpc.Server` panic on `GracefulStop`](https://github.com/etcd-io/etcd/pull/8987) with [TLS-enabled server](https://github.com/etcd-io/etcd/issues/8916).
- Fix ["multiple peer URLs cannot start" issue](https://github.com/etcd-io/etcd/issues/8383).
- Fix server-side auth so [concurrent auth operations do not return old revision error](https://github.com/etcd-io/etcd/pull/8442).
- Handle [WAL renaming failure on Windows](https://github.com/etcd-io/etcd/pull/8286).
- Upgrade [`coreos/go-systemd`](https://github.com/coreos/go-systemd/releases) to `v15` (see https://github.com/coreos/go-systemd/releases/tag/v15).
- [Put back `/v2/machines`](https://github.com/etcd-io/etcd/pull/8062) endpoint for python-etcd wrapper.

### client v2

- [Fail-over v2 client](https://github.com/etcd-io/etcd/pull/8519) to next endpoint on [oneshot failure](https://github.com/etcd-io/etcd/issues/8515).

### Package `raft`

- Add [non-voting member](https://github.com/etcd-io/etcd/pull/8751).
  - To implement [Raft thesis 4.2.1 Catching up new servers](https://github.com/etcd-io/etcd/issues/8568).
  - `Learner` node does not vote or promote itself.

### Other

- Support previous two minor versions (see our [new release policy](https://github.com/etcd-io/etcd/pull/8805)).
- `v3.3.x` is the last release cycle that supports `ACI`.
  - [AppC was officially suspended](https://github.com/appc/spec#-disclaimer-), as of late 2016.
  - [`acbuild`](https://github.com/containers/build#this-project-is-currently-unmaintained) is not maintained anymore.
  - `*.aci` files won't be available from etcd v3.4 release.
- Add container registry [`gcr.io/etcd-development/etcd`](https://gcr.io/etcd-development/etcd).
  - [quay.io/coreos/etcd](https://quay.io/coreos/etcd) is still supported as secondary.

### Go

- Require [*Go 1.9+*](https://github.com/etcd-io/etcd/issues/6174).
- Compile with [*Go 1.9.3*](https://golang.org/doc/devel/release.html#go1.9).
- Deprecate [`golang.org/x/net/context`](https://github.com/etcd-io/etcd/pull/8511).


<hr>

