go-raft [![Build Status](https://drone.io/github.com/goraft/raft/status.png)](https://drone.io/github.com/goraft/raft/latest) [![Coverage Status](https://coveralls.io/repos/goraft/raft/badge.png?branch=master)](https://coveralls.io/r/goraft/raft?branch=master)
=======

## Overview

This is a Go implementation of the Raft distributed consensus protocol.
Raft is a protocol by which a cluster of nodes can maintain a replicated state machine.
The state machine is kept in sync through the use of a replicated log.

For more details on Raft, you can read [In Search of an Understandable Consensus Algorithm][raft-paper] by Diego Ongaro and John Ousterhout.

## Project Status

This library is feature complete but should be considered experimental until it has seen more usage.
If you have any questions on implementing go-raft in your project please file an issue.
There is an [active community][community] of developers who can help.
go-raft is under the MIT license.

[community]: https://github.com/goraft/raft/contributors

### Features

- Leader election
- Log replication
- Configuration changes
- Log compaction
- Unit tests
- Fast Protobuf Log Encoding
- HTTP transport

### Projects

These projects are built on go-raft:

- [coreos/etcd](https://github.com/coreos/etcd) - A highly-available key value store for shared configuration and service discovery.
- [goraft/raftd](https://github.com/goraft/raftd) - A reference implementation for using the go-raft library for distributed consensus.
- [skynetservices/skydns](https://github.com/skynetservices/skydns) - DNS for skynet or any other service discovery.
- [influxdb/influxdb](https://github.com/influxdb/influxdb) - An open-source, distributed, time series, events, and metrics database.
- [Weed File System](https://weed-fs.googlecode.com) - A scalable distributed key-to-file system with O(1) disk access for each read.

If you have a project that you're using go-raft in, please add it to this README so others can see implementation examples.

## Contact and Resources

- [raft-dev][raft-dev] is a mailing list for discussion about best practices
  and implementation of Raft. Not goraft specific but helpful if you have
  questions.
- [Slides from Ben's talk][bens-talk] which includes easy to understand
  diagrams of leader election and replication
- The [Raft Consensus homepage][raft-home] has links to additional raft
  implementations, slides to talks on Raft and general information

[raft-home]:  http://raftconsensus.github.io/
[raft-dev]: https://groups.google.com/forum/#!forum/raft-dev
[bens-talk]: https://speakerdeck.com/benbjohnson/raft-the-understandable-distributed-consensus-protocol

## The Raft Protocol

This section provides a summary of the Raft protocol from a high level.
For a more detailed explanation on the failover process and election terms please see the full paper describing the protocol: [In Search of an Understandable Consensus Algorithm][raft-paper].

### Overview

Maintaining state in a single process on a single server is easy.
Your process is a single point of authority so there are no conflicts when reading and writing state.
Even multi-threaded processes can rely on locks or coroutines to serialize access to the data.

However, in a distributed system there is no single point of authority.
Servers can crash or the network between two machines can become unavailable or any number of other problems can occur.

A distributed consensus protocol is used for maintaining a consistent state across multiple servers in a cluster.
Many distributed systems are built upon the Paxos protocol but Paxos can be difficult to understand and there are many gaps between Paxos and real world implementation.

An alternative is the [Raft distributed consensus protocol][raft-paper] by Diego Ongaro and John Ousterhout.
Raft is a protocol built with understandability as a primary tenet and it centers around two things:

1. Leader Election
2. Replicated Log

With these two constructs, you can build a system that can maintain state across multiple servers -- even in the event of multiple failures.

### Leader Election

The Raft protocol effectively works as a master-slave system whereby state changes are written to a single server in the cluster and are distributed out to the rest of the servers in the cluster.
This simplifies the protocol since there is only one data authority and conflicts will not have to be resolved.

Raft ensures that there is only one leader at a time.
It does this by performing elections among the nodes in the cluster and requiring that a node must receive a majority of the votes in order to become leader.
For example, if you have 3 nodes in your cluster then a single node would need 2 votes in order to become the leader.
For a 5 node cluster, a server would need 3 votes to become leader.

### Replicated Log

To maintain state, a log of commands is maintained.
Each command makes a change to the state of the server and the command is deterministic.
By ensuring that this log is replicated identically between all the nodes in the cluster we can replicate the state at any point in time in the log by running each command sequentially.

Replicating the log under normal conditions is done by sending an `AppendEntries` RPC from the leader to each of the other servers in the cluster (called Peers).
Each peer will append the entries from the leader through a 2-phase commit process which ensure that a majority of servers in the cluster have entries written to log.


## Raft in Practice

### Optimal Cluster Size

The primary consideration when choosing the node count in your Raft cluster is the number of nodes that can simultaneously fail.
Because Raft requires a majority of nodes to be available to make progress, the number of node failures the cluster can tolerate is `(n / 2) - 1`.

This means that a 3-node cluster can tolerate 1 node failure.
If 2 nodes fail then the cluster cannot commit entries or elect a new leader so progress stops.
A 5-node cluster can tolerate 2 node failures. A 9-node cluster can tolerate 4 node failures.
It is unlikely that 4 nodes will simultaneously fail so clusters larger than 9 nodes are not common.

Another consideration is performance.
The leader must replicate log entries for each follower node so CPU and networking resources can quickly be bottlenecked under stress in a large cluster.


### Scaling Raft

Once you grow beyond the maximum size of your cluster there are a few options for scaling Raft:

1. *Core nodes with dumb replication.*
   This option requires you to maintain a small cluster (e.g. 5 nodes) that is involved in the Raft process and then replicate only committed log entries to the remaining nodes in the cluster.
   This works well if you have reads in your system that can be stale.

2. *Sharding.*
   This option requires that you segment your data into different clusters.
   This option works well if you need very strong consistency and therefore need to read and write heavily from the leader.

If you have a very large cluster that you need to replicate to using Option 1 then you may want to look at performing hierarchical replication so that nodes can better share the load.


## History

Ben Johnson started this library for use in his behavioral analytics database called [Sky](https://github.com/skydb/sky).
He put it under the MIT license in the hopes that it would be useful for other projects too.

[raft-paper]: https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf
