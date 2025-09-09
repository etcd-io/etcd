## Overview

Go tests in this directory analyze the usage of Etcd API based on collected
traces from a Kubernetes cluster. They output information on:

1. Number of calls per gRPC method used by Kubernetes
1. Key pattern
1. Arguments provided to KV, Watch and Lease methods

This information can be used to track the coverage of k8s-etcd contract.

### Manual test execution

At first we will manually set up the cluster, run e2e tests, download traces and
then execute the test.

1. Customize and set the environment variables used by the code snippets below:

```shell
# Used for patches, building kind nodes, and running e2e tests.
export KUBERNETES_REPO="$(go env GOPATH)/src/k8s.io/kubernetes"
# Used when creating kind cluster and running e2e tests.
export KUBECONFIG="${KUBERNETES_REPO}/kind-with-tracing-config"
```

1. Set up [KIND
cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation), exercise
Kubernetes API, export traces to [Jaeger](https://www.jaegertracing.io/) and
then download them:

   ```shell
   make k8s-coverage
   ```

1. Run Go test

   ```shell
   go test -v -timeout 60s go.etcd.io/etcd/tests/v3/robustness/coverage
   ```

1. Clean up the environment

   ```shell
   kind delete cluster --name kind-with-external-etcd
   docker network rm kind-with-external-etcd
   ```

### Manual trace collection from robustness tests

1. Run [Jaeger](https://www.jaegertracing.io/) container:

   ```shell
   docker run --rm --name jaeger \
     -p 16686:16686 \
     -p 4317:4317 \
     jaegertracing/jaeger:2.6.0 --set=extensions.jaeger_storage.backends.some_storage.memory.max_traces=20000000
   ```

1. Run robustness tests. For example:

   ```shell
   env \
     TRACING_SERVER_ADDR=localhost:4317 \
     GO_TEST_FLAGS='--timeout 10m --count=1 -v --run "^TestRobustness.*/Kubernetes.*"' \
     make test-robustness
   ```

1. Download traces and put them into `tests/robustness/coverage/testdata`
directory in Etcd git repository:

   ```shell
   curl -v --get --retry 10 --retry-connrefused -o testdata/demo_traces.json \
     -H "Content-Type: application/json" \
     --data-urlencode "query.start_time_min=$(date --date="5 days ago" -Ins)" \
     --data-urlencode "query.start_time_max=$(date -Ins)" \
     --data-urlencode "query.service_name=etcd" \
     "http://localhost:16686/api/v3/traces"
   ```

1. Run Go test

   ```shell
   go test -v -timeout 60s go.etcd.io/etcd/tests/v3/robustness/coverage
   ```

   It will show the coverage of Kubernetes-Etcd surface by robustness tests.:w

### Automated test execution

Work on improving these tests is tracked in
[#20182](https://github.com/etcd-io/etcd/issues/20182) and periodic runs in
[#20642](https://github.com/etcd-io/etcd/issues/20642).
