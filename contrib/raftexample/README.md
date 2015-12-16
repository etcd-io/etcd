# raftexample

raftexample is an example usage of etcd's [raft library](../../raft). It provides a simple REST API for a key-value store cluster backed by the [Raft][raft] consensus algorithm.

[raft]: http://raftconsensus.github.io/

## Getting Started

### Running single node raftexample

First start a single-member cluster of raftexample:

```sh
raftexample --id 1 --cluster http://127.0.0.1:12379 --port 12380
```

Each raftexample process maintains a single raft instance and a key-value server.
The process's list of comma separated peers (--cluster), its raft ID index into the peer list (--id), and http key-value server port (--port) are passed through the command line.

Next, store a value ("hello") to a key ("my-key"):

```
curl -L http://127.0.0.1:12380/my-key -XPUT -d hello
```

Finally, retrieve the stored key:

```
curl -L http://127.0.0.1:12380/my-key
```

### Running a local cluster

First install [goreman](https://github.com/mattn/goreman), which manages Procfile-based applications.

The [Procfile script](./Procfile) will set up a local example cluster. You can start it with:

```sh
goreman start
```

This will bring up three raftexample instances.

You can write a key-value pair to any member of the cluster and likewise retrieve it from any member.

### Fault Tolerance

To test cluster recovery, first start a cluster and write a value "foo":
```sh
goreman start
curl -L http://127.0.0.1:12380/my-key -XPUT -d foo
```

Next, remove a node and replace the value with "bar" to check cluster availability:

```sh
goreman run stop raftexample2
curl -L http://127.0.0.1:12380/my-key -XPUT -d bar
curl -L http://127.0.0.1:32380/my-key
```

Finally, bring the node back up and verify it recovers with the updated value "bar":
```sh
goreman run start raftexample2
curl -L http://127.0.0.1:22380/my-key
```

## Design

The raftexample consists of three components: a raft-backed key-value store, a REST API server, and a raft consensus server based on etcd's raft implementation.

The raft-backed key-value store is a key-value map that holds all committed key-values.
The store bridges communication between the raft server and the REST server.
Key-value updates are issued through the store to the raft server.
The store updates its map once raft reports the updates are committed.

The REST server exposes the current raft consensus by accessing the raft-backed key-value store.
A GET command looks up a key in the store and returns the value, if any.
A key-value PUT command issues an update proposal to the store.

The raft server participates in consensus with its cluster peers.
When the REST server submits a proposal, the raft server transmits the proposal to its peers.
When raft reaches a consensus, the server publishes all committed updates over a commit channel.
For raftexample, this commit channel is consumed by the key-value store.

## Project Details

### TODO
- Snapshot support
- Dynamic reconfiguration
