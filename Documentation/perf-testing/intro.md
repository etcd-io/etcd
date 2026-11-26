# Introduction to performance testing

The ClusterLoader2 tests deploy etcd and benchmark clients in a Kubernetes cluster, collect metrics through Prometheus, and report results to PerfDash.

## How the tests run

| Component | Responsibility |
| --- | --- |
| Prow | Starts jobs and publishes logs and artifacts. |
| `kubernetes/test-infra` | Defines job schedules, source revisions, commands, and machines. |
| `etcd-io/etcd` | Provides the server under test. |
| `kubernetes/perf-tests` | Provides ClusterLoader2, the etcd scenario, and the benchmark client and image. |

The [Prow jobs][cluster-jobs] provision a Kubernetes cluster and run the [etcd scenario][scenario] through ClusterLoader2.
The scenario installs etcd-operator, creates an etcd cluster, loads initial data, and starts writer pods.
The [cluster specification][etcd-cluster-template] selects the etcd version, member count, storage, and server configuration.

```text
Prow job                            Test cluster
  ClusterLoader2 -----------------> Kubernetes API
       |                              |
       |                              +--> etcd-operator --> etcd pods
       |                              +--> Benchmark client pods
       |                              +--> Prometheus
       |
       +--- Collect measurements <--- Prometheus

  Benchmark client pods --> etcd Service --> etcd pods
  Prometheus -----------------------------> etcd metrics
```

ClusterLoader2 already provides Prometheus metric collection and reports for PerfDash.
Reusing these was the main reason for the [original integration][cl2-etcd-integration].
Clients and servers can run on different machines to avoid competing for host resources.
That placement requires scheduling rules and suitable nodes.

## The benchmark client

The client is `request-benchmark etcd-put` from `kubernetes/perf-tests`.
Its implementation is in [mode_etcd_put.go][etcd-put-client], separate from the `benchmark` command in the etcd repository.
It uses the etcd Go client to send writes at a configured rate, with serialized Pod objects or random bytes as values.
It also supports background compaction and defragmentation.

The [writer template][writer-template] selects the client image and passes the workload parameters as command arguments.
The image is built from [util-images/request-benchmark][client-source] and published using its [Makefile][request-benchmark-image].

## Limits of the benchmark inside the job container

The [local benchmark script][benchmark-script] builds and runs etcd and its client inside the same job container.
They share host resources, so client load and host contention can affect throughput and latency across runs.
This makes performance changes harder to attribute to etcd and limits repeatability.

## Results

Results include throughput and latency.
ClusterLoader2 also collects fdatasync latency, database size, and cpu and memory resource use through Prometheus.
The [measurement definitions][measurements] contain the queries.

| View | Use |
| --- | --- |
| [Prow][prow] | Read logs, source revisions, and artifacts for a run. |
| [TestGrid][testgrid] | Track job history and failures. |
| [PerfDash][perfdash] | Compare performance across runs. |

[benchmark-script]: ../../scripts/benchmark_test.sh
[request-benchmark-image]: https://github.com/kubernetes/perf-tests/blob/master/util-images/request-benchmark/Makefile
[client-source]: https://github.com/kubernetes/perf-tests/tree/master/util-images/request-benchmark
[etcd-put-client]: https://github.com/kubernetes/perf-tests/blob/master/util-images/request-benchmark/mode_etcd_put.go
[writer-template]: https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/etcd/modules/writer/deployment.yaml
[scenario]: https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/etcd/config.yaml
[cl2-etcd-integration]: https://github.com/kubernetes/perf-tests/pull/4356
[etcd-cluster-template]: https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/etcd/modules/cluster/cluster.yaml
[measurements]: https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/etcd/modules/measurements.yaml
[cluster-jobs]: https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/sig-scalability/sig-scalability-periodic-jobs.yaml
[prow]: https://prow.k8s.io/
[testgrid]: https://testgrid.k8s.io/sig-scalability-benchmarks
[perfdash]: https://perf-dash.k8s.io/
