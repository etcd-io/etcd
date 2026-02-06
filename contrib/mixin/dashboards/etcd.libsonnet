{
  grafanaDashboards+:: if !$._config.grafana7x then {
    local g = import './g.libsonnet',
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
  } else {},
}
