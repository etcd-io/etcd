## Metrics

**NOTE: The metrics feature is considered as an experimental. We might add/change/remove metrics without warning in the future releases.**

etcd uses [Prometheus](http://prometheus.io/) for metrics reporting in the server. The metrics can be used for real-time monitoring and debugging.

The simplest way to see the available metrics is to cURL the metrics endpoint `/metrics` of etcd. The format is described [here](http://prometheus.io/docs/instrumenting/exposition_formats/).


You can also follow the doc [here](http://prometheus.io/docs/introduction/getting_started/) to start a Promethus server and monitor etcd metrics.

The naming of metrics follows the suggested [best practice of Promethus](http://prometheus.io/docs/practices/naming/). A metric name has an `etcd` prefix as its namespace and a subsystem prefix (for example `wal` and `etcdserver`).

etcd now exposes the following metrics:

### etcdserver

| Name                                    | Description                                      | Type    |
|-----------------------------------------|--------------------------------------------------|---------|
| file_descriptors_used_total             | The total number of file descriptors used        | Gauge   |
| proposal_durations_milliseconds         | The latency distributions of committing proposal | Summary |
| pending_proposal_total                  | The total number of pending proposals            | Gauge   |
| proposal_failed_total                   | The total number of failed proposals             | Counter |

High file descriptors (`file_descriptors_used_total`) usage (near the file descriptors limitation of the process) indicates a potential out of file descriptors issue. That might cause etcd fails to create new WAL files and panics.

[Proposal](glossary.md#proposal) durations (`proposal_durations_milliseconds`) give you an summary about the proposal commit latency. Latency can be introduced into this process by network and disk IO.

Pending proposal (`pending_proposal_total`) gives you an idea about how many proposal are in the queue and waiting for commit. An increasing pending number indicates a high client load or an unstable cluster.

Failed proposals (`proposal_failed_total`) are normally related to two issues: temporary failures related to a leader election or longer duration downtime caused by a loss of quorum in the cluster.


### store

These metrics describe the accesses into the data store of etcd members that exist in the cluster. They 
are useful to count what kind of actions are taken by users. It is also useful to see and whether all etcd members 
"see" the same set of data mutations, and whether reads and watches (which are local) are equally distributed.

All these metrics are prefixed with `etcd_store_`. 

| Name                      | Description                                                                          | Type                   |
|---------------------------|------------------------------------------------------------------------------------------|--------------------|
| reads_total               | Total number of reads from store, should differ among etcd members (local reads).    | Counter(action)        |
| writes_total              | Total number of writes to store, should be same among all etcd members.              | Counter(action)        |
| reads_failed_total        | Number of failed reads from store (e.g. key missing) on local reads.                 | Counter(action)        |
| writes_failed_total     | Number of failed writes to store (e.g. failed compare and swap).                       | Counter(action)        |
| expires_total             | Total number of expired keys (due to TTL).                                           | Counter                |
| watch_requests_totals     | Total number of incoming watch requests to this etcd member (local watches).         | Counter                | 
| watchers                  | Current count of active watchers on this etcd member.                                | Gauge                  |

Both `reads_total` and `writes_total` count both successful and failed requests. `reads_failed_total` and 
`writes_failed_total` count failed requests. A lot of failed writes indicate possible contentions on keys (e.g. when 
doing  `compareAndSet`), and read failures indicate that some clients try to access keys that don't exist.

Example Prometheus queries that may be useful from these metrics (across all etcd members):

 *  `sum(rate(etcd_store_reads_total{job="etcd"}[1m])) by (action)`
    `max(rate(etcd_store_writes_total{job="etcd"}[1m])) by (action)`
    
    Rate of reads and writes by action, across all servers across a time window of `1m`. The reason why `max` is used
     for writes as opposed to `sum` for reads is because all of etcd nodes in the cluster apply all writes to their stores.
    Shows the rate of successfull readonly/write queries across all servers, across a time window of `1m`.
 * `sum(rate(etcd_store_watch_requests_total{job="etcd"}[1m]))`
    
    Shows rate of new watch requests per second. Likely driven by how often watched keys change. 
 * `sum(etcd_store_watchers{job="etcd"})`
    
    Number of active watchers across all etcd servers.        


### wal

| Name                               | Description                                      | Type    |
|------------------------------------|--------------------------------------------------|---------|
| fsync_durations_microseconds       | The latency distributions of fsync called by wal | Summary |
| last_index_saved                   | The index of the last entry saved by wal         | Gauge   |

Abnormally high fsync duration (`fsync_durations_microseconds`) indicates disk issues and might cause the cluster to be unstable.

### snapshot

| Name                                       | Description                                                | Type    |
|--------------------------------------------|------------------------------------------------------------|---------|
| snapshot_save_total_durations_microseconds | The total latency distributions of save called by snapshot | Summary |

Abnormally high snapshot duration (`snapshot_save_total_durations_microseconds`) indicates disk issues and might cause the cluster to be unstable.


### rafthttp

| Name                              | Description                                | Type    | Labels                         |
|-----------------------------------|--------------------------------------------|---------|--------------------------------|
| message_sent_latency_microseconds | The latency distributions of messages sent | Summary | sendingType, msgType, remoteID |
| message_sent_failed_total         | The total number of failed messages sent   | Summary | sendingType, msgType, remoteID |


Abnormally high message duration (`message_sent_latency_microseconds`) indicates network issues and might cause the cluster to be unstable.

An increase in message failures (`message_sent_failed_total`) indicates more severe network issues and might cause the cluster to be unstable.

Label `sendingType` is the connection type to send messages. `message`, `msgapp` and `msgappv2` use HTTP streaming, while `pipeline` does HTTP request for each message.

Label `msgType` is the type of raft message. `MsgApp` is log replication message; `MsgSnap` is snapshot install message; `MsgProp` is proposal forward message; the others are used to maintain raft internal status. If you have a large snapshot, you would expect a long msgSnap sending latency. For other types of messages, you would expect low latency, which is comparable to your ping latency if you have enough network bandwidth.

Label `remoteID` is the member ID of the message destination.


### proxy

etcd members operating in proxy mode do not do store operations. They forward all requests
 to cluster instances.

Tracking the rate of requests coming from a proxy allows one to pin down which machine is performing most reads/writes.

All these metrics are prefixed with `etcd_proxy_`

| Name                      | Description                                                                         | Type                   |
|---------------------------|-----------------------------------------------------------------------------------------|--------------------|
| requests_total            | Total number of requests by this proxy instance.    .                               | Counter(method)        |
| handled_total             | Total number of fully handled requests, with responses from etcd members.           | Counter(method)        |
| dropped_total             | Total number of dropped requests due to forwarding errors to etcd members.          | Counter(method,error)  |
| handling_duration_seconds | Bucketed handling times by HTTP method, including round trip to member instances.   | Histogram(method)      |  

Example Prometheus queries that may be useful from these metrics (across all etcd servers):

 *  `sum(rate(etcd_proxy_handled_total{job="etcd"}[1m])) by (method)`
    
    Rate of requests (by HTTP method) handled by all proxies, across a window of `1m`. 
 * `histogram_quantile(0.9, sum(increase(etcd_proxy_events_handling_time_seconds_bucket{job="etcd",method="GET"}[5m])) by (le))`
   `histogram_quantile(0.9, sum(increase(etcd_proxy_events_handling_time_seconds_bucket{job="etcd",method!="GET"}[5m])) by (le))`
    
    Show the 0.90-tile latency (in seconds) of handling of user requestsacross all proxy machines, with a window of `5m`.  
 * `sum(rate(etcd_proxy_dropped_total{job="etcd"}[1m])) by (proxying_error)`
    
    Number of failed request on the proxy. This should be 0, spikes here indicate connectivity issues to etcd cluster.
            