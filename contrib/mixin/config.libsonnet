{

  _config+:: {

    // set to true if dashboards should be compatible with Grafana 7x or earlier
    grafana7x: false,

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
}
