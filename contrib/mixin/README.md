# Prometheus Monitoring Mixin for etcd

> NOTE: This project is *alpha* stage. Flags, configuration, behaviour and design may change significantly in following releases.

A set of customisable Prometheus alerts for etcd.

Instructions for use are the same as the [kubernetes-mixin](https://github.com/kubernetes-monitoring/kubernetes-mixin).

## Background

* For more information about monitoring mixins, see this [design doc](https://docs.google.com/document/d/1A9xvzwqnFVSOZ5fD3blKODXfsat5fg6ZhnKu9LK3lB4/edit#).

## Testing alerts

Make sure to have [jsonnet](https://jsonnet.org/) and [gojsontoyaml](https://github.com/brancz/gojsontoyaml) installed.

First compile the mixin to a YAML file, which the promtool will read:
```
jsonnet -e '(import "mixin.libsonnet").prometheusAlerts' | gojsontoyaml > mixin.yaml
```

Then run the unit test:
```
promtool test rules test.yaml
```
