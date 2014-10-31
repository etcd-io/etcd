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
