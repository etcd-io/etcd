# Optimal etcd Cluster Size

etcd's Raft consensus algorithm is most efficient in small clusters between 3 and 9 peers. Let's briefly explore how etcd works internally to understand why.

## Writing to etcd

Writes to an etcd peer are always redirected to the leader of the cluster and distributed to all of the peers immediately. A write is only considered successful when a majority of the peers acknowledge the write.

For example, in a 5 node cluster, a write operation is only as fast as the 3rd fastest machine. This is the main reason for keeping your etcd cluster below 9 nodes. In practice, you only need to worry about write performance in high latency environments such as a cluster spanning multiple data centers.

## Leader Election

The leader election process is similar to writing a key &mdash; a majority of the cluster must acknowledge the new leader before cluster operations can continue. The longer each node takes to elect a new leader means you have to wait longer before you can write to the cluster again. In low latency environments this process takes milliseconds.

## Odd Cluster Size

The other important cluster optimization is to always have an odd cluster size. Adding an odd node to the cluster doesn't change the size of the majority and therefore doesn't increase the total latency of the majority as described above. But you do gain a higher tolerance for peer failure by adding the extra machine. You can see this in practice when comparing two even and odd sized clusters:

| Cluster Size | Majority   | Failure Tolerance |
|--------------|------------|-------------------|
| 8 machines   | 5 machines | 3 machines        |
| 9 machines   | 5 machines | **4 machines**    |

As you can see, adding another node to bring the cluster up to an odd size is always worth it. During a network partition, an odd cluster size also guarantees that there will almost always be a majority of the cluster that can continue to operate and be the source of truth when the partition ends.

## Cluster Management

Currently, each CoreOS machine is an etcd peer &mdash; if you have 30 CoreOS machines, you have 30 etcd peers and end up with a cluster size that is way too large. If desired, you may manually stop some of these etcd instances to increase cluster performance.

Functionality is being developed to expose two different types of followers: active and benched followers. Active followers will influence operations within the cluster. Benched followers will not participate, but will transparently proxy etcd traffic to an active follower. This allows every CoreOS machine to expose etcd on port 4001 for ease of use. Benched followers will have the ability to transition into an active follower if needed.
