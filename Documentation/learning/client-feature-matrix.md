---
title: Client feature matrix
---

## Features

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
Automatic retry | Yes | .
Retry backoff | Yes | .
Automatic failover | Yes | .
Load balancer |	Round-Robin | ·
`WithRequireLeader(context.Context)` | Yes | .
`TLS` | Yes | Yes
`SetEndpoints` | Yes | .
`Sync` endpoints | Yes | .
`AutoSyncInterval` | Yes | .
`KeepAlive` ping | Yes | .
`MaxCallSendMsgSize` | Yes | .
`MaxCallRecvMsgSize` | Yes | .
`RejectOldCluster` | Yes | .

## [KV](https://godoc.org/go.etcd.io/etcd/clientv3#KV)


Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`Put` | Yes | .
`Get` | Yes | .
`Delete` | Yes | .
`Compact` | Yes | .
`Do(Op)` | Yes | .
`Txn` | Yes | .

## [Lease](https://godoc.org/go.etcd.io/etcd/clientv3#Lease)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`Grant` | Yes | .
`Revoke` | Yes | .
`TimeToLive` | Yes | .
`Leases` | Yes | .
`KeepAlive` | Yes | .
`KeepAliveOnce` | Yes | .

## [Watcher](https://godoc.org/go.etcd.io/etcd/clientv3#Watcher)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`Watch` | Yes | Yes
`RequestProgress` | Yes | .

## [Cluster](https://godoc.org/go.etcd.io/etcd/clientv3#Cluster)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`MemberList` | Yes | Yes
`MemberAdd` | Yes | Yes
`MemberRemove` | Yes | Yes
`MemberUpdate` | Yes | Yes

## [Maintenance](https://godoc.org/go.etcd.io/etcd/clientv3#Maintenance)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`AlarmList` | Yes | Yes
`AlarmDisarm` | Yes | ·
`Defragment` | Yes | ·
`Status` | Yes | ·
`HashKV` | Yes | ·
`Snapshot` | Yes | ·
`MoveLeader` | Yes | ·

## [Auth](https://godoc.org/go.etcd.io/etcd/clientv3#Auth)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`AuthEnable` | Yes | .
`AuthDisable` | Yes | .
`UserAdd` | Yes | .
`UserDelete` | Yes | .
`UserChangePassword` | Yes | .
`UserGrantRole` | Yes | .
`UserGet` | Yes | .
`UserList` | Yes | .
`UserRevokeRole` | Yes | .
`RoleAdd` | Yes | .
`RoleGrantPermission` | Yes | .
`RoleGet` | Yes | .
`RoleList` | Yes | .
`RoleRevokePermission` | Yes | .
`RoleDelete` | Yes | .

## [clientv3util](https://godoc.org/go.etcd.io/etcd/clientv3/clientv3util)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`KeyExists` | Yes | No
`KeyMissing` | Yes | No

## [Concurrency](https://godoc.org/go.etcd.io/etcd/clientv3/concurrency)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`Session` | Yes | No
`NewMutex(Session, prefix)` | Yes | No
`NewElection(Session, prefix)` | Yes | No
`NewLocker(Session, prefix)` | Yes | No
`STM Isolation SerializableSnapshot` | Yes | No
`STM Isolation Serializable` | Yes | No
`STM Isolation RepeatableReads` | Yes | No
`STM Isolation ReadCommitted` | Yes | No
`STM Get` | Yes | No
`STM Put` | Yes | No
`STM Rev` | Yes | No
`STM Del` | Yes | No

## [Leasing](https://godoc.org/go.etcd.io/etcd/clientv3/leasing)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`NewKV(Client, prefix)` | Yes | No

## [Mirror](https://godoc.org/go.etcd.io/etcd/clientv3/mirror)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`SyncBase` | Yes | No
`SyncUpdates` | Yes | No

## [Namespace](https://godoc.org/go.etcd.io/etcd/clientv3/namespace)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`KV` | Yes | No
`Lease` | Yes | No
`Watcher` | Yes | No

## [Naming](https://godoc.org/go.etcd.io/etcd/clientv3/naming)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`GRPCResolver` | Yes | No

## [Ordering](https://godoc.org/go.etcd.io/etcd/clientv3/ordering)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`KV` | Yes | No

## [Snapshot](https://godoc.org/go.etcd.io/etcd/clientv3/snapshot)

Feature | `clientv3-grpc1.14` | `jetcd v0.0.2`
:-------|:--------------------|:--------------
`Save` | Yes | No
`Status` | Yes | No
`Restore` | Yes | No
