local g = import 'g.libsonnet';

{
  stat: {
    local stat = g.panel.stat,
    base(title, targets):
      stat.new(title)
      + stat.queryOptions.withTargets(targets)
      + stat.queryOptions.withInterval('1m'),
    up(title, targets):
      self.base(title, targets)
      + stat.options.withColorMode('none')
      + stat.options.withGraphMode('none')
      + stat.options.reduceOptions.withCalcs([
        'lastNotNull',
      ]),
  },
  timeSeries: {
    local timeSeries = g.panel.timeSeries,
    local fieldOverride = g.panel.timeSeries.fieldOverride,
    local custom = timeSeries.fieldConfig.defaults.custom,
    local defaults = timeSeries.fieldConfig.defaults,
    local options = timeSeries.options,


    base(title, targets):
      timeSeries.new(title)
      + timeSeries.queryOptions.withTargets(targets)
      + timeSeries.queryOptions.withInterval('1m')
      + custom.withLineWidth(2)
      + custom.withFillOpacity(0)
      + custom.withShowPoints('never'),

    rpcRate(title, targets):
      self.base(title, targets)
      + timeSeries.standardOptions.withUnit('ops'),
    activeStreams(title, targets):
      self.base(title, targets),
    dbSize(title, targets):
      self.base(title, targets)
      + timeSeries.standardOptions.withUnit('bytes'),
    diskSync(title, targets):
      self.base(title, targets)
      + timeSeries.standardOptions.withUnit('s'),
    memory(title, targets):
      self.base(title, targets)
      + timeSeries.standardOptions.withUnit('bytes'),
    traffic(title, targets):
      self.base(title, targets)
      + timeSeries.standardOptions.withUnit('Bps'),
    raftProposals(title, targets):
      self.base(title, targets),
    leaderElections(title, targets):
      self.base(title, targets),
    peerRtt(title, targets):
      self.base(title, targets)
      + timeSeries.standardOptions.withUnit('s'),
  },
}
