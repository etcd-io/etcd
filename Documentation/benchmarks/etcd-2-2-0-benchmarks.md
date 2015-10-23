## Physical machines

GCE n1-highcpu-2 machine type

- 1x dedicated local SSD mounted as etcd data directory
- 1x dedicated slow disk for the OS
- 1.8 GB memory
- 2x CPUs

## etcd Cluster

3 etcd 2.2.0 members, each runs on a single machine.

Detailed versions:

```
etcd Version: 2.2.0
Git SHA: e4561dd
Go Version: go1.5
Go OS/Arch: linux/amd64
```

Also, we use 3 etcd 2.1.0 alpha-stage members to form cluster to get base performance. etcd's commit head is at [c7146bd5](https://github.com/coreos/etcd/commits/c7146bd5f2c73716091262edc638401bb8229144), which is the same as the one that we use in [etcd 2.1 benchmark](./etcd-2-1-0-benchmarks.md).

## Testing

Bootstrap another machine and use benchmark tool [boom](https://github.com/rakyll/boom) with customized patch to send requests to each etcd member. Check [here](../../hack/benchmark/) for instructions.

## Performance

### reading one single key

| key size in bytes | number of clients | target etcd server | read QPS | 90th Percentile Latency (ms) |
|-------------------|-------------------|--------------------|----------|---------------|
| 64                | 1                 | leader only        | 2816 (+2%) | 0.4 (0%) |
| 64                | 64                | leader only        | 17572 (-4%) | 6.0 (+1%) |
| 64                | 256               | leader only        | 16943 (-7%) | 24.1 (+4%) |
| 256               | 1                 | leader only        | 2349 (+7%) | 0.5 (0%) |
| 256               | 64                | leader only        | 16009 (-7%) | 7.1 (+9%)|
| 256               | 256               | leader only        | 15239 (0%) | 28.4 (-3%) |
| 64                | 64                | all servers        | 49502 (-3%) | 2.3 (+4%) |
| 64                | 256               | all servers        | 50005 (-3%) | 9.1 (0%) |
| 256               | 64                | all servers        | 48664 (-3%) | 2.3 (+4%) |
| 256               | 256               | all servers        | 49914 (-4%) | 9.1 (-2%) |

### writing one single key

| key size in bytes | number of clients | target etcd server | write QPS | 90th Percentile Latency (ms) |
|-------------------|-------------------|--------------------|-----------|---------------|
| 64                | 1                 | leader only        | 51 (-11%) | 22.3 (+1%) |
| 64                | 64                | leader only        | 2182 (+14%) | 31.5 (-14%) |
| 64                | 256               | leader only        | 5841 (+5%) | 52.0 (+2%) |
| 256               | 1                 | leader only        | 57 (+7%) | 20.7 (-7%) |
| 256               | 64                | leader only        | 1973 (+1%) | 38.3 (+8%) |
| 256               | 256               | leader only        | 5193 (+4%) | 59.8 (-7%) |
| 64                | 64                | all servers        | 1745 (+62%) | 54.2 (-49%) |
| 64                | 256               | all servers        | 4487 (+21%) | 78.0 (-29%) |
| 256               | 64                | all servers        | 1507 (+44%) | 59.5 (-43%) |
| 256               | 256               | all servers        | 4231 (+15%) | 82.6 (-25%) |

### performance changes explanation

- read QPS in most scenarios is decreased by 3~7%. The reason is that etcd records store metrics for each store operation. The metrics is important for monitoring and debugging, so this is acceptable.

- write QPS to leader is increased up to 14%. This is because we decouple raft main loop and entry apply loop, which avoids them blocking each other.

- write QPS to all servers is increased by 15~62% because follower could receive latest commit index earlier and commit proposals faster.
