local g = import './g.libsonnet';
local prometheusQuery = g.query.prometheus;

function(variables, config) {
  up:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(etcd_server_has_leader{%s, %s="$cluster"})' % [config.etcd_selector, config.clusterLabel]
    )
    + prometheusQuery.withLegendFormat(|||
      {{cluster}} - {{namespace}}
    |||),

  rpcRate:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(rate(grpc_server_started_total{%s, %s="$cluster",grpc_type="unary"}[$__rate_interval]))' % [config.etcd_selector, config.clusterLabel]
    )
    + prometheusQuery.withLegendFormat('RPC rate'),
  rpcFailedRate:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(rate(grpc_server_handled_total{%s, %s="$cluster",grpc_type="unary",grpc_code=~"Unknown|FailedPrecondition|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded"}[$__rate_interval]))' % [config.etcd_selector, config.clusterLabel]
    )
    + prometheusQuery.withLegendFormat('RPC failed rate'),
  watchStreams:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(grpc_server_started_total{%(etcd_selector)s,%(clusterLabel)s="$cluster",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"}) - sum(grpc_server_handled_total{%(clusterLabel)s="$cluster",grpc_service="etcdserverpb.Watch",grpc_type="bidi_stream"})' % config
    )
    + prometheusQuery.withLegendFormat('Watch streams'),
  leaseStreams:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(grpc_server_started_total{%(etcd_selector)s,%(clusterLabel)s="$cluster",grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"}) - sum(grpc_server_handled_total{%(clusterLabel)s="$cluster",grpc_service="etcdserverpb.Lease",grpc_type="bidi_stream"})' % config
    )
    + prometheusQuery.withLegendFormat('Lease streams'),
  dbSize:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'etcd_mvcc_db_total_size_in_bytes{%s, %s="$cluster"}' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} DB size'),
  walFsync:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket{%s, %s="$cluster"}[$__rate_interval])) by (instance, le))' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} WAL fsync'),
  dbFsync:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'histogram_quantile(0.99, sum(rate(etcd_disk_backend_commit_duration_seconds_bucket{%s, %s="$cluster"}[$__rate_interval])) by (instance, le))' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} DB fsync'),
  memory:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'process_resident_memory_bytes{%s, %s="$cluster"}' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} resident memory'),
  clientTrafficIn:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'rate(etcd_network_client_grpc_received_bytes_total{%s, %s="$cluster"}[$__rate_interval])' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} client traffic in'),
  clientTrafficOut:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'rate(etcd_network_client_grpc_sent_bytes_total{%s, %s="$cluster"}[$__rate_interval])' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} client traffic out'),
  peerTrafficIn:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(rate(etcd_network_peer_received_bytes_total{%s, %s="$cluster"}[$__rate_interval])) by (instance)' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} peer traffic in'),
  peerTrafficOut:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'sum(rate(etcd_network_peer_sent_bytes_total{%s, %s="$cluster"}[$__rate_interval])) by (instance)' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} peer traffic out'),
  raftProposals:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'changes(etcd_server_leader_changes_seen_total{%s, %s="$cluster"}[1d])' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} total leader elections per day'),
  leaderElections:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'changes(etcd_server_leader_changes_seen_total{%s, %s="$cluster"}[1d])' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} total leader elections per day'),
  peerRtt:
    prometheusQuery.new(
      '$' + variables.datasource.name,
      'histogram_quantile(0.99, sum by (instance, le) (rate(etcd_network_peer_round_trip_time_seconds_bucket{%s, %s="$cluster"}[$__rate_interval])))' % [config.etcd_selector, config.clusterLabel],
    )
    + prometheusQuery.withLegendFormat('{{instance}} peer round trip time'),
}
