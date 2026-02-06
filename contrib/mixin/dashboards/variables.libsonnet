// variables.libsonnet
local g = import './g.libsonnet';
local var = g.dashboard.variable;


function(config) {
  datasource:
    var.datasource.new('datasource', 'prometheus')
    + var.datasource.generalOptions.withLabel('Data Source'),

  cluster:
    var.query.new('cluster')
    + var.query.generalOptions.withLabel('cluster')
    + var.query.withDatasourceFromVariable(self.datasource)
    + { refresh: config.dashboard_var_refresh }
    + var.query.queryTypes.withLabelValues(
      config.clusterLabel,
      'etcd_server_has_leader{%s}' % [config.etcd_selector]
    ),

}
