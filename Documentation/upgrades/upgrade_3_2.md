---
title: Upgrade etcd from 3.1 to 3.2
---

In the general case, upgrading from etcd 3.1 to 3.2 can be a zero-downtime, rolling upgrade:
 - one by one, stop the etcd v3.1 processes and replace them with etcd v3.2 processes
 - after running all v3.2 processes, new features in v3.2 are available to the cluster

Before [starting an upgrade](#upgrade-procedure), read through the rest of this guide to prepare.

### Upgrade checklists

**NOTE:** When [migrating from v2 with no v3 data](https://github.com/etcd-io/etcd/issues/9480), etcd server v3.2+ panics when etcd restores from existing snapshots but no v3 `ETCD_DATA_DIR/member/snap/db` file. This happens when the server had migrated from v2 with no previous v3 data. This also prevents accidental v3 data loss (e.g. `db` file might have been moved). etcd requires that post v3 migration can only happen with v3 data. Do not upgrade to newer v3 versions until v3.0 server contains v3 data.

Highlighted breaking changes in 3.2.

#### Changed default `snapshot-count` value

Higher `--snapshot-count` holds more Raft entries in memory until snapshot, thus causing [recurrent higher memory usage](https://github.com/kubernetes/kubernetes/issues/60589#issuecomment-371977156). Since leader retains latest Raft entries for longer, a slow follower has more time to catch up before leader snapshot. `--snapshot-count` is a tradeoff between higher memory usage and better availabilities of slow followers.

Since v3.2, the default value of `--snapshot-count` has [changed from from 10,000 to 100,000](https://github.com/etcd-io/etcd/pull/7160).

#### Changed gRPC dependency (>=3.2.10)

3.2.10 or later now requires [grpc/grpc-go](https://github.com/grpc/grpc-go/releases) `v1.7.5` (<=3.2.9 requires `v1.2.1`).

##### Deprecated `grpclog.Logger`

`grpclog.Logger` has been deprecated in favor of [`grpclog.LoggerV2`](https://github.com/grpc/grpc-go/blob/master/grpclog/loggerv2.go). `clientv3.Logger` is now `grpclog.LoggerV2`.

Before

```go
import "github.com/coreos/etcd/clientv3"
clientv3.SetLogger(log.New(os.Stderr, "grpc: ", 0))
```

After

```go
import "github.com/coreos/etcd/clientv3"
import "google.golang.org/grpc/grpclog"
clientv3.SetLogger(grpclog.NewLoggerV2(os.Stderr, os.Stderr, os.Stderr))

// log.New above cannot be used (not implement grpclog.LoggerV2 interface)
```

##### Deprecated `grpc.ErrClientConnTimeout`

Previously, `grpc.ErrClientConnTimeout` error is returned on client dial time-outs. 3.2 instead returns `context.DeadlineExceeded` (see [#8504](https://github.com/etcd-io/etcd/issues/8504)).

Before

```go
// expect dial time-out on ipv4 blackhole
_, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"http://254.0.0.1:12345"},
    DialTimeout: 2 * time.Second
})
if err == grpc.ErrClientConnTimeout {
	// handle errors
}
```

After

```go
_, err := clientv3.New(clientv3.Config{
    Endpoints:   []string{"http://254.0.0.1:12345"},
    DialTimeout: 2 * time.Second
})
if err == context.DeadlineExceeded {
	// handle errors
}
```

#### Changed maximum request size limits (>=3.2.10)

3.2.10 and 3.2.11 allow custom request size limits in server side. >=3.2.12 allows custom request size limits for both server and **client side**. In previous versions(v3.2.10, v3.2.11), client response size was limited to only 4 MiB.

Server-side request limits can be configured with `--max-request-bytes` flag:

```bash
# limits request size to 1.5 KiB
etcd --max-request-bytes 1536

# client writes exceeding 1.5 KiB will be rejected
etcdctl put foo [LARGE VALUE...]
# etcdserver: request is too large
```

Or configure `embed.Config.MaxRequestBytes` field:

```go
import "github.com/coreos/etcd/embed"
import "github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"

// limit requests to 5 MiB
cfg := embed.NewConfig()
cfg.MaxRequestBytes = 5 * 1024 * 1024

// client writes exceeding 5 MiB will be rejected
_, err := cli.Put(ctx, "foo", [LARGE VALUE...])
err == rpctypes.ErrRequestTooLarge
```

**If not specified, server-side limit defaults to 1.5 MiB**.

Client-side request limits must be configured based on server-side limits.

```bash
# limits request size to 1 MiB
etcd --max-request-bytes 1048576
```

```go
import "github.com/coreos/etcd/clientv3"

cli, _ := clientv3.New(clientv3.Config{
    Endpoints: []string{"127.0.0.1:2379"},
    MaxCallSendMsgSize: 2 * 1024 * 1024,
    MaxCallRecvMsgSize: 3 * 1024 * 1024,
})


// client writes exceeding "--max-request-bytes" will be rejected from etcd server
_, err := cli.Put(ctx, "foo", strings.Repeat("a", 1*1024*1024+5))
err == rpctypes.ErrRequestTooLarge


// client writes exceeding "MaxCallSendMsgSize" will be rejected from client-side
_, err = cli.Put(ctx, "foo", strings.Repeat("a", 5*1024*1024))
err.Error() == "rpc error: code = ResourceExhausted desc = grpc: trying to send message larger than max (5242890 vs. 2097152)"


// some writes under limits
for i := range []int{0,1,2,3,4} {
    _, err = cli.Put(ctx, fmt.Sprintf("foo%d", i), strings.Repeat("a", 1*1024*1024-500))
    if err != nil {
        panic(err)
    }
}
// client reads exceeding "MaxCallRecvMsgSize" will be rejected from client-side
_, err = cli.Get(ctx, "foo", clientv3.WithPrefix())
err.Error() == "rpc error: code = ResourceExhausted desc = grpc: received message larger than max (5240509 vs. 3145728)"
```

**If not specified, client-side send limit defaults to 2 MiB (1.5 MiB + gRPC overhead bytes) and receive limit to `math.MaxInt32`**. Please see [clientv3 godoc](https://godoc.org/github.com/coreos/etcd/clientv3#Config) for more detail.

#### Changed raw gRPC client wrappers

3.2.12 or later changes the function signatures of `clientv3` gRPC client wrapper. This change was needed to support [custom `grpc.CallOption` on message size limits](https://github.com/etcd-io/etcd/pull/9047).

Before and after

```diff
-func NewKVFromKVClient(remote pb.KVClient) KV {
+func NewKVFromKVClient(remote pb.KVClient, c *Client) KV {

-func NewClusterFromClusterClient(remote pb.ClusterClient) Cluster {
+func NewClusterFromClusterClient(remote pb.ClusterClient, c *Client) Cluster {

-func NewLeaseFromLeaseClient(remote pb.LeaseClient, keepAliveTimeout time.Duration) Lease {
+func NewLeaseFromLeaseClient(remote pb.LeaseClient, c *Client, keepAliveTimeout time.Duration) Lease {

-func NewMaintenanceFromMaintenanceClient(remote pb.MaintenanceClient) Maintenance {
+func NewMaintenanceFromMaintenanceClient(remote pb.MaintenanceClient, c *Client) Maintenance {

-func NewWatchFromWatchClient(wc pb.WatchClient) Watcher {
+func NewWatchFromWatchClient(wc pb.WatchClient, c *Client) Watcher {
```

#### Changed `clientv3.Lease.TimeToLive` API

Previously, `clientv3.Lease.TimeToLive` API returned `lease.ErrLeaseNotFound` on non-existent lease ID. 3.2 instead returns TTL=-1 in its response and no error (see [#7305](https://github.com/etcd-io/etcd/pull/7305)).

Before

```go
// when leaseID does not exist
resp, err := TimeToLive(ctx, leaseID)
resp == nil
err == lease.ErrLeaseNotFound
```

After

```go
// when leaseID does not exist
resp, err := TimeToLive(ctx, leaseID)
resp.TTL == -1
err == nil
```

#### Moved `clientv3.NewFromConfigFile` to `clientv3.yaml.NewConfig`

`clientv3.NewFromConfigFile` is moved to `yaml.NewConfig`.

Before

```go
import "github.com/coreos/etcd/clientv3"
clientv3.NewFromConfigFile
```

After

```go
import clientv3yaml "github.com/coreos/etcd/clientv3/yaml"
clientv3yaml.NewConfig
```

#### Change in `--listen-peer-urls` and `--listen-client-urls`

3.2 now rejects domains names for `--listen-peer-urls` and `--listen-client-urls` (3.1 only prints out warnings), since domain name is invalid for network interface binding. Make sure that those URLs are properly formated as `scheme://IP:port`.

See [issue #6336](https://github.com/etcd-io/etcd/issues/6336) for more contexts.

### Server upgrade checklists

#### Upgrade requirements

To upgrade an existing etcd deployment to 3.2, the running cluster must be 3.1 or greater. If it's before 3.1, please [upgrade to 3.1](upgrade_3_1.md) before upgrading to 3.2.

Also, to ensure a smooth rolling upgrade, the running cluster must be healthy. Check the health of the cluster by using the `etcdctl endpoint health` command before proceeding.

#### Preparation

Before upgrading etcd, always test the services relying on etcd in a staging environment before deploying the upgrade to the production environment.

Before beginning, [backup the etcd data](../op-guide/maintenance.md#snapshot-backup). Should something go wrong with the upgrade, it is possible to use this backup to [downgrade](#downgrade) back to existing etcd version. Please note that the `snapshot` command only backs up the v3 data. For v2 data, see [backing up v2 datastore](../v2/admin_guide.md#backing-up-the-datastore).

#### Mixed versions

While upgrading, an etcd cluster supports mixed versions of etcd members, and operates with the protocol of the lowest common version. The cluster is only considered upgraded once all of its members are upgraded to version 3.2. Internally, etcd members negotiate with each other to determine the overall cluster version, which controls the reported version and the supported features.

#### Limitations

Note: If the cluster only has v3 data and no v2 data, it is not subject to this limitation.

If the cluster is serving a v2 data set larger than 50MB, each newly upgraded member may take up to two minutes to catch up with the existing cluster. Check the size of a recent snapshot to estimate the total data size. In other words, it is safest to wait for 2 minutes between upgrading each member.

For a much larger total data size, 100MB or more , this one-time process might take even more time. Administrators of very large etcd clusters of this magnitude can feel free to contact the [etcd team][etcd-contact] before upgrading, and we'll be happy to provide advice on the procedure.

#### Downgrade

If all members have been upgraded to v3.2, the cluster will be upgraded to v3.2, and downgrade from this completed state is **not possible**. If any single member is still v3.1, however, the cluster and its operations remains "v3.1", and it is possible from this mixed cluster state to return to using a v3.1 etcd binary on all members.

Please [backup the data directory](../op-guide/maintenance.md#snapshot-backup) of all etcd members to make downgrading the cluster possible even after it has been completely upgraded.

### Upgrade procedure

This example shows how to upgrade a 3-member v3.1 ectd cluster running on a local machine.

#### 1. Check upgrade requirements

Is the cluster healthy and running v3.1.x?

```
$ ETCDCTL_API=3 etcdctl endpoint health --endpoints=localhost:2379,localhost:22379,localhost:32379
localhost:2379 is healthy: successfully committed proposal: took = 6.600684ms
localhost:22379 is healthy: successfully committed proposal: took = 8.540064ms
localhost:32379 is healthy: successfully committed proposal: took = 8.763432ms

$ curl http://localhost:2379/version
{"etcdserver":"3.1.7","etcdcluster":"3.1.0"}
```

#### 2. Stop the existing etcd process

When each etcd process is stopped, expected errors will be logged by other cluster members. This is normal since a cluster member connection has been (temporarily) broken:

```
2017-04-27 14:13:31.491746 I | raft: c89feb932daef420 [term 3] received MsgTimeoutNow from 6d4f535bae3ab960 and starts an election to get leadership.
2017-04-27 14:13:31.491769 I | raft: c89feb932daef420 became candidate at term 4
2017-04-27 14:13:31.491788 I | raft: c89feb932daef420 received MsgVoteResp from c89feb932daef420 at term 4
2017-04-27 14:13:31.491797 I | raft: c89feb932daef420 [logterm: 3, index: 9] sent MsgVote request to 6d4f535bae3ab960 at term 4
2017-04-27 14:13:31.491805 I | raft: c89feb932daef420 [logterm: 3, index: 9] sent MsgVote request to 9eda174c7df8a033 at term 4
2017-04-27 14:13:31.491815 I | raft: raft.node: c89feb932daef420 lost leader 6d4f535bae3ab960 at term 4
2017-04-27 14:13:31.524084 I | raft: c89feb932daef420 received MsgVoteResp from 6d4f535bae3ab960 at term 4
2017-04-27 14:13:31.524108 I | raft: c89feb932daef420 [quorum:2] has received 2 MsgVoteResp votes and 0 vote rejections
2017-04-27 14:13:31.524123 I | raft: c89feb932daef420 became leader at term 4
2017-04-27 14:13:31.524136 I | raft: raft.node: c89feb932daef420 elected leader c89feb932daef420 at term 4
2017-04-27 14:13:31.592650 W | rafthttp: lost the TCP streaming connection with peer 6d4f535bae3ab960 (stream MsgApp v2 reader)
2017-04-27 14:13:31.592825 W | rafthttp: lost the TCP streaming connection with peer 6d4f535bae3ab960 (stream Message reader)
2017-04-27 14:13:31.693275 E | rafthttp: failed to dial 6d4f535bae3ab960 on stream Message (dial tcp [::1]:2380: getsockopt: connection refused)
2017-04-27 14:13:31.693289 I | rafthttp: peer 6d4f535bae3ab960 became inactive
2017-04-27 14:13:31.936678 W | rafthttp: lost the TCP streaming connection with peer 6d4f535bae3ab960 (stream Message writer)
```

It's a good idea at this point to [backup the etcd data](../op-guide/maintenance.md#snapshot-backup) to provide a downgrade path should any problems occur:

```
$ etcdctl snapshot save backup.db
```

#### 3. Drop-in etcd v3.2 binary and start the new etcd process

The new v3.2 etcd will publish its information to the cluster:

```
2017-04-27 14:14:25.363225 I | etcdserver: published {Name:s1 ClientURLs:[http://localhost:2379]} to cluster a9ededbffcb1b1f1
```

Verify that each member, and then the entire cluster, becomes healthy with the new v3.2 etcd binary:

```
$ ETCDCTL_API=3 /etcdctl endpoint health --endpoints=localhost:2379,localhost:22379,localhost:32379
localhost:22379 is healthy: successfully committed proposal: took = 5.540129ms
localhost:32379 is healthy: successfully committed proposal: took = 7.321771ms
localhost:2379 is healthy: successfully committed proposal: took = 10.629901ms
```

Upgraded members will log warnings like the following until the entire cluster is upgraded. This is expected and will cease after all etcd cluster members are upgraded to v3.2:

```
2017-04-27 14:15:17.071804 W | etcdserver: member c89feb932daef420 has a higher version 3.2.0
2017-04-27 14:15:21.073110 W | etcdserver: the local etcd version 3.1.7 is not up-to-date
2017-04-27 14:15:21.073142 W | etcdserver: member 6d4f535bae3ab960 has a higher version 3.2.0
2017-04-27 14:15:21.073157 W | etcdserver: the local etcd version 3.1.7 is not up-to-date
2017-04-27 14:15:21.073164 W | etcdserver: member c89feb932daef420 has a higher version 3.2.0
```

#### 4. Repeat step 2 to step 3 for all other members

#### 5. Finish

When all members are upgraded, the cluster will report upgrading to 3.2 successfully:

```
2017-04-27 14:15:54.536901 N | etcdserver/membership: updated the cluster version from 3.1 to 3.2
2017-04-27 14:15:54.537035 I | etcdserver/api: enabled capabilities for version 3.2
```

```
$ ETCDCTL_API=3 /etcdctl endpoint health --endpoints=localhost:2379,localhost:22379,localhost:32379
localhost:2379 is healthy: successfully committed proposal: took = 2.312897ms
localhost:22379 is healthy: successfully committed proposal: took = 2.553476ms
localhost:32379 is healthy: successfully committed proposal: took = 2.517902ms
```

[etcd-contact]: https://groups.google.com/forum/#!forum/etcd-dev
