{
  _config+:: {
    etcd_selector: 'job=~".*etcd.*"',
  },

  prometheusAlerts+:: {
    groups+: [
      {
        name: "etcd",
        rules: [
          {
            alert: "EtcdInsufficientMembers",
            expr: |||
              count(up{%(etcd_selector)s} == 0) by (job) > (count(up{%(etcd_selector)s}) by (job) / 2 - 1)
            ||| % $._config,
            "for": "3m",
            labels: {
              severity: "critical",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": insufficient members ({{ $value }}).',
            },
          },
          {
            alert: "EtcdNoLeader",
            expr: |||
              etcd_server_has_leader{%(etcd_selector)s} == 0
            ||| % $._config,
            "for": "1m",
            labels: {
              severity: "critical",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": member {{ $labels.instance }} has no leader.',
            },
          },
          {
            alert: "EtcdHighNumberOfLeaderChanges",
            expr: |||
              rate(etcd_server_leader_changes_seen_total{%(etcd_selector)s}[15m]) > 3
            ||| % $._config,
            "for": "15m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": instance {{ $labels.instance }} has seen {{ $value }} leader changes within the last hour.',
            },
          },
          {
            alert: "EtcdHighNumberOfFailedGRPCRequests",
            expr: |||
              100 * sum(rate(grpc_server_handled_total{%(etcd_selector)s, grpc_code!="OK"}[5m])) BY (job, instance, grpc_service, grpc_method)
                /
              sum(rate(grpc_server_handled_total{%(etcd_selector)s}[5m])) BY (job, instance, grpc_service, grpc_method)
                > 1
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": {{ $value }}% of requests for {{ $labels.grpc_method }} failed on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHighNumberOfFailedGRPCRequests",
            expr: |||
              100 * sum(rate(grpc_server_handled_total{%(etcd_selector)s, grpc_code!="OK"}[5m])) BY (job, instance, grpc_service, grpc_method)
                /
              sum(rate(grpc_server_handled_total{%(etcd_selector)s}[5m])) BY (job, instance, grpc_service, grpc_method)
                > 5
            ||| % $._config,
            "for": "5m",
            labels: {
              severity: "critical",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": {{ $value }}% of requests for {{ $labels.grpc_method }} failed on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdGRPCRequestsSlow",
            expr: |||
              histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket{%(etcd_selector)s, grpc_type="unary"}[5m])) by (job, instance, grpc_service, grpc_method, le))
              > 0.15
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "critical",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": gRPC requests to {{ $labels.grpc_method }} are taking {{ $value }}s on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHighNumberOfFailedHTTPRequests",
            expr: |||
              100 * sum(rate(etcd_http_failed_total{%(etcd_selector)s}[5m])) BY (job, instance, method)
                /
              sum(rate(etcd_http_received_total{%(etcd_selector)s}[5m])) BY (job, instance, method)
                > 1
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": {{ $value }}%% of requests for {{ $labels.method }} failed on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHighNumberOfFailedHTTPRequests",
            expr: |||
              100 * sum(rate(etcd_http_failed_total{%(etcd_selector)s}[5m])) BY (job, instance, method)
                /
              sum(rate(etcd_http_received_total{%(etcd_selector)s}[5m])) BY (job, instance, method)
                > 5
            ||| % $._config,
            "for": "5m",
            labels: {
              severity: "critical",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": {{ $value }}%% of requests for {{ $labels.method }} failed on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHTTPRequestsSlow",
            expr: |||
              histogram_quantile(0.99, rate(etcd_http_successful_duration_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.15
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": HTTP requests to {{ $labels.method }} are taking {{ $value }} on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdMemberCommunicationSlow",
            expr: |||
              histogram_quantile(0.99, rate(etcd_network_peer_round_trip_time_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.15
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": member communication with {{ $labels.To }} is taking {{ $value }}s on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHighNumberOfFailedProposals",
            expr: |||
              rate(etcd_server_proposals_failed_total{%(etcd_selector)s}[15m]) > 5
            ||| % $._config,
            "for": "15m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": {{ $value }} proposal failures within the last hour on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHighFsyncDurations",
            expr: |||
              histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.5
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": 99th percentile fync durations are {{ $value }}s on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            alert: "EtcdHighCommitDurations",
            expr: |||
              histogram_quantile(0.99, rate(etcd_disk_backend_commit_duration_seconds_bucket{%(etcd_selector)s}[5m]))
              > 0.25
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: 'Etcd cluster "{{ $labels.job }}": 99th percentile commit durations {{ $value }}s on etcd instance {{ $labels.instance }}.',
            },
          },
          {
            record: "instance:fd_utilization",
            expr: "process_open_fds / process_max_fds",
          },
          {
            alert: "FdExhaustionClose",
            expr: |||
              predict_linear(instance:fd_utilization{%(etcd_selector)s}[1h], 3600 * 4) > 1
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "warning",
            },
            annotations: {
              message: '{{ $labels.job }} instance {{ $labels.instance }} will exhaust its file descriptors soon',
            },
          },
          {
            alert: "FdExhaustionClose",
            expr: |||
              predict_linear(instance:fd_utilization{%(etcd_selector)s}[10m], 3600) > 1
            ||| % $._config,
            "for": "10m",
            labels: {
              severity: "critical",
            },
            annotations: {
              description: '{{ $labels.job }} instance {{ $labels.instance }} will exhaust its file descriptors soon',
            },
          }
        ],
      },
    ],
  },
}
