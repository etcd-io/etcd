---
title: etcd versus other key-value stores
---

The name "etcd" originated from two ideas, the unix "/etc" folder and "d"istributed systems. The "/etc" folder is a place to store configuration data for a single system whereas etcd stores configuration information for large scale distributed systems. Hence, a "d"istributed "/etc" is "etcd".

etcd is designed as a general substrate for large scale distributed systems. These are systems that will never tolerate split-brain operation and are willing to sacrifice availability to achieve this end. etcd stores metadata in a consistent and fault-tolerant way. An etcd cluster is meant to provide key-value storage with best of class stability, reliability, scalability and performance.

Distributed systems use etcd as a consistent key-value store for configuration management, service discovery, and coordinating distributed work. Many [organizations][production-users] use etcd to implement production systems such as container schedulers, service discovery services, and distributed data storage. Common distributed patterns using etcd include [leader election][etcd-etcdctl-elect], [distributed locks][etcd-etcdctl-lock], and monitoring machine liveness.

## Use cases

- Container Linux by CoreOS: Applications running on [Container Linux][container-linux] get automatic, zero-downtime Linux kernel updates. Container Linux uses [locksmith] to coordinate updates. Locksmith implements a distributed semaphore over etcd to ensure only a subset of a cluster is rebooting at any given time.
- [Kubernetes][kubernetes] stores configuration data into etcd for service discovery and cluster management; etcd's consistency is crucial for correctly scheduling and operating services. The Kubernetes API server persists cluster state into etcd. It uses etcd's watch API to monitor the cluster and roll out critical configuration changes.

## Comparison chart

Perhaps etcd already seems like a good fit, but as with all technological decisions, proceed with caution. Please note this documentation is written by the etcd team. Although the ideal is a disinterested comparison of technology and features, the authors’ expertise and biases obviously favor etcd. Use only as directed.

The table below is a handy quick reference for spotting the differences among etcd and its most popular alternatives at a glance. Further commentary and details for each column are in the sections following the table.

|  | etcd | ZooKeeper | Consul | NewSQL (Cloud Spanner, CockroachDB, TiDB) |
| --- | --- | --- | --- | --- |
| Concurrency Primitives |  [Lock RPCs][etcd-v3lock], [Election RPCs][etcd-v3election], [command line locks][etcd-etcdctl-lock], [command line elections][etcd-etcdctl-elect], [recipes][etcd-recipe]  in go | External [curator recipes][curator] in Java | [Native lock API][consul-lock] | [Rare][newsql-leader], if any |
| Linearizable Reads | [Yes][etcd-linread] | No | [Yes][consul-linread] | Sometimes |
| Multi-version Concurrency Control | [Yes][etcd-mvcc] | No | No | Sometimes |
| Transactions | [Field compares, Read, Write][etcd-txn] | [Version checks, Write][zk-txn] | [Field compare, Lock, Read, Write][consul-txn]  | SQL-style |
| Change Notification | [Historical and current key intervals][etcd-watch] | [Current keys and directories][zk-watch] | [Current keys and prefixes][consul-watch] | Triggers (sometimes) |
| User permissions | [Role based][etcd-rbac] | [ACLs][zk-acl] | [ACLs][consul-acl] | Varies (per-table [GRANT][cockroach-grant], per-database [roles][spanner-roles]) |
| HTTP/JSON API | [Yes][etcd-json] | No | [Yes][consul-json] | Rarely |
| Membership Reconfiguration | [Yes][etcd-reconfig] | [>3.5.0][zk-reconfig] | [Yes][consul-reconfig] | Yes |
| Maximum reliable database size | Several gigabytes | Hundreds of megabytes (sometimes several gigabytes) | Hundreds of MBs | Terabytes+ |
| Minimum read linearization latency | Network RTT | No read linearization | RTT + fsync | Clock barriers (atomic, NTP) |

### ZooKeeper

