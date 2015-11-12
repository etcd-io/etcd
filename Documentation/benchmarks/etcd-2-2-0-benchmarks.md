# Benchmarking etcd v2.2.0

## Physical Machines

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

## Testing

Bootstrap another machine, outside of the etcd cluster, and run the [`boom` HTTP benchmark tool](https://github.com/rakyll/boom) with a connection reuse patch to send requests to each etcd cluster member. See the [benchmark instructions](../../hack/benchmark/) for the patch and the steps to reproduce our procedures.

The performance is calulated through results of 100 benchmark rounds.

## Performance

### Single Key Read Performance

| key size in bytes | number of clients | target etcd server | average read QPS | read QPS stddev | average 90th Percentile Latency (ms) | latency stddev |
|-------------------|-------------------|--------------------|------------------|-----------------|--------------------------------------|----------------|
| 64 | 1 | leader only | 2303.100058 | 200.195677 | 0.490909 | 0.055215 |
| 64 | 64 | leader only | 15048.153165 | 685.109960 | 7.580808 | 0.456530 |
| 64 | 256 | leader only | 14507.925844 | 433.994436 | 29.757576 | 1.046697 |
| 256 | 1 | leader only | 2161.752194 | 213.860718 | 0.523232 | 0.061674 |
| 256 | 64 | leader only | 14789.363681 | 791.971785 | 7.685859 | 0.475885 |
| 256 | 256 | leader only | 14424.322382 | 511.801593 | 29.924242 | 1.421202 |
| 64 | 64 | all servers | 45751.755033 | 2048.297281 | 2.471717 | 0.137833 |
| 64 | 256 | all servers | 46591.906475 | 1273.488203 | 10.143434 | 0.588812 |
| 256 | 64 | all servers | 45331.517278 | 1847.016935 | 2.484848 | 0.122568 |
| 256 | 256 | all servers | 46484.600176 | 1339.932381 | 10.175758 | 0.737055 |

### Single Key Write Performance

| key size in bytes | number of clients | target etcd server | average write QPS | write QPS stddev | average 90th Percentile Latency (ms) | latency stddev |
|-------------------|-------------------|--------------------|------------------|-----------------|--------------------------------------|----------------|
| 64 | 1 | leader only | 54.945088 | 4.118924 | 24.505051 | 13.260608 |
| 64 | 64 | leader only | 2139.147290 | 124.798268 | 35.232323 | 3.401391 |
| 64 | 256 | leader only | 4580.721572 | 581.281641 | 70.534343 | 10.216071 |
| 256 | 1 | leader only | 55.547778 | 3.688401 | 22.367677 | 4.333702 |
| 256 | 64 | leader only | 2051.523698 | 151.040962 | 36.838384 | 4.198309 |
| 256 | 256 | leader only | 4441.967122 | 560.205410 | 71.591919 | 10.031644 |
| 64 | 64 | all servers | 1625.122714 | 85.339261 | 58.509091 | 5.137426 |
| 64 | 256 | all servers | 4461.190079 | 297.759157 | 89.472727 | 36.479273 |
| 256 | 64 | all servers | 1599.365323 | 94.007225 | 60.111111 | 6.426113 |
| 256 | 256 | all servers | 4315.163628 | 192.787326 | 88.975758 | 7.009403 |

## Performance Changes

- Because etcd now records metrics for each API call, read QPS performance seems to see a minor decrease in most scenarios. This minimal performance impact was judged a reasonable investment for the breadth of monitoring and debugging information returned.

- Write QPS to cluster leaders seems to be increased by a small margin. This is because the main loop and entry apply loops were decoupled in the etcd raft logic, eliminating several blocks between them.

- Write QPS to all members seems to be increased by a significant margin, because followers now receive the latest commit index sooner, and commit proposals more quickly.
