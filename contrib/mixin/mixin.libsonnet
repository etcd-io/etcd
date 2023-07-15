{
  _config+:: {
    etcd_selector: 'job=~".*etcd.*"',
    // etcd_instance_labels are the label names that are uniquely
    // identifying an instance and need to be aggreated away for alerts
    // that are about an etcd cluster as a whole. For example, if etcd
    // instances are deployed on K8s, you will likely want to change
    // this to 'instance, pod'.
    etcd_instance_labels: 'instance',
    // scrape_interval_seconds is the global scrape interval which can be
    // used to dynamically adjust rate windows as a function of the interval.
    scrape_interval_seconds: 30,
    // Dashboard variable refresh option on Grafana (https://grafana.com/docs/grafana/latest/datasources/prometheus/).
    // 0 : Never (Will never refresh the Dashboard variables values)
    // 1 : On Dashboard Load  (Will refresh Dashboards variables when dashboard are loaded)
    // 2 : On Time Range Change (Will refresh Dashboards variables when time range will be changed)
    dashboard_var_refresh: 2,
    // clusterLabel is used to identify a cluster.
    clusterLabel: 'job',
  },

  prometheusAlerts+:: {
    groups+: [
      {
        name: 'etcd',
        rules: [
          {
            alert: 'etcdMembersDown',
            expr: |||
              max without (endpoint) (
                sum without (%(etcd_instance_labels)s) (up{%(etcd_selector)s} == bool 0)
              or
                count without (To) (
                  sum without (%(etcd_instance_labels)s) (rate(etcd_network_peer_sent_failures_total{%(etcd_selector)s}[%(network_failure_range)ss])) > 0.01
                )
              )
              > 0
            ||| % { etcd_instance_labels: $._config.etcd_instance_labels, etcd_selector: $._config.etcd_selector, network_failure_range: $._config.scrape_interval_seconds * 4 },
            'for': '10m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": members are down ({{ $value }}).' % $._config.clusterLabel,
              summary: 'etcd cluster members are down.',
            },
          },
          {
            alert: 'etcdInsufficientMembers',
            expr: |||
              sum(up{%(etcd_selector)s} == bool 1) without (%(etcd_instance_labels)s) < ((count(up{%(etcd_selector)s}) without (%(etcd_instance_labels)s) + 1) / 2)
            ||| % $._config,
            'for': '3m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": insufficient members ({{ $value }}).' % $._config.clusterLabel,
              summary: 'etcd cluster has insufficient number of members.',
            },
          },
          {
            alert: 'etcdNoLeader',
            expr: |||
              etcd_server_has_leader{%(etcd_selector)s} == 0
            ||| % $._config,
            'for': '1m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": member {{ $labels.instance }} has no leader.' % $._config.clusterLabel,
              summary: 'etcd cluster has no leader.',
            },
          },
          {
            alert: 'etcdHighNumberOfLeaderChanges',
            expr: |||
              increase((max without (%(etcd_instance_labels)s) (etcd_server_leader_changes_seen_total{%(etcd_selector)s}) or 0*absent(etcd_server_leader_changes_seen_total{%(etcd_selector)s}))[15m:1m]) >= 4
            ||| % $._config,
            'for': '5m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": {{ $value }} leader changes within the last 15 minutes. Frequent elections may be a sign of insufficient resources, high network latency, or disruptions by other components and should be investigated.' % $._config.clusterLabel,
              summary: 'etcd cluster has high number of leader changes.',
            },
          },
          {
            alert: 'etcdHighNumberOfFailedGRPCRequests',
            expr: |||
              100 * sum(rate(grpc_server_handled_total{%(etcd_selector)s, grpc_code=~"Unknown|FailedPrecondition|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded"}[5m])) without (grpc_type, grpc_code)
                /
              sum(rate(grpc_server_handled_total{%(etcd_selector)s}[5m])) without (grpc_type, grpc_code)
                > 1
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": {{ $value }}%% of requests for {{ $labels.grpc_method }} failed on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster has high number of failed grpc requests.',
            },
          },
          {
            alert: 'etcdHighNumberOfFailedGRPCRequests',
            expr: |||
              100 * sum(rate(grpc_server_handled_total{%(etcd_selector)s, grpc_code=~"Unknown|FailedPrecondition|ResourceExhausted|Internal|Unavailable|DataLoss|DeadlineExceeded"}[5m])) without (grpc_type, grpc_code)
                /
              sum(rate(grpc_server_handled_total{%(etcd_selector)s}[5m])) without (grpc_type, grpc_code)
                > 5
            ||| % $._config,
            'for': '5m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": {{ $value }}%% of requests for {{ $labels.grpc_method }} failed on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster has high number of failed grpc requests.',
            },
          },
          {
            alert: 'etcdGRPCRequestsSlow',
            expr: |||
              histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket{%(etcd_selector)s, grpc_method!="Defragment", grpc_type="unary"}[5m])) without(grpc_type))
              > 0.15
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": 99th percentile of gRPC requests is {{ $value }}s on etcd instance {{ $labels.instance }} for {{ $labels.grpc_method }} method.' % $._config.clusterLabel,
              summary: 'etcd grpc requests are slow',
            },
          },
          {
            alert: 'etcdMemberCommunicationSlow',
            expr: |||
              histogram_quantile(0.99, rate(etcd_network_peer_round_trip_time_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.15
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": member communication with {{ $labels.To }} is taking {{ $value }}s on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster member communication is slow.',
            },
          },
          {
            alert: 'etcdHighNumberOfFailedProposals',
            expr: |||
              rate(etcd_server_proposals_failed_total{%(etcd_selector)s}[15m]) > 5
            ||| % $._config,
            'for': '15m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": {{ $value }} proposal failures within the last 30 minutes on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster has high number of proposal failures.',
            },
          },
          {
            alert: 'etcdHighFsyncDurations',
            expr: |||
              histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.5
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": 99th percentile fsync durations are {{ $value }}s on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster 99th percentile fsync durations are too high.',
            },
          },
          {
            alert: 'etcdHighFsyncDurations',
            expr: |||
              histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket{%(etcd_selector)s}[5m]))
              > 1
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": 99th percentile fsync durations are {{ $value }}s on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster 99th percentile fsync durations are too high.',
            },
          },
          {
            alert: 'etcdHighCommitDurations',
            expr: |||
              histogram_quantile(0.99, rate(etcd_disk_backend_commit_duration_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.25
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": 99th percentile commit durations {{ $value }}s on etcd instance {{ $labels.instance }}.' % $._config.clusterLabel,
              summary: 'etcd cluster 99th percentile commit durations are too high.',
            },
          },
          {
            alert: 'etcdDatabaseQuotaLowSpace',
            expr: |||
              (last_over_time(etcd_mvcc_db_total_size_in_bytes[5m]) / last_over_time(etcd_server_quota_backend_bytes[5m]))*100 > 95
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'critical',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": database size exceeds the defined quota on etcd instance {{ $labels.instance }}, please defrag or increase the quota as the writes to etcd will be disabled when it is full.' % $._config.clusterLabel,
              summary: 'etcd cluster database is running full.',
            },
          },
          {
            alert: 'etcdExcessiveDatabaseGrowth',
            expr: |||
              predict_linear(etcd_mvcc_db_total_size_in_bytes[4h], 4*60*60) > etcd_server_quota_backend_bytes
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": Predicting running out of disk space in the next four hours, based on write observations within the past four hours on etcd instance {{ $labels.instance }}, please check as it might be disruptive.' % $._config.clusterLabel,
              summary: 'etcd cluster database growing very fast.',
            },
          },
          {
            alert: 'etcdDatabaseHighFragmentationRatio',
            expr: |||
              (last_over_time(etcd_mvcc_db_total_size_in_use_in_bytes[5m]) / last_over_time(etcd_mvcc_db_total_size_in_bytes[5m])) < 0.5 and etcd_mvcc_db_total_size_in_use_in_bytes > 104857600
            ||| % $._config,
            'for': '10m',
            labels: {
              severity: 'warning',
            },
            annotations: {
              description: 'etcd cluster "{{ $labels.%s }}": database size in use on instance {{ $labels.instance }} is {{ $value | humanizePercentage }} of the actual allocated disk space, please run defragmentation (e.g. etcdctl defrag) to retrieve the unused fragmented disk space.' % $._config.clusterLabel,
              summary: 'etcd database size in use is less than 50% of the actual allocated storage.',
              runbook_url: 'https://etcd.io/docs/v3.5/op-guide/maintenance/#defragmentation',
            },
          },
        ],
      },
    ],
  },

  grafanaDashboards+:: {
    local g = import 'g.libsonnet',
    local panels = import './panels.libsonnet',
    local variables = import './variables.libsonnet',
    local targets = import './targets.libsonnet',
    local v = variables($._config),
    local t = targets(v, $._config),

    'etcd.json':
      g.dashboard.new('etcd')
      + g.dashboard.withUid(std.md5('etcd.json'))
      + g.dashboard.withRefresh('10s')
      + g.dashboard.time.withFrom('now-15m')
      + g.dashboard.time.withTo('now')
      + g.dashboard.withDescription('etcd sample Grafana dashboard with Prometheus')
      + g.dashboard.withTags(['etcd-mixin'])
      + g.dashboard.withVariables([
        v.datasource,
        v.cluster,
      ])
      + g.dashboard.withPanels(
        [
          panels.stat.up('Up', t.up) { gridPos: { x: 0, h: 7, w: 6, y: 0 } },
          panels.timeSeries.rpcRate('RPC rate', [t.rpcRate, t.rpcFailedRate]) { gridPos: { x: 6, h: 7, w: 10, y: 0 } },
          panels.timeSeries.activeStreams('Active streams', [t.watchStreams, t.leaseStreams]) { gridPos: { x: 16, h: 7, w: 8, y: 0 } },
          panels.timeSeries.dbSize('DB size', [t.dbSize]) { gridPos: { x: 0, h: 7, w: 8, y: 25 } },
          panels.timeSeries.diskSync('Disk sync duration', [t.walFsync, t.dbFsync]) { gridPos: { x: 8, h: 7, w: 8, y: 25 } },
          panels.timeSeries.memory('Memory', [t.memory]) { gridPos: { x: 16, h: 7, w: 8, y: 25 } },
          panels.timeSeries.traffic('Client traffic in', [t.clientTrafficIn]) { gridPos: { x: 0, h: 7, w: 6, y: 50 } },
          panels.timeSeries.traffic('Client traffic out', [t.clientTrafficOut]) { gridPos: { x: 6, h: 7, w: 6, y: 50 } },
          panels.timeSeries.traffic('Peer traffic in', [t.peerTrafficIn]) { gridPos: { x: 12, h: 7, w: 6, y: 50 } },
          panels.timeSeries.traffic('Peer traffic out', [t.peerTrafficOut]) { gridPos: { x: 18, h: 7, w: 6, y: 50 } },
          panels.timeSeries.raftProposals('Raft proposals', [t.raftProposals]) { gridPos: { x: 0, h: 7, w: 8, y: 75 } },
          panels.timeSeries.leaderElections('Total leader elections per day', [t.leaderElections]) { gridPos: { x: 8, h: 7, w: 8, y: 75 } },
          panels.timeSeries.peerRtt('Peer round trip time', [t.peerRtt]) { gridPos: { x: 16, h: 7, w: 8, y: 75 } },
        ]
      ),
  },
}
