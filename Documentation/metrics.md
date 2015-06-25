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

| Name                              | Description                                | Type    | Labels                     |
|-----------------------------------|--------------------------------------------|---------|----------------------------|
| message_sent_latency_microseconds | The latency distributions of messages sent | Summary | channel, msgType, remoteID |
| message_sent_failed_total         | The total number of failed messages sent   | Summary | channel, msgType, remoteID |


Abnormally high message duration (`message_sent_latency_microseconds`) indicates network issues and might cause the cluster to be unstable.

An increase in message failures (`message_sent_failed_total`) indicates more severe network issues and might cause the cluster to be unstable.

Label `channel` is the channel to send message. `message`, `msgapp` and `msgappv2` channels use HTTP streaming, while `pipeline` does HTTP request for each message.

Label `msgType` is the type of raft message. `MsgApp` is log replication message; `MsgSnap` is snapshot install message; `MsgProp` is proposal forward message; the others are used to maintain raft internal status. If you have a large snapshot, you would expect a long msgSnap sending latency. For other types of messages, you would expect low latency, which is comparable to your ping latency if you have enough network bandwidth.

Label `remoteID` is the member ID of the message destination.
