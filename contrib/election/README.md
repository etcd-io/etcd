# Election

Election is a fork from [raftexample](../raftexample/) and removes the code of web service and kv storage, since we only need to run the election process and do not propose writes, reads or confchanges to the [raft module](https://github.com/HoniiTro19/raft). No data is actually stored in raft instances in our experiments, while wal is still retained to persist raft hard state and no-op entry. Though no-op entry seems not necessary since no data entry needs to commit, we still maintain this point and do not wait until no-op entry commits when counting election time. Snapshot is also kept while we have not yet decided to compact log entries and trigger snapshots. 

Election also provides a mock network module to simulate high latency packets tranmission and message loss. The [raft module](https://github.com/HoniiTro19/raft) used in Election only contains election algorithm and messages about votes and heartbeats, and ignores other parts of raft consensus algorithm.

For more information about raft consensus algorithm, please refer to [raft dissertation](https://github.com/ongardie/dissertation/blob/master/stanford.pdf) since etcd is a good implementation of this paper.

## Getting Started

### Building Election

Clone `etcd` to `<directory>/etcd` and build the project.

```sh
cd <directory>/etcd
./scripts/build_election.sh
```

### Running A Single Node

First start a single-member cluster of election with the default configuration.

```sh
election --id 1 --cluster http://127.0.0.1:12379
```

### Running A Local Cluster

First install [goreman](https://github.com/mattn/goreman), which manages Procfile-based applications.

The [Procfile script](./Procfile) will set up a local example cluster. Goreman will use the procfile to bring up three raft instances and continuously run the election algorithm. 

```
# Use goreman to run `go install github.com/mattn/goreman@latest`
electionexample1: ../../bin/contrib/election --id 1 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379
electionexample2: ../../bin/contrib/election --id 2 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379
electionexample3: ../../bin/contrib/election --id 3 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379
```

To start the local cluster with goreman, run the following command.

```sh
goreman -basedir contrib/election start
```

### Election Experiments
|   parameter   |   detail  |
|  ----  | ----  |
|  cluster | comma separated cluster peers |
| id  | member index in the cluster peers |
| duration | alive duration of the raft instance in seconds |
| latency | average latency of the real network condition |
| resdir | the directory to output experiment logs |
| mocknet | whether to use mock network module to simulate message latency and loss |
| msgloss | ratio to trigger message loss in percentage (only works when mocknet is true) |
| msgdelay | additional latency for message transmission (only works when mocknet is true) |
