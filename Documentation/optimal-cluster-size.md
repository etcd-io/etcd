# Optimal etcd Cluster Size

etcd's Raft consensus algorithm is most efficient in small clusters between 3 and 9 peers. For clusters larger than 9, etcd will select a subset of instances to participate in the algorithm in order to keep it efficient. The end of this document briefly explores how etcd works internally and why these choices have been made.

## Cluster Management

You can manage the active cluster size through the [cluster config API](https://github.com/coreos/etcd/blob/master/Documentation/api.md#cluster-config). `activeSize` represents the etcd peers allowed to actively participate in the consensus algorithm.

If the total number of etcd instances exceeds this number, additional peers are started as [standbys](https://github.com/coreos/etcd/blob/master/Documentation/design/standbys.md), which can be promoted to active participation if one of the existing active instances has failed or been removed.

## Internals of etcd

### Writing to etcd

Writes to an etcd peer are always redirected to the leader of the cluster and distributed to all of the peers immediately. A write is only considered successful when a majority of the peers acknowledge the write.

For example, in a cluster with 5 peers, a write operation is only as fast as the 3rd fastest machine. This is the main reason for keeping the number of active peers below 9. In practice, you only need to worry about write performance in high latency environments such as a cluster spanning multiple data centers.

### Leader Election

The leader election process is similar to writing a key &mdash; a majority of the active peers must acknowledge the new leader before cluster operations can continue. The longer each peer takes to elect a new leader means you have to wait longer before you can write to the cluster again. In low latency environments this process takes milliseconds.

### Odd Active Cluster Size

The other important cluster optimization is to always have an odd active cluster size (i.e. `activeSize`). Adding an odd node to the number of peers doesn't change the size of the majority and therefore doesn't increase the total latency of the majority as described above. But, you gain a higher tolerance for peer failure by adding the extra machine. You can see this in practice when comparing two even and odd sized clusters:

| Active Peers | Majority   | Failure Tolerance |
|--------------|------------|-------------------|
| 1 peers      | 1 peers    | None              |
| 3 peers      | 2 peers    | 1 peer            |
| 4 peers      | 3 peers    | 1 peer           |
| 5 peers      | 3 peers    | **2 peers**       |
| 6 peers      | 4 peers    | 2 peers           |
| 7 peers      | 4 peers    | **3 peers**       |
| 8 peers      | 5 peers    | 3 peers           |
| 9 peers      | 5 peers    | **4 peers**       |

As you can see, adding another peer to bring the number of active peers up to an odd size is always worth it. During a network partition, an odd number of active peers also guarantees that there will almost always be a majority of the cluster that can continue to operate and be the source of truth when the partition ends.
