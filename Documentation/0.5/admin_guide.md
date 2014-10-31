##Administration
###Data Directory

When first started, etcd stores its configuration into the data directory. This configuration includes the local member ID, cluster ID, and initial cluster configuration. The directory contains all data that etcd needs to operate and recover after a restart (for example in order to rejoin a cluster).

If a member’s data directory is ever lost or corrupted, the user should remove the etcd member from the cluster via the [members API][0]. (The member can then be re-added to the cluster with an empty data directory, again using the [members API][0], and it will recover state).

An etcd member restarted with previously used command line arguments but a new data directory is considered a different member. And it can potentially corrupt the cluster. If you are spinning up multiple clusters for testing it is recommended that you specify a unique initial-cluster-token for the different clusters. This can protect you from cluster corruption in case of the misconfiguration metioned above.

The data directory has two sub-directories in it:

1. wal: etcd stores its append-only log files in the wal directory. Before any command gets committed, etcd ensures that the log entry that represents the command is written to wal. The name of the wal file is <seq>-<logindex>.wal. Seq is the sequence number of log file, it starts from 0. Log index is the expected index of the first log entry written to the log file. etcd will create new wal file periodically with a higher sequence number.(TODO: how to clean old wal file)

2. snap  (TODO: format not stable)

###Things to avoid:
1. restart a etcd member without previous data directory
2. restart a etcd member with different data directory

[0]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/other_apis.md#members-api
