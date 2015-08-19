## Physical machines

GCE n1-highcpu-2 machine type

- 1x dedicated local SSD mounted under /var/lib/etcd
- 1x dedicated slow disk for the OS
- 1.8 GB memory
- 2x CPUs

## etcd Cluster

3 etcd 2.2.0-alpha.1 members, each runs on a single machine.

Detailed versions:

```
etcd Version: 2.2.0-alpha.1+git
Git SHA: 28b61ac
Go Version: go1.4.2
Go OS/Arch: linux/amd64
```

Also, we use 3 etcd 2.1.0 alpha-stage members to form cluster to get base performance. etcd's commit head is at [c7146bd5](https://github.com/coreos/etcd/commits/c7146bd5f2c73716091262edc638401bb8229144), which is the same as the one that we use in [etcd 2.1 benchmark](./etcd-2-1-0-benchmarks.md).

## Testing

Bootstrap another machine and use benchmark tool [boom](https://github.com/rakyll/boom) to send requests to each etcd member. Check [here](../../hack/benchmark/) for instructions.

## Performance

### reading one single key

| key size in bytes | number of clients | target etcd server | read QPS | 90th Percentile Latency (ms) |
|-------------------|-------------------|--------------------|----------|---------------|
| 64                | 1                 | leader only        | 2216 (+5%) | 0.5 (-17%) |
| 64                | 64                | leader only        | 16038 (-10%) | 6.1 (+0%) |
| 64                | 256               | leader only        | 15497 (-16%) | 22.4 (+5%) |
| 256               | 1                 | leader only        | 2115  (-8%) | 0.5 (+0%) |
| 256               | 64                | leader only        | 16083 (-13%) | 6.1 (+8%) |
| 256               | 256               | leader only        | 15444 (-17%) | 21.9 (+2%) |
| 64                | 64                | all servers        | 45101 (-9%) | 2.1 (+5%) |
| 64                | 256               | all servers        | 50558 (-14%) | 8.0 (+8%) |
| 256               | 64                | all servers        | 45415 (-8%) | 2.1 (+5%) |
| 256               | 256               | all servers        | 50531 (-14%) | 8.1 (+20%) |

### writing one single key

| key size in bytes | number of clients | target etcd server | write QPS | 90th Percentile Latency (ms) |
|-------------------|-------------------|--------------------|-----------|---------------|
| 64                | 1                 | leader only        | 61 (+3%)  | 18.0 (-15%) |
| 64                | 64                | leader only        | 2092 (+14%) | 37.2 (-8%) |
| 64                | 256               | leader only        | 2407 (-43%) | 71.0 (+2%) |
| 256               | 1                 | leader only        | 60 (+15%)  | 18.5 (-38%) |
| 256               | 64                | leader only        | 2186 (+33%) | 37.2 (-16%) |
| 256               | 256               | leader only        | 2385 (-42%) | 81.9 (+8%) |
| 64                | 64                | all servers        | 1758 (+72%) | 53.1 (-50%) |
| 64                | 256               | all servers        | 4547 (+31%) | 86.7 (-31%) |
| 256               | 64                | all servers        | 1667 (+66%) | 54.7 (-50%) |
| 256               | 256               | all servers        | 4695 (+33%) | 81.3 (-25%) |

### performance changes explanation

- read QPS in all scenarios is decreased by 10~20%. One reason is that etcd records store metrics for each store operation. The metrics is important for monitoring and debugging, so this is acceptable. The other reason is that HTTP handler checks key access permission in each request for authentication purpose. We could improve this by skipping the check when authentication feature is disabled.

- write QPS to leader is increased by 10~20%, except 256-client cases. This is because we decouple raft main loop and entry apply loop, which avoids them blocking each other.

- write QPS to leader using 256 clients is decreased by 40%. This is caused by etcd limiting the number of client connections improperly. We will enhance the method to eliminate this performance downgrade.

- write QPS to all servers is increased by 30~70% because follower could receive latest commit index earlier and commit proposals faster.
