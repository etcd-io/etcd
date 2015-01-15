## Administration

### Data Directory

#### Lifecycle

When first started, etcd stores its configuration into a data directory specified by the data-dir configuration parameter.
Configuration is stored in the write ahead log and includes: the local member ID, cluster ID, and initial cluster configuration.
The write ahead log and snapshot files are used during member operation and to recover after a restart.

If a member’s data directory is ever lost or corrupted then the user should remove the etcd member from the cluster via the [members API][members-api].

A user should avoid restarting an etcd member with a data directory from an out-of-date backup.
Using an out-of-date data directory can lead to inconsistency as the member had agreed to store information via raft then re-joins saying it needs that information again.
For maximum safety, if an etcd member suffers any sort of data corruption or loss, it must be removed from the cluster.
Once removed the member can be re-added with an empty data directory.

[members-api]: https://github.com/coreos/etcd/blob/master/Documentation/other_apis.md#members-api

#### Contents

The data directory has two sub-directories in it:

1. wal: write ahead log files are stored here. For details see the [wal package documentation][wal-pkg]
2. snap: log snapshots are stored here. For details see the [snap package documentation][snap-pkg]

[wal-pkg]: http://godoc.org/github.com/coreos/etcd/wal
[snap-pkg]: http://godoc.org/github.com/coreos/etcd/snap

### Cluster Management

#### Lifecycle

If you are spinning up multiple clusters for testing it is recommended that you specify a unique initial-cluster-token for the different clusters.
This can protect you from cluster corruption in case of mis-configuration because two members started with different cluster tokens will refuse members from each other.

#### Optimal Cluster Size

The recommended etcd cluster size is 3, 5 or 7, which is decided by the fault tolerance requirement. A 7-member cluster can provide enough fault tolerance in most cases. While larger cluster provides better fault tolerance, its write performance becomes lower since data needs to be replicated to more machines.

#### Fault Tolerance Table

It is recommended to have an odd number of members in a cluster. Having an odd cluster size doesn't change the number needed for majority, but you gain a higher tolerance for failure by adding the extra member. You can see this in practice when comparing even and odd sized clusters:

| Cluster Size | Majority   | Failure Tolerance |
|--------------|------------|-------------------|
| 1 | 1 | 0 |
| 3 | 2 | 1 |
| 4 | 3 | 1 |
| 5 | 3 | **2** |
| 6 | 4 | 2 |
| 7 | 4 | **3** |
| 8 | 5 | 3 |
| 9 | 5 | **4** |

As you can see, adding another member to bring the size of cluster up to an odd size is always worth it. During a network partition, an odd number of members also guarantees that there will almost always be a majority of the cluster that can continue to operate and be the source of truth when the partition ends.

### Member Migration

When there is a scheduled machine maintenance or retirement, you might want to migrate an etcd member to another machine without losing the data and changing the member ID. 

The data directory contains all the data to recover a member to its point-in-time state. To migrate a member:

* Stop the member process
* Copy the data directory of the now-idle member to the new machine
* Update the peer URLs for that member to reflect the new machine according to the [member api] [change peer url]
* Start etcd on the new machine, using the same configuration and the copy of the data directory

This example will walk you through the process of migrating the infra1 member to a new machine:

|Name|Peer URL|
|------|--------------|
|infra0|10.0.1.10:2380|
|infra1|10.0.1.11:2380|
|infra2|10.0.1.12:2380|

```
$ export ETCDCTL_PEERS=http://10.0.1.10:2379,http://10.0.1.11:2379,http://10.0.1.12:2379
```

```
$ etcdctl member list
84194f7c5edd8b37: name=infra0 peerURLs=http://10.0.1.10:2380 clientURLs=http://127.0.0.1:2379,http://10.0.1.10:2379
b4db3bf5e495e255: name=infra1 peerURLs=http://10.0.1.11:2380 clientURLs=http://127.0.0.1:2379,http://10.0.1.11:2379
bc1083c870280d44: name=infra2 peerURLs=http://10.0.1.12:2380 clientURLs=http://127.0.0.1:2379,http://10.0.1.12:2379
```

#### Stop the member etcd process

```
$ ssh core@10.0.1.11
```