ZooKeeper solves the same problem as etcd: distributed system coordination and metadata storage. However, etcd has the luxury of hindsight taken from engineering and operational experience with ZooKeeper’s design and implementation. The lessons learned from Zookeeper certainly informed etcd’s design, helping it support large scale systems like Kubernetes. The improvements etcd made over Zookeeper include:

* Dynamic cluster membership reconfiguration
* Stable read/write under high load
* A multi-version concurrency control data model
* Reliable key monitoring which never silently drop events
* Lease primitives decoupling connections from sessions
* APIs for safe distributed shared locks

Furthermore, etcd supports a wide range of languages and frameworks out of the box. Whereas Zookeeper has its own custom Jute RPC protocol, which is totally unique to Zookeeper and limits its [supported language bindings][zk-bindings], etcd’s client protocol is built from [gRPC][grpc], a popular RPC framework with language bindings for go, C++, Java, and more. Likewise, gRPC can be serialized into JSON over HTTP, so even general command line utilities like `curl` can talk to it. Since systems can select from a variety of choices, they are built on etcd with native tooling rather than around etcd with a single fixed set of technologies.

When considering features, support, and stability, new applications planning to use Zookeeper for a consistent key value store would do well to choose etcd instead.

### Consul

Consul is an end-to-end service discovery framework. It provides built-in health checking, failure detection, and DNS services. In addition, Consul exposes a key value store with RESTful HTTP APIs. [As it stands in Consul 1.0][dbtester-comparison-results], the storage system does not scale as well as other systems like etcd or Zookeeper in key-value operations; systems requiring millions of keys will suffer from high latencies and memory pressure. The key value API is missing, most notably, multi-version keys, conditional transactions, and reliable streaming watches.

etcd and Consul solve different problems. If looking for a distributed consistent key value store, etcd is a better choice over Consul. If looking for end-to-end cluster service discovery, etcd will not have enough features; choose Kubernetes, Consul, or SmartStack.

### NewSQL (Cloud Spanner, CockroachDB, TiDB)

Both etcd and NewSQL databases (e.g., [Cockroach][cockroach], [TiDB][tidb], [Google Spanner][spanner]) provide strong data consistency guarantees with high availability. However, the significantly different system design parameters lead to significantly different client APIs and performance characteristics.

NewSQL databases are meant to horizontally scale across data centers. These systems typically partition data across multiple consistent replication groups (shards), potentially distant, storing data sets on the order of terabytes and above. This sort of scaling makes them poor candidates for distributed coordination as they have long latencies from waiting on clocks and expect updates with mostly localized dependency graphs. The data is organized into tables, including SQL-style query facilities with richer semantics than etcd, but at the cost of additional complexity for processing, planning, and optimizing queries.

In short, choose etcd for storing metadata or coordinating distributed applications. If storing more than a few GB of data or if full SQL queries are needed, choose a NewSQL database.

## Using etcd for metadata

etcd replicates all data within a single consistent replication group. For storing up to a few GB of data with consistent ordering, this is the most efficient approach. Each modification of cluster state, which may change multiple keys, is assigned a global unique ID, called a revision in etcd, from a monotonically increasing counter for reasoning over ordering. Since there’s only a single replication group, the modification request only needs to go through the raft protocol to commit. By limiting consensus to one replication group, etcd gets distributed consistency with a simple protocol while achieving low latency and high throughput.

The replication behind etcd cannot horizontally scale because it lacks data sharding. In contrast, NewSQL databases usually shard data across multiple consistent replication groups, storing data sets on the order of terabytes and above. However, to assign each modification a global unique and increasing ID, each request must go through an additional coordination protocol among replication groups. This extra coordination step may potentially conflict on the global ID, forcing ordered requests to retry. The result is a more complicated approach with typically worse performance than etcd for strict ordering.

If an application reasons primarily about metadata or metadata ordering, such as to coordinate processes, choose etcd. If the application needs a large data store spanning multiple data centers and does not heavily depend on strong global ordering properties, choose a NewSQL database.

## Using etcd for distributed coordination

