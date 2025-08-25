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
cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) with
tracing exporting to [Jaeger](https://www.jaegertracing.io/)

   1. Patch and build kubernetes images

      ```shell
      git -C "$KUBERNETES_REPO" apply --recount ${PWD}/patches/kubernetes/*
      kind build node-image
      ```

   1. Create docker network:

      ```shell
      docker network create kind-with-external-etcd \
        --driver bridge \
        --gateway "192.168.32.1" \
        --subnet "192.168.32.0/24"
      ```

      Note: You will need to adjust the configuration files and commands if you
      choose a different subnet.

   1. Run [Jaeger](https://www.jaegertracing.io/) container:

      ```shell
      docker run --rm --name jaeger \
        -p 16686:16686 \
        -p 4317:4317 \
        jaegertracing/jaeger:2.6.0 --set=extensions.jaeger_storage.backends.some_storage.memory.max_traces=20000000
      ```

   1. Run `etcd` (in root of `etcd` repository):

      ```shell
      make build
      cp bin/etcd "${KUBERNETES_REPO}/third_party/etcd/etcd"
      bin/etcd --watch-progress-notify-interval=5s \
        --listen-client-urls http://192.168.32.1:2379 \
        --advertise-client-urls http://192.168.32.1:2379 \
        --enable-distributed-tracing \
        --distributed-tracing-address="192.168.32.1:4317" \
        --distributed-tracing-service-name="etcd" \
        --distributed-tracing-sampling-rate=1000000
      ```

   1. Create [KIND
cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#installation):

      ```shell
      export KIND_EXPERIMENTAL_DOCKER_NETWORK=kind-with-external-etcd
      kind create cluster --config kind-with-tracing.yaml --name kind-with-external-etcd --image kindest/node:latest
      ```

1. Exercise Kubernetes API. For example, build and run Conformance tests from
Kubernetes repository (this usually takes 30-40m or will time out after 1 hour):

```shell
make WHAT="test/e2e/e2e.test"
./_output/bin/e2e.test \
  -context kind-kind-with-external-etcd \
  -ginkgo.focus="\[sig-apps\].*Conformance" \
  -num-nodes 2
build/run.sh make test-cmd
```

1. Download traces and put them into `tests/robustness/coverage/testdata`
directory in Etcd git repository:

```shell
curl -v --get --retry 10 --retry-connrefused -o testdata/demo_traces.json \
  -H "Content-Type: application/json" \
  --data-urlencode "query.start_time_min=$(date --date="5 days ago" -Ins)" \
  --data-urlencode "query.start_time_max=$(date --date="2 minutes ago" -Ins)" \
  --data-urlencode "query.service_name=etcd" \
  "http://192.168.32.1:16686/api/v3/traces"
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

### Automated test execution

Work on improving these tests is tracked in [#20182](https://github.com/etcd-io/etcd/issues/20182).
