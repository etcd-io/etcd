# Performance

## Understanding performance

etcd provides stable, sustained high performance. Two factors define performance: latency and throughput. Latency is the time taken to complete an operation. Throughput is the total operations completed within some time period. Usually average latency increases as the overall throughput increases when etcd accepts concurrent client requests. In common cloud environments, like a standard `n-4` on Google Compute Engine (GCE) or a comparable machine type on AWS, a three member etcd cluster finishes a request in less than one millisecond under light load, and can complete more than 30,000 requests per second under heavy load.

etcd uses the Raft consensus algorithm to replicate requests among members and reach agreement. Consensus performance, especially commit latency, is limited by two physical constraints: network IO latency and disk IO latency. The minimum time to finish an etcd request is the network Round Trip Time (RTT) between members, plus the time `fdatasync` requires to commit the data to permanent storage. The RTT within a datacenter may be as long as several hundred microseconds. A typical RTT within the United States is around 50ms, and can be as slow as 400ms between continents. The typical fdatasync latency for a spinning disk is about 10ms. For SSDs, the latency is often lower than 1ms. To increase throughput, etcd batches multiple requests together and submits them to Raft. This batching policy lets etcd attain high throughput despite heavy load.

There are other sub-systems which impact the overall performance of etcd. Each serialized etcd request must run through etcd’s boltdb-backed MVCC storage engine, which usually takes tens of microseconds to finish. Periodically etcd incrementally snapshots its recently applied requests, merging them back with the previous on-disk snapshot. This process may lead to a latency spike. Although this is usually not a problem on SSDs, it may double the observed latency on HDD. Likewise, inflight compactions can impact etcd’s performance. Fortunately, the impact is often insignificant since the compaction is staggered so it does not compete for resources with regular requests. The RPC system, gRPC, gives etcd a well-defined, extensible API, but it also introduces additional latency, especially for local reads.

## Benchmarks