etcd has distributed coordination primitives such as event watches, leases, elections, and distributed shared locks out of the box (Note that in the case of the distributed shared lock, users need to be aware about its non obvious properties. The details are described below). These primitives are both maintained and supported by the etcd developers; leaving these primitives to external libraries shirks the responsibility of developing foundational distributed software, essentially leaving the system incomplete. NewSQL databases usually expect these distributed coordination primitives to be authored by third parties. Likewise, ZooKeeper famously has a separate and independent [library][curator] of coordination recipes. Consul, which provides a native locking API, goes so far as to apologize that it’s “[not a bulletproof method][consul-bulletproof]”.

In theory, it’s possible to build these primitives atop any storage systems providing strong consistency. However, the algorithms tend to be subtle; it is easy to develop a locking algorithm that appears to work, only to suddenly break due to  thundering herd and timing skew. Furthermore, other primitives supported by etcd, such as transactional memory depend on etcd’s MVCC data model; simple strong consistency is not enough.

For distributed coordination, choosing etcd can help prevent operational headaches and save engineering effort.

### Notes on the usage of lock and lease
etcd provides [lock APIs][etcd-v3lock] which are based on [the lease mechanism][lease] and [its implementation in etcd][etcdlease]. The basic idea of the lease mechanism is: a server grants a token, which is called a lease, to a requesting client. When the server grants a lease, it associates a TTL with the lease. When the server detects the passage of time longer than the TTL, it revokes the lease. While the client holds a non revoked lease it can claim that it owns access to a resource associated with the lease. In the case of etcd, the resource is a key in the etcd keyspace. etcd provides lock APIs with this scheme. However, the lock APIs cannot be used as mutual exclusion mechanism by themselves. The APIs are called lock because [for historical reasons][chubby]. The lock APIs can, however, be used as an optimization mechanism of mutual exclusion as described below.

The most important aspect of the lease mechanism is that TTL is defined as a physical time interval. Both of the server and client measures passing of time with their own clocks. It allows a situation that the server revokes the lease but the client still claims it owns the lease.

Then how does the lease mechanism guarantees mutual exclusion of the locking mechanism? Actually, the lease mechanism itself doesn't guarantee mutual exclusion. Owning a lease cannot guarantee the owner holds a lock of the resource.

In the case of controlling mutual accesses to keys of etcd itself with etcd lock, mutual exclusion is implemented based on the mechanism of version number validation (it is sometimes called compare and swap in other systems like Consul). In etcd's RPCs like `Put` or `Txn`, we can specify required conditions about revision number and lease ID for the operations. If the conditions are not satisfied, the operation can fail. With this mechanism, etcd provides distributed locking for clients. It means that a client knows that it is acquiring a lock of a key when its requests are completed by etcd cluster successfully.

In distributed locking literature similar designs are described:
* In [the paper of Chubby][chubby], the concept of *sequencer* is introduced. We interpret that sequencer is an almost same to the combination of revision number and lease ID of etcd.
* In [How to do distributed locking][fencing], Martin Kleppmann introduced the idea of *fencing token*. The authors interpret that fencing token is revision number in the case of etcd. In [Note on fencing and distributed locks][fencing-zk] Flavio Junqueira discussed how the idea of fencing token should be implemented in the case of zookeeper.
* In [Practical Uses of Synchronized Clocks in Distributed Systems][physicalclock], we can find a description that Thor implements a distributed locking mechanism based on version number validation and lease.

Why do etcd and other systems provide lease if they provide mutual exclusion based on version number validation? Well, leases provide an optimization mechanism for reducing a number of aborted requests.

Note that in the case of etcd keys, it can be locked efficiently because of the mechanisms of lease and version number validation. If users need to protect resources which aren't related to etcd, the resources must provide the version number validation mechanism and consistency of replicas like keys of etcd. The lock feature of etcd itself cannot be used for protecting external resources.

