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

[members-api]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/other_apis.md#members-api

#### Contents

The data directory has two sub-directories in it:

1. wal: write ahead log files are stored here. For details see the [wal package documentation][wal-pkg]
2. snap: log snapshots are stored here. For details see the [snap package documentation][snap-pkg]

[wal-pkg]: http://godoc.org/github.com/coreos/etcd/wal
[snap-pkg]: http://godoc.org/github.com/coreos/etcd/snap

### Cluster Lifecycle

If you are spinning up multiple clusters for testing it is recommended that you specify a unique initial-cluster-token for the different clusters.
This can protect you from cluster corruption in case of mis-configuration because two members started with different cluster tokens will refuse members from each other.

### Member Migration

When there is a scheduled machine maintenance or retirement, you might want to migrate an etcd member to another machine without losing the data and changing the member ID. 

The data directory contains all the data to recover a member to its point-in-time state. To migrate a member:

* Stop the member process
* Copy the data directory of the now-idle member to the new machine
* Update the peer URLs for that member to reflect the new machine according to the [member api] [change peer url]
* Start etcd on the new machine, using the same configuration and the copy of the data directory

[change peer url]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/other_apis.md#change-the-peer-urls-of-a-member 

### Disaster Recovery

etcd is designed to be resilient to machine failures. An etcd cluster can automatically recover from any number of temporary failures (for example, machine reboots), and a cluster of N members can tolerate up to _(N/2)-1_ permanent failures (where a member can no longer access the cluster, due to hardware failure or disk corruption). However, in extreme circumstances, a cluster might permanently lose enough members such that quorum is irrevocably lost. For example, if a three-node cluster suffered two simultaneous and unrecoverable machine failures, it would be normally impossible for the cluster to restore quorum and continue functioning.

To recover from such scenarios, etcd provides functionality to backup and restore the datastore and recreate the cluster without data loss.

#### Backing up the datastore

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
