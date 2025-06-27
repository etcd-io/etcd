## Overview

This is still Work In Progress.

Go tests in this directory analyze the usage of Etcd API based on collected
traces from a running cluster. They will output information on:

1. Traces that **did go** through the contract interface:
   1. Get (split by: `Revision` set)
   1. OptimisticPut (split by: `ExpectedRevision` set, `Lease` set, `GetOnFailure` set)
   1. OptimisticDelete (split by: `ExpectedRevision` set, `GetOnFailure` set)
   1. List (split by: `Revision` set, `Limit` set or one or zero)
1. Traces that **did not go** through the contract interface:
   1. Known uses of the interface within `KV`:
      1. Healthchecks (reads to `/registry/health`)
      1. Counts
      1. Compact checks (i.e. reads that use `compact_rev_key`)
      1. Consistent Reads
   1. Known uses of the interface outside `KV`:
      1. Calls to `Lease/LeaseGrant` (used for master leases)
      1. Calls to `Maintenance/Status` (used to get information on database size)
      1. Calls to `Watch/Watch` (as it is not part of the contract interface yet)

### Manual test execution

At first we will manually set up the cluster, run e2e tests, download traces and
then execute the test.

0\. The commands from 1\. to 3\. below are expected to be run in a `kubernetes`
repository patched with
https://github.com/AwesomePatrol/kubernetes/commit/5a18a66c06bad4350fb1fd52b3ca50ab885dbbf1
or
https://github.com/AwesomePatrol/kubernetes/tree/add-kubernetes-etcd-contract-tracker

```
git clone https://github.com/AwesomePatrol/kubernetes -b add-kubernetes-etcd-contract-tracker --depth 1
cd kubernetes
```

1\. Setup [KIND
cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) with
tracing exporting to [Jaeger](https://www.jaegertracing.io/)

```
mkdir -p apiserver-conf
cat <<EOF > apiserver-conf/tracing.yaml
apiVersion: apiserver.config.k8s.io/v1beta1
kind: TracingConfiguration
samplingRatePerMillion: 1000000
EOF
cat <<EOF > kind-with-tracing.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: ipv4
featureGates:
  "APIServerTracing": true
  "AllBeta": true
runtimeConfig:
  "api/all": "true"
nodes:
- role: control-plane
  extraMounts:
  - hostPath: apiserver-conf/
    containerPath: /shared-to-cp
    readOnly: true
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    etcd:
      local:
        extraArgs:
          experimental-enable-distributed-tracing: "true"
          experimental-distributed-tracing-address: "0.0.0.0:4317"
          experimental-distributed-tracing-service-name: "etcd"
    apiServer:
        extraArgs:
          tracing-config-file: "/shared-to-cp/tracing.yaml"
        extraVolumes:
          - name: tracing
            hostPath: "/shared-to-cp"
            mountPath: "/shared-to-cp"
- role: worker
- role: worker
EOF
export KUBECONFIG="$(pwd)/kind-with-tracing-config"
kind build node-image
kind create cluster --config kind-with-tracing.yaml --image kindest/node:latest
kubectl run jaeger --overrides='{ "apiVersion": "v1", "spec": { "hostNetwork": true, "nodeName": "kind-control-plane", "tolerations": [{"effect": "NoExecute", "operator": "Exists"}]} }' --labels='tier=control-plane' --image jaegertracing/jaeger:2.6.0
```

2\. Build and run Conformance tests (this will take a while):

```
export KUBECONFIG="$(pwd)/kind-with-tracing-config"
make WHAT="test/e2e/e2e.test"
./_output/bin/e2e.test -context kind-kind -ginkgo.focus=".*Conformance" -num-nodes 2
```

3\. Download traces:

```
kubectl port-forward jaeger --address 0.0.0.0 --address :: 16686:16686 &
curl --retry 10 -v -o all_traces.json "http://127.0.0.1:16686/api/traces?limit=0&lookback=2d&service=etcd&service=apiserver"
```

4\. Copy `all_traces.json` to `testdata` directory (`tests/robustness/coverage/testdata` in Etcd repository)

```
mkdir -p tests/robustness/coverage/testdata
cp all_traces.json tests/robustness/coverage/testdata/
```

5\. Run Go test

```
go test -v -timeout 60s -run ^TestInterfaceUse$ go.etcd.io/etcd/tests/v3/robustness/coverage
```

### Automated test execution

In the future all steps above will be automated and executed periodically. This
is tracked in https://github.com/etcd-io/etcd/issues/20182
