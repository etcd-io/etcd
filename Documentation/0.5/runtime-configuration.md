## Runtime Reconfiguration

etcd comes with support for incremental runtime reconfiguration, which allows users to update the membership of the cluster at run time.

## Reconfiguration Use Cases

Let us walk through the four use cases for re-configuring a cluster: replacing a member, increasing or decreasing cluster size, and restarting a cluster from a majority failure.

### Replace a Non-recoverable Member

The most common use case of cluster reconfiguration is to replace a member because of a permanent failure of the existing member: for example, hardware failure or data directory corruption.
It is important to replace failed members as soon as the failure is detected.
If etcd falls below a simple majority of members it can no longer accept writes: e.g. in a 3 member cluster the loss of two members will cause writes to fail and the cluster to stop operating.

If you want to migrate a running member to another machine, please refer [member migration section][member migration].

[member migration]: https://github.com/coreos/etcd/blob/master/Documentation/0.5/admin_guide.md#member-migration

### Increase Cluster Size

To make your cluster more resilient to machine failure you can increase the size of the cluster.
For example, if the cluster consists of three machines, it can tolerate one failure.
If we increase the cluster size to five, it can tolerate two machine failures.

Increasing the cluster size can also provide better read performance.
When a client accesses etcd, the normal read gets the data from the local copy of each member (members always shares the same view of the cluster at the same index, which is guaranteed by the sequential consistency of etcd).
Since clients can read from any member, increasing the number of members thus increases overall read throughput.

### Decrease Cluster Size

To improve the write performance of a cluster, you might want to trade off resilience by removing members.
etcd replicates the data to the majority of members of the cluster before committing the write.
Decreasing the cluster size means the etcd cluster has to do less work for each write, thus increasing the write performance.

### Restart Cluster from Majority Failure

If the majority of your cluster is lost, then you need to take manual action in order to recover safely.
The basic steps in the recovery process include creating a new cluster using the old data, forcing a single member to act as the leader, and finally using runtime configuration to add members to this new cluster.

TODO: https://github.com/coreos/etcd/issues/1242

## Cluster Reconfiguration Operations

Now that we have the use cases in mind, let us lay out the operations involved in each.

Before making any change, the simple majority (quorum) of etcd members must be available.
This is essentially the same requirement as for any other write to etcd.

All changes to the cluster are done one at a time:

To replace a single member you will make an add then a remove operation
To increase from 3 to 5 members you will make two add operations
To decrease from 5 to 3 you will make two remove operations

All of these examples will use the `etcdctl` command line tool that ships with etcd.
If you want to use the member API directly you can find the documentation [here](https://github.com/coreos/etcd/blob/master/Documentation/0.5/other_apis.md).

### Remove a Member

First, we need to find the target member:

```
$ etcdctl member list
6e3bd23ae5f1eae0: name=node2 peerURLs=http://localhost:7002 clientURLs=http://127.0.0.1:4002
924e2e83e93f2560: name=node3 peerURLs=http://localhost:7003 clientURLs=http://127.0.0.1:4003
a8266ecf031671f3: name=node1 peerURLs=http://localhost:7001 clientURLs=http://127.0.0.1:4001
```

Let us say the member ID we want to remove is a8266ecf031671f3.
We then use the `remove` command to perform the removal:

```
$ etcdctl member remove a8266ecf031671f3
Removed member a8266ecf031671f3 from cluster
```

The target member will stop itself at this point and print out the removal in the log:

```
etcd: this member has been permanently removed from the cluster. Exiting.
```

Removal of the leader is safe, but the cluster will be out of progress for a period of election timeout because it needs to elect the new leader.

### Add a Member

Adding a member is a two step process:

 * Add the new member to the cluster via the [members API](https://github.com/coreos/etcd/blob/master/Documentation/0.5/other_apis.md#post-v2members) or the `etcdctl member add` command.
 * Start the member with the correct configuration.

Using `etcdctl` let's add the new member to the cluster:

```
$ etcdctl member add infra3 http://10.0.1.13:2380
added member 9bf1b35fc7761a23 to cluster
ETCD_NAME="infra3"
ETCD_INITIAL_CLUSTER="infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380,infra2=http://10.0.1.12:2380,infra3=http://10.0.1.13:2380"
ETCD_INITIAL_CLUSTER_STATE=existing
```

> Notice that infra3 was added to the cluster using its advertised peer URL.

Now start the new etcd process with the relevant flags for the new member:

```
$ export ETCD_NAME="infra3"
$ export ETCD_INITIAL_CLUSTER="infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380,infra2=http://10.0.1.12:2380,infra3=http://10.0.1.13:2380"
$ export ETCD_INITIAL_CLUSTER_STATE=existing
$ etcd -listen-client-urls http://10.0.1.13:2379 -advertise-client-urls http://10.0.1.13:2379  -listen-peer-urls http://10.0.1.13:2380 -initial-advertise-peer-urls http://10.0.1.13:2380
```

The new member will run as a part of the cluster and immediately begin catching up with the rest of the cluster.

If you are adding multiple members the best practice is to configure the new member, then start the process, then configure the next, and so on.
A common case is increasing a cluster from 1 to 3: if you add one member to a 1-node cluster, the cluster cannot make progress before the new member starts because it needs two members as majority to agree on the consensus.

#### Error Cases

In the following case we have not included our new host in the list of enumerated nodes.
If this is a new cluster, the node must be added to the list of initial cluster members.

```
$ etcd -name infra3 \
  -initial-cluster infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380,infra2=http://10.0.1.12:2380 \
  -initial-cluster-state existing
etcdserver: assign ids error: the member count is unequal
exit 1
```

In this case we give a different address (10.0.1.14:2380) to the one that we used to join the cluster (10.0.1.13:2380).

```
$ etcd -name infra4 \
  -initial-cluster infra0=http://10.0.1.10:2380,infra1=http://10.0.1.11:2380,infra2=http://10.0.1.12:2380,infra4=http://10.0.1.14:2380 \
  -initial-cluster-state existing
etcdserver: assign ids error: unmatched member while checking PeerURLs
exit 1
```

When we start etcd using the data directory of a removed member, etcd will exit automatically if it connects to any alive member in the cluster:

```
$ etcd
etcd: this member has been permanently removed from the cluster. Exiting.
exit 1
```
