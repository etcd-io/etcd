# Metrics

etcd uses [Prometheus][prometheus] for metrics reporting. The metrics can be used for real-time monitoring and debugging. etcd does not persist its metrics; if a member restarts, the metrics will be reset.

The simplest way to see the available metrics is to cURL the metrics endpoint `/metrics`. The format is described [here](http://prometheus.io/docs/instrumenting/exposition_formats/).

Follow the [Prometheus getting started doc][prometheus-getting-started] to spin up a Prometheus server to collect etcd metrics.

The naming of metrics follows the suggested [Prometheus best practices][prometheus-naming]. A metric name has an `etcd` or `etcd_debugging` prefix as its namespace and a subsystem prefix (for example `wal` and `etcdserver`).

## etcd namespace metrics

The metrics under the `etcd` prefix are for monitoring and alerting. They are stable high level metrics. If there is any change of these metrics, it will be included in release notes.

Metrics that are etcd2 related are documented [here][v2-http-metrics].

### gRPC requests

These metrics describe the requests served by a specific etcd member: total received requests, total failed requests, and processing latency. They are useful for tracking user-generated traffic hitting the etcd cluster.

All these metrics are prefixed with `etcd_grpc_`

| Name                           | Description                                                                         | Type                   |
|--------------------------------|-------------------------------------------------------------------------------------|------------------------|
| requests_total                 | Total number of received requests                                                   | Counter(method)        |
| requests_failed_total                   | Total number of failed requests.                                                    | Counter(method,error)  |
| unary_requests_duration_seconds     | Bucketed handling duration of the requests.                                         | Histogram(method)      |


Example Prometheus queries that may be useful from these metrics (across all etcd members):
 
 * `sum(rate(etcd_grpc_requests_failed_total{job="etcd"}[1m]) by (grpc_method) / sum(rate(etcd_grpc_total{job="etcd"})[1m]) by (grpc_method)` 
    
    Shows the fraction of events that failed by gRPC method across all members, across a time window of `1m`.
 
 * `sum(rate(etcd_grpc_requests_total{job="etcd",grpc_method="PUT"})[1m]) by (grpc_method)`
    
    Shows the rate of PUT requests across all members, across a time window of `1m`.
    
 * `histogram_quantile(0.9, sum(rate(etcd_grpc_unary_requests_duration_seconds{job="etcd",grpc_method="PUT"}[5m]) ) by (le))`
    
    Show the 0.90-tile latency (in seconds) of PUT request handling across all members, with a window of `5m`.      

## etcd_debugging namespace metrics

The metrics under the `etcd_debugging` prefix are for debugging. They are very implementation dependent and volatile. They might be changed or removed without any warning in new etcd releases. Some of the metrics might be moved to the `etcd` prefix when they become more stable.

### etcdserver

| Name                                    | Description                                      | Type      |
|-----------------------------------------|--------------------------------------------------|-----------|
| proposal_duration_seconds              | The latency distributions of committing proposal | Histogram |
| proposals_pending                       | The current number of pending proposals          | Gauge     |
| proposals_failed_total                   | The total number of failed proposals             | Counter   |

[Proposal][glossary-proposal] duration (`proposal_duration_seconds`) provides a proposal commit latency histogram. The reported latency reflects network and disk IO delays in etcd.

Proposals pending (`proposals_pending`) indicates how many proposals are queued for commit. Rising pending proposals suggests there is a high client load or the cluster is unstable.

Failed proposals (`proposals_failed_total`) are normally related to two issues: temporary failures related to a leader election or longer duration downtime caused by a loss of quorum in the cluster.

### wal

| Name                               | Description                                      | Type      |
|------------------------------------|--------------------------------------------------|-----------|
| fsync_duration_seconds            | The latency distributions of fsync called by wal | Histogram |
| last_index_saved                   | The index of the last entry saved by wal         | Gauge     |

Abnormally high fsync duration (`fsync_duration_seconds`) indicates disk issues and might cause the cluster to be unstable.

### snapshot

| Name                                       | Description                                                | Type      |
|--------------------------------------------|------------------------------------------------------------|-----------|
| snapshot_save_total_duration_seconds      | The total latency distributions of save called by snapshot | Histogram |

Abnormally high snapshot duration (`snapshot_save_total_duration_seconds`) indicates disk issues and might cause the cluster to be unstable.

### rafthttp

| Name                              | Description                                | Type         | Labels                         |
|-----------------------------------|--------------------------------------------|--------------|--------------------------------|
| message_sent_latency_seconds      | The latency distributions of messages sent | HistogramVec | sendingType, msgType, remoteID |
| message_sent_failed_total         | The total number of failed messages sent   | Summary      | sendingType, msgType, remoteID |


Abnormally high message duration (`message_sent_latency_seconds`) indicates network issues and might cause the cluster to be unstable.

An increase in message failures (`message_sent_failed_total`) indicates more severe network issues and might cause the cluster to be unstable.

Label `sendingType` is the connection type to send messages. `message`, `msgapp` and `msgappv2` use HTTP streaming, while `pipeline` does HTTP request for each message.

Label `msgType` is the type of raft message. `MsgApp` is log replication messages; `MsgSnap` is snapshot install messages; `MsgProp` is proposal forward messages; the others maintain internal raft status. Given large snapshots, a lengthy msgSnap transmission latency should be expected. For other types of messages, given enough network bandwidth, latencies comparable to ping latency should be expected.

Label `remoteID` is the member ID of the message destination.

## Prometheus supplied metrics

The Prometheus client library provides a number of metrics under the `go` and `process` namespaces. There are a few that are particlarly interesting.

| Name                              | Description                                | Type         |
|-----------------------------------|--------------------------------------------|--------------|
| process_open_fds                  | Number of open file descriptors.           | Gauge        |
| process_max_fds                   | Maximum number of open file descriptors.   | Gauge        |

Heavy file descriptor (`process_open_fds`) usage (i.e., near the process's file descriptor limit, `process_max_fds`) indicates a potential file descriptor exhaustion issue. If the file descriptors are exhausted, etcd may panic because it cannot create new WAL files.

[glossary-proposal]: learning/glossary.md#proposal
[prometheus]: http://prometheus.io/
[prometheus-getting-started]: http://prometheus.io/docs/introduction/getting_started/
[prometheus-naming]: http://prometheus.io/docs/practices/naming/
[v2-http-metrics]: v2/metrics.md#http-requests
