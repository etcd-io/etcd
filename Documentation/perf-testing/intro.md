# Introduction to performance testing

This guide explains how Prow runs etcd performance tests and publishes their results.

## From a test definition to a running benchmark

Four components define and run the tests:

| Component | Responsibility |
| --- | --- |
| Prow | Starts jobs from schedules or supported GitHub events and tracks their execution. |
| `kubernetes/test-infra` | Defines which repositories, commands, environment, and resources each job uses. |
| `etcd-io/etcd` | Provides the etcd source, benchmark client, and scripts for running local benchmarks. |
| `kubernetes/perf-tests` | Provides tools and scenarios for deploying benchmark components and collecting measurements. |

Prow runs each job as a pod in a build cluster, a Kubernetes cluster dedicated to test execution.

```text
Schedule or supported GitHub event
                |
                v
       Prow selects a job
       defined in test-infra
                |
                v
       Job pod in a build cluster
         - Source checkouts
         - Test commands
                |
                +--> Run etcd and its client in the job container
                |
                +--> Provision a separate test cluster
                     and deploy etcd and client pods there
                |
                v
       Logs, measurements, and saved files
                |
                v
       Prow run page and result dashboards
```

Prow prepares the source checkouts and starts the configured commands.
Those commands prepare the environment, run the benchmark, and collect its output.
The job definition selects the triggers and source revisions, such as a branch for scheduled tests or a supported pull request.
The [Prow contributor guide][prow-guide] explains the GitHub commands.

## Where etcd and its clients run

The tests use two deployment arrangements.
In both, benchmark clients send requests directly to etcd.

### Inside the job container

The test builds etcd from its source checkout and starts it as a local process.
A client in the same container sends requests through the local network interface.
The script waits for etcd to become ready and stops it after the benchmark.

Contributors can also run this benchmark on a development host.
The client and server share CPU, memory, and storage resources, so either process can limit the result.
The [benchmark script][benchmark-script] and [job definition][api-job] contain the implementation.

### As pods in a separate test cluster

The job provisions a Kubernetes cluster and starts ClusterLoader2, a tool that deploys test components and collects measurements.
ClusterLoader2 uses the Kubernetes API to install etcd-operator and request an etcd cluster.
The operator manages the etcd pods and their storage.
ClusterLoader2 initializes test data and starts the client pods.

```text
Prow job pod                         Separate test cluster
  ClusterLoader2 -----------------> Kubernetes API
       |                              |
       |                              +--> etcd-operator --> etcd pods
       |                              +--> Benchmark client pods
       |                              +--> Prometheus
       |
       +--- Collect measurements <--- Prometheus

Within the test cluster:
  Benchmark client pods --> etcd Service --> etcd pods
  Prometheus -----------------------------> etcd metrics
```

The test cluster has its own etcd for the Kubernetes control plane.
The benchmark targets a separate etcd deployment that the operator manages.

The scenario can use data shaped like Kubernetes objects and include compaction and defragmentation.
These are etcd operations, not Kubernetes API requests.

Separate pods do not guarantee separate machines or dedicated resources.
The [test scenario][scenario] describes the sequence, and the [job definitions][cluster-jobs] select its parameters and infrastructure.

## How results reach contributors

Clients measure throughput, the number of completed requests per second, and latency, the time each request takes.
ClusterLoader2 also collects server measurements from Prometheus, such as disk timings, database size, and resource use.

Prow saves the reports as artifacts and publishes them with the logs and execution status.
Contributors use three views of the results:

| View | Use |
| --- | --- |
| [Prow][prow] | Open an individual run and inspect its logs, source revisions, and artifacts. |
| [TestGrid][testgrid] | Follow job history and find failed or interrupted runs. |
| [PerfDash][perfdash] | Compare reported performance measurements across runs. |

A successful run means that its commands and assertions pass, not that performance improves.
Use the measurements to assess performance, including whether completed requests keep pace with the configured rate.

## Follow a test or investigate a change

Start with a run in Prow and trace its configuration back to the source:

1. Open the job definition in `test-infra` to identify its commands, source revisions, and environment.
2. Follow those commands into the benchmark script or test scenario to understand the deployment and requests.
3. Open the run artifacts to relate the measurements to that specific execution.

For tests that deploy an etcd image, testing a source change requires an image that contains it.
An etcd pull request alone does not change the image selected by the job.

For a performance comparison, keep the environment, workload, and feature configuration equal between revisions.

[prow-guide]: ../contributor-guide/prow_jobs.md
[benchmark-script]: ../../scripts/benchmark_test.sh
[api-job]: https://github.com/kubernetes/test-infra/blob/master/config/jobs/etcd/etcd-benchmarks-periodic.yaml
[scenario]: https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/testing/etcd/config.yaml
[cluster-jobs]: https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/sig-scalability/sig-scalability-periodic-jobs.yaml
[prow]: https://prow.k8s.io/
[testgrid]: https://testgrid.k8s.io/sig-etcd
[perfdash]: https://perf-dash.k8s.io/
