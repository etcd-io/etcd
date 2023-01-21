# election

Election is a fork from [raftexample](../raftexample/) to help transmit messages between raft nodes, which removes the code of web service / kv storage and retains that of wal / snapshot / raft node / transport. It also provides a mock network module to simulate high latency packets tranmission and packets loss. The [raft module](https://github.com/HoniiTro19/raft) only contains election algorithm and ignores log replication or other parts of raft consensus algorithm.

## Getting Started

### Building election

Clone `etcd` to `<directory>/etcd`

```sh
cd <directory>/etcd
./scripts/build_election.sh
```

### Running single node election

First start a single-member cluster of election:

```sh
election --id 1 --cluster http://127.0.0.1:12379
```

### Running a local cluster

First install [goreman](https://github.com/mattn/goreman), which manages Procfile-based applications.

The [Procfile script](./Procfile) will set up a local example cluster. Start it with:

```sh
goreman -basedir contrib/election start
```
This will bring up three election instances.

### Election experiments