Benchmarking etcd performance can be done with the [benchmark](https://github.com/coreos/etcd/tree/master/tools/benchmark) CLI tool included with etcd.

For some baseline performance numbers, we consider a three member etcd cluster with the following hardware configuration:

- Google Cloud Compute Engine
- 3 machines of 8 vCPUs + 16GB Memory + 50GB SSD
- 1 machine(client) of 16 vCPUs + 30GB Memory + 50GB SSD
- Ubuntu 17.04
- etcd 3.2.0, go 1.8.3

With this configuration, etcd can approximately write:

| Number of keys | Key size in bytes | Value size in bytes | Number of connections | Number of clients | Target etcd server | Average write QPS | Average latency per request | Average server RSS |
|---------------:|------------------:|--------------------:|----------------------:|------------------:|--------------------|------------------:|----------------------------:|-------------------:|
| 10,000 | 8 | 256 | 1 | 1 | leader only | 583 | 1.6ms | 48 MB |
| 100,000 | 8 | 256 | 100 | 1000 | leader only | 44,341 | 22ms |  124MB |
| 100,000 | 8 | 256 | 100 | 1000 | all members |  50,104 | 20ms |  126MB |

Sample commands are:

```sh
# write to leader
benchmark --endpoints=${HOST_1} --target-leader --conns=1 --clients=1 \
    put --key-size=8 --sequential-keys --total=10000 --val-size=256
benchmark --endpoints=${HOST_1} --target-leader  --conns=100 --clients=1000 \
    put --key-size=8 --sequential-keys --total=100000 --val-size=256

# write to all members
benchmark --endpoints=${HOST_1},${HOST_2},${HOST_3} --conns=100 --clients=1000 \
    put --key-size=8 --sequential-keys --total=100000 --val-size=256
```

Linearizable read requests go through a quorum of cluster members for consensus to fetch the most recent data. Serializable read requests are cheaper than linearizable reads since they are served by any single etcd member, instead of a quorum of members, in exchange for possibly serving stale data. etcd can read: 

| Number of requests | Key size in bytes | Value size in bytes | Number of connections | Number of clients | Consistency | Average read QPS | Average latency per request |
|-------------------:|------------------:|--------------------:|----------------------:|------------------:|-------------|-----------------:|----------------------------:|
| 10,000 | 8 | 256 | 1 | 1 | Linearizable | 1,353 | 0.7ms |
| 10,000 | 8 | 256 | 1 | 1 | Serializable | 2,909 | 0.3ms |
| 100,000 | 8 | 256 | 100 | 1000 | Linearizable | 141,578 | 5.5ms |
| 100,000 | 8 | 256 | 100 | 1000 | Serializable | 185,758 | 2.2ms |

Sample commands are:

```sh
# Single connection read requests
benchmark --endpoints=${HOST_1},${HOST_2},${HOST_3} --conns=1 --clients=1 \
    range YOUR_KEY --consistency=l --total=10000
benchmark --endpoints=${HOST_1},${HOST_2},${HOST_3} --conns=1 --clients=1 \
    range YOUR_KEY --consistency=s --total=10000

# Many concurrent read requests
benchmark --endpoints=${HOST_1},${HOST_2},${HOST_3} --conns=100 --clients=1000 \
    range YOUR_KEY --consistency=l --total=100000
benchmark --endpoints=${HOST_1},${HOST_2},${HOST_3} --conns=100 --clients=1000 \
    range YOUR_KEY --consistency=s --total=100000
```

We encourage running the benchmark test when setting up an etcd cluster for the first time in a new environment to ensure the cluster achieves adequate performance; cluster latency and throughput can be sensitive to minor environment differences.

## Disks

Fast disk is very important for a healthy etcd cluster. etcd exposes very useful [metrics][metrics] such as `wal_fsync_duration_seconds` and `backend_commit_duration_seconds` via Prometheus. Other tools such as [fio][fio] and [diskbench][diskbench] can provide information such as IOPS and read/write rate in MiBs.

Sample fio command random read/write 16k blocks: * Note this will generate a 4GB file.

```sh
fio --randrepeat=1 \
  --ioengine=libaio \
  --direct=1 \
  --gtod_reduce=1 \
  --name=etcd-disk-io-test \
  --filename=etcd_read_write.io \
  --bs=16k --iodepth=64 --size=4G \
  --readwrite=randrw --rwmixread=75
```

Example output of SSD drive:

```sh
etcd-disk-io-test: (g=0): rw=randrw, bs=(R) 4096B-4096B, (W) 4096B-4096B, (T) 4096B-4096B, ioengine=libaio, iodepth=64
fio-3.7
Starting 1 process
etcd-disk-io-test: Laying out IO file (1 file / 4096MiB)
Jobs: 1 (f=1): [m(1)][100.0%][r=362MiB/s,w=119MiB/s][r=92.6k,w=30.6k IOPS][eta 00m:00s]
etcd-disk-io-test: (groupid=0, jobs=1): err= 0: pid=7019: Fri Mar 22 09:57:17 2019
   read: IOPS=90.1k, BW=352MiB/s (369MB/s)(3070MiB/8722msec)
   bw (  KiB/s): min=323568, max=391608, per=100.00%, avg=361922.35, stdev=17944.92, samples=17
   iops        : min=80892, max=97902, avg=90480.59, stdev=4486.23, samples=17
  write: IOPS=30.1k, BW=118MiB/s (123MB/s)(1026MiB/8722msec)
   bw (  KiB/s): min=106504, max=130680, per=100.00%, avg=120970.35, stdev=6126.94, samples=17
   iops        : min=26626, max=32670, avg=30242.59, stdev=1531.74, samples=17
  cpu          : usr=12.53%, sys=52.90%, ctx=135891, majf=0, minf=7
  IO depths    : 1=0.1%, 2=0.1%, 4=0.1%, 8=0.1%, 16=0.1%, 32=0.1%, >=64=100.0%
     submit    : 0=0.0%, 4=100.0%, 8=0.0%, 16=0.0%, 32=0.0%, 64=0.0%, >=64=0.0%
     complete  : 0=0.0%, 4=100.0%, 8=0.0%, 16=0.0%, 32=0.0%, 64=0.1%, >=64=0.0%
     issued rwts: total=785920,262656,0,0 short=0,0,0,0 dropped=0,0,0,0
     latency   : target=0, window=0, percentile=100.00%, depth=64

Run status group 0 (all jobs):
   READ: bw=352MiB/s (369MB/s), 352MiB/s-352MiB/s (369MB/s-369MB/s), io=3070MiB (3219MB), run=8722-8722msec
  WRITE: bw=118MiB/s (123MB/s), 118MiB/s-118MiB/s (123MB/s-123MB/s), io=1026MiB (1076MB), run=8722-8722msec

Disk stats (read/write):
    dm-4: ios=767338/256593, merge=0/0, ticks=435534/40456, in_queue=476977, util=98.96%, aggrios=395222/131476, aggrmerge=0/0, aggrticks=225096/20908, aggrin_queue=246457, aggrutil=98.64%
    dm-1: ios=790444/262880, merge=0/0, ticks=450192/41589, in_queue=492687, util=98.64%, aggrios=790444/262680, aggrmerge=0/200, aggrticks=440412/37872, aggrin_queue=482628, aggrutil=100.00%
  nvme0n1: ios=790444/262680, merge=0/200, ticks=440412/37872, in_queue=482628, util=100.00%
    dm-0: ios=0/72, merge=0/0, ticks=0/227, in_queue=227, util=0.66%, aggrios=0/36, aggrmerge=0/36, aggrticks=0/119, aggrin_queue=389, aggrutil=3.78%
  nvme1n1: ios=0/36, merge=0/36, ticks=0/119, in_queue=389, util=3.78%
```

Sample fio command seqential write 32k blocks: * Note this will generate a 4GB file.

```sh
fio --randrepeat=1 \
  --ioengine=libaio \
  --direct=1 \
  --name=etcd-disk-io-test \
  --filename=etcd_write.io \
  --bs=32k --size=4G \
  --readwrite=write
```

Example output of SSD drive:

```sh
etcd-disk-io-test: (g=0): rw=write, bs=(R) 32.0KiB-32.0KiB, (W) 32.0KiB-32.0KiB, (T) 32.0KiB-32.0KiB, ioengine=libaio, iodepth=1
fio-3.7
Starting 1 process
Jobs: 1 (f=1): [W(1)][100.0%][r=0KiB/s,w=332MiB/s][r=0,w=10.6k IOPS][eta 00m:00s]
etcd-disk-io-test: (groupid=0, jobs=1): err= 0: pid=18563: Fri Mar 22 15:05:38 2019
  write: IOPS=11.9k, BW=371MiB/s (390MB/s)(4096MiB/11026msec)
    slat (usec): min=4, max=5208, avg= 6.74, stdev=15.34
    clat (nsec): min=932, max=8648.8k, avg=76501.51, stdev=85294.07
     lat (usec): min=62, max=8668, avg=83.34, stdev=88.97
    clat percentiles (usec):
     |  1.00th=[   62],  5.00th=[   63], 10.00th=[   63], 20.00th=[   64],
     | 30.00th=[   65], 40.00th=[   68], 50.00th=[   69], 60.00th=[   71],
     | 70.00th=[   73], 80.00th=[   78], 90.00th=[   93], 95.00th=[  112],
     | 99.00th=[  182], 99.50th=[  237], 99.90th=[  351], 99.95th=[ 1369],
     | 99.99th=[ 4490]
   bw (  KiB/s): min=321600, max=416064, per=99.99%, avg=380357.82, stdev=27434.07, samples=22
   iops        : min=10050, max=13002, avg=11886.18, stdev=857.31, samples=22
  lat (nsec)   : 1000=0.01%
  lat (usec)   : 2=0.01%, 10=0.01%, 50=0.02%, 100=92.39%, 250=7.18%
  lat (usec)   : 500=0.34%, 750=0.01%, 1000=0.01%
  lat (msec)   : 2=0.02%, 4=0.02%, 10=0.01%
  cpu          : usr=2.52%, sys=8.34%, ctx=131116, majf=0, minf=14
  IO depths    : 1=100.0%, 2=0.0%, 4=0.0%, 8=0.0%, 16=0.0%, 32=0.0%, >=64=0.0%
     submit    : 0=0.0%, 4=100.0%, 8=0.0%, 16=0.0%, 32=0.0%, 64=0.0%, >=64=0.0%
     complete  : 0=0.0%, 4=100.0%, 8=0.0%, 16=0.0%, 32=0.0%, 64=0.0%, >=64=0.0%
     issued rwts: total=0,131072,0,0 short=0,0,0,0 dropped=0,0,0,0
     latency   : target=0, window=0, percentile=100.00%, depth=1

Run status group 0 (all jobs):
  WRITE: bw=371MiB/s (390MB/s), 371MiB/s-371MiB/s (390MB/s-390MB/s), io=4096MiB (4295MB), run=11026-11026msec

Disk stats (read/write):
    dm-4: ios=0/129866, merge=0/0, ticks=0/9932, in_queue=9932, util=86.31%, aggrios=0/65816, aggrmerge=0/0, aggrticks=0/5003, aggrin_queue=5003, aggrutil=84.65%
    dm-1: ios=0/131362, merge=0/0, ticks=0/9674, in_queue=9673, util=84.65%, aggrios=0/131352, aggrmerge=0/10, aggrticks=0/4097, aggrin_queue=11133, aggrutil=99.66%
  nvme0n1: ios=0/131352, merge=0/10, ticks=0/4097, in_queue=11133, util=99.66%
    dm-0: ios=0/271, merge=0/0, ticks=0/333, in_queue=333, util=1.12%, aggrios=0/132, aggrmerge=0/139, aggrticks=0/214, aggrin_queue=693, aggrutil=5.41%
  nvme1n1: ios=0/132, merge=0/139, ticks=0/214, in_queue=693, util=5.41%
```

[metrics]: ../metrics.md
[diskbench]: https://github.com/ongardie/diskbenchmark
[fio]: https://github.com/axboe/fio