The [lock directory][executable-example] contains an executable example which follows [the scenario described in the article by Martin Kleppmann][fencing]. The example shows how stop the world GC of go runtime can cause false revoking of etcd lock and how version number validation can prevent access to the shared resource from a client which acquires a stale lock.

[production-users]: ../../ADOPTERS.md
[grpc]: https://www.grpc.io
[consul-bulletproof]: https://www.consul.io/docs/internals/sessions.html
[curator]: http://curator.apache.org/
[cockroach]: https://github.com/cockroachdb/cockroach
[spanner]: https://cloud.google.com/spanner/
[tidb]: https://github.com/pingcap/tidb
[etcd-v3lock]: https://godoc.org/github.com/etcd-io/etcd/etcdserver/api/v3lock/v3lockpb
[etcd-v3election]: https://godoc.org/github.com/coreos/etcd-io/etcdserver/api/v3election/v3electionpb
[etcd-etcdctl-lock]: ../../etcdctl/README.md#lock-lockname-command-arg1-arg2-
[etcd-etcdctl-elect]: ../../etcdctl/README.md#elect-options-election-name-proposal
[etcd-mvcc]: data_model.md
[etcd-recipe]: https://godoc.org/github.com/etcd-io/etcd/contrib/recipes
[consul-lock]: https://www.consul.io/docs/commands/lock.html
[newsql-leader]: http://dl.acm.org/citation.cfm?id=2960999
[etcd-reconfig]: ../op-guide/runtime-configuration.md
[zk-reconfig]: https://zookeeper.apache.org/doc/trunk/zookeeperReconfig.html
[consul-reconfig]: https://www.consul.io/docs/guides/servers.html
[etcd-linread]: api_guarantees.md#linearizability
[consul-linread]: https://www.consul.io/docs/agent/http.html#consistency
[etcd-json]: ../dev-guide/api_grpc_gateway.md
[consul-json]: https://www.consul.io/docs/agent/http.html#formatted-json-output
[etcd-txn]: api.md#transaction
[zk-txn]: https://zookeeper.apache.org/doc/r3.4.3/api/org/apache/zookeeper/ZooKeeper.html#multi(java.lang.Iterable)
[consul-txn]: https://www.consul.io/docs/agent/http/kv.html#txn
[etcd-watch]: api.md#watch-streams
[zk-watch]: https://zookeeper.apache.org/doc/trunk/zookeeperProgrammers.html#ch_zkWatches
[consul-watch]: https://www.consul.io/docs/agent/watches.html
[etcd-commonname]: ../op-guide/authentication.md#using-tls-common-name
[etcd-rbac]: ../op-guide/authentication.md#working-with-roles
[zk-acl]: https://zookeeper.apache.org/doc/r3.1.2/zookeeperProgrammers.html#sc_ZooKeeperAccessControl
[consul-acl]: https://www.consul.io/docs/internals/acl.html
[cockroach-grant]: https://www.cockroachlabs.com/docs/stable/grant.html
[spanner-roles]: https://cloud.google.com/spanner/docs/iam#roles
[zk-bindings]: https://zookeeper.apache.org/doc/r3.1.2/zookeeperProgrammers.html#ch_bindings
[container-linux]: https://coreos.com/why
[locksmith]: https://github.com/coreos/locksmith
[kubernetes]: https://kubernetes.io/docs/concepts/overview/what-is-kubernetes/
[dbtester-comparison-results]: https://github.com/coreos/dbtester/tree/master/test-results/2018Q1-02-etcd-zookeeper-consul
[etcdlease]: https://godoc.org/github.com/etcd-io/etcd/clientv3#Lease
[lease]: https://web.stanford.edu/class/cs240/readings/89-leases.pdf
[chubby]: https://research.google/pubs/pub27897/
[fencing]: https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
[physicalclock]: http://www.dainf.cefetpr.br/~tacla/SDII/PracticalUseOfClocks.pdf
[fencing-zk]: https://fpj.me/2016/02/10/note-on-fencing-and-distributed-locks/
[executable-example]: lock/README.md
