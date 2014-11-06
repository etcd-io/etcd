# etcd 0.6+ Internal Storage Plan and Roadmap
etcd-dev team @ CoreOS
November 2014

## Abstract 

We propose a new design of the underlying key-value used by etcd. The goal of the new design is to improve maintainability, extensibility and potential performance of the store and sets a solid base for data reliability and scalability for the future development. The key-value pairs are immutable, multi-versioned, and the value can be arbitrary binary data. The goal of etcd is to hold millions of small keys with multiple versions with minimum snapshot pausing time. To support these requirements we plan on adding a new persistent backend to the store. We propose to first implement and test this design and then merge it for the etcd 0.6 release.

## Background

The [store pkg](https://github.com/coreos/etcd/blob/master/store/store.go#L41-L62) is currently used by etcd as the underlying storage system. It is a monolithic store that consists of both an in-memory hash based tree storage and an in-memory events history lists (to support watch functionality). The original design goal of the store was simplicity and rapid development time. The store has allowed the project to iterate quickly and add a variety of new features to etcd. However, today this store design and implementation cannot satisfy the future needs of the etcd project. 

## Requirements

### Snapshots

Snapshotting the state of the system is a data scalability challenge for all state-machine-based consistent replication systems like ZooKeeper, Chubby, etcd, doozerd, and consul.

etcd currently uses a naive approach. It first pauses all store operations (i.e. “stops the world”) and does a deep copy of the in-memory data tree, then it resumes the world and writes the snapshot to disk. This has two major drawbacks: 
  1. it doubles the memory usage while taking the snapshot
  2. it blocks the progress of the local etcd process

#### Chubby

The first version of Chubby’s Paxos implementation used the same approach as etcd: pause the world and copy the whole data store in memory, then resume the world and write the copied version of the data store to disk. The second version of Chubby’s Paxos implemented virtually pause-less snapshots. It utilizes a “shadow” data structure to track updates while the underlying database is serialized to disk.

#### Zookeeper

ZooKeeper’s solution to the problem is to take fuzzy snapshots. While the ZooKeeper server is taking a snapshot, updates still can occur to the data store. Thus, the snapshot includes a subset of the updates to the data tree that occurred while the snapshot was in process. The snapshot may not correspond to any data store that actually existed. ZooKeeper can recover using this snapshot because it takes advantage of the idempotent nature of its updates. By replaying the transaction log against fuzzy snapshots, ZooKeeper recreates the state of the system at the end of the log.

#### Others

doozerd also supports non-pausing snapshots. It implements an in-memory MVCC data store. The snapshot routine is able to grab a version of the store, and any concurrent updates occur as newer versions. 

Consul uses LMDB as its persistent layer. LMDB also supports non-pausing snapshots by taking advantage of its multiple version B-Tree implementation.

However, all of the systems mentioned above still suffer from slow snapshots and huge overhead as the number of in-memory keys grows to O(10*million). They always write the whole snapshot (O(GB)) to disk even if the changes are relative small (O(MB)). 

[lmdb] http://symas.com/mdb/

#### etcd’s proposal

We presume that modern-day Chubby is using leveldb or SSTables as its persistence layer. SSTables are a log-structured data store which support incremental snapshots. That means that Chubby amortizes the snapshot time to each operation. When the disk usage is low, it spins out another routine to compact the on-disk log structured files.

In the [Raft thesis](raft-thesis), Diego also discusses performing incremental snapshots by using a log-structured state machine. 

We propose that etcd should support a non-pausing approach and incremental snapshots.

[raft-thesis]: https://ramcloud.stanford.edu/~ongaro/thesis.pdf

### Event History

We keep a history of the results of previous operations, so users can _watch_ for events within the history window and get the same result as they would at a previous point in time. A better solution is to keep the actual old version of the value, instead of just keeping the operation result (e.g. modification) in a limited-length queue. This makes our store a _multiple version store_ and potentially provides for unbounded history.

The application can control when to clean up the old versions. So it does not need to worry about recovering from an unknown state and have to costly get a whole copy of global view when failure happens and it cannot keep up with the event history.

This also makes it easier to implement an optimistic concurrency controller. Multiple writers can write to a shared store and the store itself keeps the version history. After all of the writing completes, the application can scan the version histories to detect update conflicts and determine if the final version is valid or it needs to retry.

### Memory Usage
	
Keeping 10 million small key-value pairs should not be a huge problem: assuming each in-memory key-value pair representation is 1KB, total memory consumption is 10GB. However, if an application wants to keep multiple versions for each key (N*10 millions versions in the entire store), we might want to store the old versions/cold keys on disk. Further, if we want to support incremental snapshotting, we probably need/want to store the key-value pairs on disk.

## Proposal

We propose the following changes to the store implementation in etcd:
- separating the etcd (application) store and the actual underlying storage
  - etcd store should handle all application-specific logic (for TTLs, CAS operation, etc)
- implementing a new etcd store backend
 - support for binary values and multiple versioned key
 - support for non-pausing and incremental snapshots with low memory usage
 - support for persistent (i.e. on disk) backend

For the implementation, we propose to use BTree as its in-memory index, and implement bitcask as the persistence layer. 

We plan to implement the store interface with a fake persistence layer first. Then hook-up that store with current etcd API. Finally, we implement the bitcask backend.

A simplified version of the proposed interface for the underlying storage follows:
```go
type store interface {
	// return the Value of the newly put key
	Put(key string, value []byte) Value
	// return the value of the key at the given revision
	Get(revision uint64, key string) Value
	Delete(key string) Value
	List(revision uint64, prefix string, depth int, limit int) []Value
	Multiops(ops []Op) [][]Value
	// get current revision
	Head() uint64 
}

type Value interface {
	Data() []byte
	Revision() uint64
}

type Op struct {
	typ  uint32
	key string
	value []byte
	...
}
```

And the proposed interface for the etcd store:
```go
type etcdstore interface {
	Set(key string, value []byte, ttl uint64)
	CAS(key string, expectRev uint64, value []byte)
	Wait(key string, expectRev uint64) 
	....
}
```

##  Implementation details

### In-memory Index

The in-memory index is a lookup table that points to the actual value on disk and might, in future, be a hot cache of the disk backend. The index must be memory efficient: we need to ensure it can hold the entire key space in memory, at least initially.

We plan to use a level sorted b-tree with each node representing a key. For example:

```
Level sorted:
/a/b
/a/c
/a/d
/a/c/d
/a/c/e
/b/c/c
/b/e/f

level order sorting: a/d < a/c/d since the first one only has two level.
List(/a) -> /a/b /a/c /a/d
```

The data of the node in b-tree is a linked-list of values with different revisions. For each key we keep all the previous versions (revisions) of the value. The representation of the key-value pair is hence (key, List of Values).

```go
type KeyValue struct {
	k     string
	v     *Value
}

type Value struct {
	uint64  rev
	[]byte   data
	*Value next
}
```

```
User    Put(foo, “bar”)
--------------------------------
Store   Create Value {1, “bar”, nil} at 0x1
        Create Path{“foo”, 0x1}
--------------------------------
User    Put(foo, “barbar”)
--------------------------------
Store   Create Value {2, “barbar”, nil} at 0x2
        Update 0x1   {1, "bar", 0x2}
---------------------------------
User    Get(1, “foo”) -> “bar”
User    Get(2, “foo”) -> “barbar”
```

#### Alternative Indexing (Open for opinions)

There are other alternatives for how we might implement the index. For example, a Hashmap:
```
HashMap: key string -> list of [value []byte, children []string]
/a -> value:5, children: /a/b, /a/c
List(/a) -> /a/b, /a/c
```
In this approach, we need to keep an additional children list for each version of the key (however, for reasons of efficiency, we can point to the old version of children if the new version of a key’s children does not change).

### Persistence and Incremental Snapshots

As discussed above, we want to store the old versions and cold keys to the disk. We also wish to support incremental snapshots. 

In fact, writing all the key-values to disk will not hurt the performance of etcd in general. We need to save the entire key-value space to disk at some point due to the need of snapshot. The only change is to make the in-memory store an index to the actual on-disk store. If we are particularly concerned about read performance, we can do caching. 

We want a log-structure on disk representation, which allows us to do incremental snapshots.

We propose to implement a go version of [Bitcask][bitcask-intro], which is used by the distributed database [Riak][bitcask-riak] in production. It is a log-structured, on-disk store, with the constraint that the key map must fit in memory. Since etcd only targets storing O(10^7) keys, this constraint should not be a problem. Therefore, we think Bitcask is probably the simplest solution for us right now. 

[bitcask-intro]: https://github.com/basho/bitcask/blob/develop/doc/bitcask-intro.pdf
[bitcask-riak]: http://docs.basho.com/riak/latest/ops/advanced/backends/bitcask/

Further references for disk backend:
http://kafka.apache.org/documentation.html#compaction
http://kafka.apache.org/documentation.html#persistence
https://www.igvita.com/2012/02/06/sstable-and-log-structured-storage-leveldb/
http://wiki.apache.org/cassandra/MemtableSSTable
https://issues.apache.org/jira/browse/CASSANDRA-1074
http://wiki.apache.org/cassandra/ArchitectureSSTable
https://www.youtube.com/watch?v=KTCkW_6zz2k
http://en.wikipedia.org/wiki/Log-structured_merge-tree
http://nosqlsummer.org/paper/lsm-tree
http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.117.5365&rep=rep1&type=pdf