```
$ sudo systemctl stop etcd
```

#### Copy the data directory of the now-idle member to the new machine

```
$ tar -cvzf node1.etcd.tar.gz /var/lib/etcd/node1.etcd 
```

```
$ scp node1.etcd.tar.gz core@10.0.1.13:~/
```

#### Update the peer URLs for that member to reflect the new machine

```
$ curl http://10.0.1.10:2379/v2/members/b4db3bf5e495e255 -XPUT \
-H "Content-Type: application/json" -d '{"peerURLs":["http://10.0.1.13:2380"]}'
```

#### Start etcd on the new machine, using the same configuration and the copy of the data directory

```
$ ssh core@10.0.1.13
```

```
$ tar -xzvf node1.etcd.tar.gz -C /var/lib/etcd
```

```
etcd -name node1 \
-listen-peer-urls http://10.0.1.13:2380 \
-listen-client-urls http://10.0.1.13:2379,http://127.0.0.1:2379 \
-advertise-client-urls http://10.0.1.13:2379,http://127.0.0.1:2379
```

[change peer url]: https://github.com/coreos/etcd/blob/master/Documentation/other_apis.md#change-the-peer-urls-of-a-member

### Disaster Recovery

etcd is designed to be resilient to machine failures. An etcd cluster can automatically recover from any number of temporary failures (for example, machine reboots), and a cluster of N members can tolerate up to _(N/2)-1_ permanent failures (where a member can no longer access the cluster, due to hardware failure or disk corruption). However, in extreme circumstances, a cluster might permanently lose enough members such that quorum is irrevocably lost. For example, if a three-node cluster suffered two simultaneous and unrecoverable machine failures, it would be normally impossible for the cluster to restore quorum and continue functioning.

To recover from such scenarios, etcd provides functionality to backup and restore the datastore and recreate the cluster without data loss.

#### Backing up the datastore

**NB:** Windows users must stop etcd before running the backup command.

The first step of the recovery is to backup the data directory on a functioning etcd node. To do this, use the `etcdctl backup` command, passing in the original data directory used by etcd. For example:

```sh
    etcdctl backup \
      --data-dir /var/lib/etcd \
      --backup-dir /tmp/etcd_backup
```

This command will rewrite some of the metadata contained in the backup (specifically, the node ID and cluster ID), which means that the node will lose its former identity. In order to recreate a cluster from the backup, you will need to start a new, single-node cluster. The metadata is rewritten to prevent the new node from inadvertently being joined onto an existing cluster.

#### Restoring a backup

To restore a backup using the procedure created above, start etcd with the `-force-new-cluster` option and pointing to the backup directory. This will initialize a new, single-member cluster with the default advertised peer URLs, but preserve the entire contents of the etcd data store. Continuing from the previous example:

```sh
    etcd \
      -data-dir=/tmp/etcd_backup \
      -force-new-cluster \
      ...
```

Now etcd should be available on this node and serving the original datastore.

Once you have verified that etcd has started successfully, shut it down and move the data back to the previous location (you may wish to make another copy as well to be safe):

```sh
    pkill etcd
    rm -fr /var/lib/etcd
    mv /tmp/etcd_backup /var/lib/etcd
    etcd \
      -data-dir=/var/lib/etcd \
      ...
```

#### Restoring the cluster

Now that the node is running successfully, you can add more nodes to the cluster and restore resiliency. See the [runtime configuration](runtime-configuration.md) guide for more details.

### Client Request Timeout

etcd sets different timeouts for various types of client requests. The timeout value is not tunable now, which will be improved soon(https://github.com/coreos/etcd/issues/2038).

#### Get requests

No timeout is set for get requests, because etcd can always return the result in a short time.

#### Watch requests

Unlimited timeout is set for watch requests. Clients can keep long-polling watch requests as long as they want.

#### Delete, Put, Post, QuorumGet requests

The default timeout is large enough to allow all key modifications in a healthy cluster, even when it is electing a new leader.

If the request times out, it indicates three possibilities:

1. the request is lost when switching leader, which occurs occasionally
2. the request is sent to a network-isolated member
3. the cluster is unhealthy, or even out-of-work

If case 2 or 3 happens, administrators should resolve it as soon as possible.
