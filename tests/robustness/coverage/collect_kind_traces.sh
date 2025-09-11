#!/usr/bin/env bash
# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs KIND (Kubernetes IN Docker) cluster to collect traces with
# Jaeger for analysis in robustness/coverage tests.
# It is based on instructions from tests/robustness/coverage/README.md

set -o errexit
set -o nounset
set -o pipefail

# 1. Customize and set the environment variables
export KUBERNETES_REPO="${KUBERNETES_REPO:-$(go env GOPATH)/src/k8s.io/kubernetes}"
export ETCD_REPO="${ETCD_REPO:-$(go env GOPATH)/src/go.etcd.io/etcd}"
export KUBECONFIG="${KUBERNETES_REPO}/kind-with-tracing-config"

echo "Using KUBERNETES_REPO: ${KUBERNETES_REPO}"
echo "Using ETCD_REPO: ${KUBERNETES_REPO}"
echo "Using KUBECONFIG: ${KUBECONFIG}"

echo "Applying patches to Kubernetes repo..."
PATCHES_DIR="${ETCD_REPO}/tests/robustness/coverage/patches/kubernetes"
pushd "${KUBERNETES_REPO}"
git apply --reverse --check "${PATCHES_DIR}/"* || git apply --recount "${PATCHES_DIR}/"*
echo "Building KIND node image..."
kind build node-image
popd

echo "Creating Docker network..."
docker network create kind-with-external-etcd \
  --driver bridge \
  --gateway "192.168.32.1" \
  --subnet "192.168.32.0/24" || echo "Docker network already exists."
rm_docker_network() {
  docker network rm kind-with-external-etcd
}
trap "rm_docker_network" EXIT SIGINT

echo "Starting Jaeger container..."
docker stop jaeger && docker wait jaeger || echo
docker run --rm --detach --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/jaeger:2.6.0 --set=extensions.jaeger_storage.backends.some_storage.memory.max_traces=20000000
stop_jaeger() {
  docker stop jaeger
  rm_docker_network
}
trap "stop_jaeger" EXIT SIGINT

echo "Building and starting etcd..."
pushd "${ETCD_REPO}"
mkdir -p "${KUBERNETES_REPO}/third_party/etcd"
DATA_DIR="$(mktemp -d -p "${DATA_DIR:-/tmp}")"
export DATA_DIR
cp "./bin/etcd" "${KUBERNETES_REPO}/third_party/etcd/etcd"
"./bin/etcd" --watch-progress-notify-interval=5s \
  --data-dir "${DATA_DIR}" \
  --listen-client-urls "http://192.168.32.1:2379" \
  --advertise-client-urls "http://192.168.32.1:2379" \
  --enable-distributed-tracing \
  --distributed-tracing-address="192.168.32.1:4317" \
  --distributed-tracing-service-name="etcd" \
  --distributed-tracing-sampling-rate=1000000 &
ETCD_PID=$!
echo "etcd started with PID: ${ETCD_PID}"
popd
stop_etcd() {
  kill "${ETCD_PID}"
  rm -rf "${DATA_DIR}"
  stop_jaeger
}
trap "stop_etcd" EXIT SIGINT

echo "Creating KIND cluster..."
export KIND_EXPERIMENTAL_DOCKER_NETWORK=kind-with-external-etcd
kind delete cluster --name kind-with-external-etcd
delete_kind_cluster() {
  kind delete cluster --name kind-with-external-etcd
  stop_etcd
}
trap "delete_kind_cluster" EXIT SIGINT
pushd "${ETCD_REPO}/tests/robustness/coverage"
kind create cluster --config kind-with-tracing.yaml --name kind-with-external-etcd --image kindest/node:latest
popd

echo "Running Kubernetes e2e tests..."
pushd "${KUBERNETES_REPO}"
make WHAT="test/e2e/e2e.test"
./_output/bin/e2e.test \
  -context kind-kind-with-external-etcd \
  -ginkgo.focus="\[sig-apps\].*Conformance" \
  -num-nodes 2
echo "Running Kubernetes cmd tests..."
./build/run.sh make test-cmd
popd

echo "Downloading traces..."
curl -v --get --retry 10 --retry-connrefused -o "${ETCD_REPO}/tests/robustness/coverage/testdata/traces.json" \
  -H "Content-Type: application/json" \
  --data-urlencode "query.start_time_min=$(date --date="5 days ago" -Ins)" \
  --data-urlencode "query.start_time_max=$(date --date="2 minutes ago" -Ins)" \
  --data-urlencode "query.service_name=etcd" \
  "http://192.168.32.1:16686/api/v3/traces"

echo "Cleaning up..."
delete_kind_cluster

echo "Done."
