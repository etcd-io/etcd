# Prometheus Monitoring Mixin for etcd

> NOTE: This project is *alpha* stage. Flags, configuration, behaviour and design may change significantly in following releases.

A customisable set of Grafana dashboard and Prometheus alerts for etcd.

Instructions for use are the same as the [kubernetes-mixin](https://github.com/kubernetes-monitoring/kubernetes-mixin).

## Grafana 7.x support

By default, this mixin generates the dashboard compatible with Grafana 8.x or newer.
To generate dashboard for Grafana 7.x, set in the config.libsonnet:

```
// set to true if dashboards should be compatible with Grafana 7x or earlier
grafana7x: true,
```

## Background

* For more information about monitoring mixins, see this [design doc](https://docs.google.com/document/d/1A9xvzwqnFVSOZ5fD3blKODXfsat5fg6ZhnKu9LK3lB4/edit#).

## Testing alerts

Make sure to have [jsonnet](https://jsonnet.org/) and [gojsontoyaml](https://github.com/brancz/gojsontoyaml) installed. You can fetch it via

```
make tools
```

First compile the mixin to a YAML file, which the promtool will read:
```
make manifests
```

Then run the unit test:
```
promtool test rules test.yaml
```
