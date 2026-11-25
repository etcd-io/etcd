#!/usr/bin/env bash
# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs a benchmark on a locally started etcd server.
# Set ETCD_CLUSTER_SIZE=3 to benchmark a local 3-member cluster.

set -euo pipefail

source ./scripts/test_lib.sh

COMMON_BENCHMARK_FLAGS="--report-perfdash"

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <benchmark-name> [tester args...]"
  exit 1
fi

BENCHMARK_NAME="$1"
ARGS="${*:2}"

# ETCD_CLUSTER_SIZE selects the number of local etcd members (default 1).
# With a value greater than 1 the members run on one host and the benchmark
# is pointed at all of them, so replication and Raft commit latency are part
# of the measurement.
CLUSTER_SIZE="${ETCD_CLUSTER_SIZE:-1}"
if ! [[ "$CLUSTER_SIZE" =~ ^[1-9][0-9]*$ ]]; then
  log_error "ETCD_CLUSTER_SIZE must be a positive integer, got '$CLUSTER_SIZE'"
  exit 1
fi

echo "Starting the etcd server (cluster size $CLUSTER_SIZE)..."

# Create a directory for etcd data under /tmp/etcd
mkdir -p /tmp/etcd
DATA_DIR=$(mktemp -d /tmp/etcd/data-XXXXXX)

etcd_pids=()
ENDPOINTS=""
if [[ $CLUSTER_SIZE -eq 1 ]]; then
  ./bin/etcd --data-dir="$DATA_DIR" > /tmp/etcd.log 2>&1 &
  etcd_pids+=($!)
else
  INITIAL_CLUSTER=""
  for ((i = 0; i < CLUSTER_SIZE; i++)); do
    INITIAL_CLUSTER+="${INITIAL_CLUSTER:+,}infra$i=http://127.0.0.1:$((2380 + i * 10000))"
  done
  for ((i = 0; i < CLUSTER_SIZE; i++)); do
    client_url="http://127.0.0.1:$((2379 + i * 10000))"
    peer_url="http://127.0.0.1:$((2380 + i * 10000))"
    ENDPOINTS+="${ENDPOINTS:+,}$client_url"
    ./bin/etcd --name "infra$i" --data-dir="$DATA_DIR/infra$i" \
      --listen-client-urls "$client_url" --advertise-client-urls "$client_url" \
      --listen-peer-urls "$peer_url" --initial-advertise-peer-urls "$peer_url" \
      --initial-cluster-token etcd-benchmark --initial-cluster "$INITIAL_CLUSTER" \
      --initial-cluster-state new > "/tmp/etcd-infra$i.log" 2>&1 &
    etcd_pids+=($!)
  done
fi
etcd_pid="${etcd_pids[*]}"

trap 'log_warning -e "Stopping etcd server - PID $etcd_pid";
      kill $etcd_pid 2>/dev/null;
      wait $etcd_pid 2>/dev/null;
      rm -rf $DATA_DIR;
      log_success "Deleted the contents from $DATA_DIR related to benchmark test"' EXIT

ETCDCTL_ENDPOINT_FLAGS=()
if [[ -n "$ENDPOINTS" ]]; then
  ETCDCTL_ENDPOINT_FLAGS=(--endpoints="$ENDPOINTS")
fi

# Wait until etcd becomes healthy
for retry in {1..10}; do
  if ./bin/etcdctl ${ETCDCTL_ENDPOINT_FLAGS[@]+"${ETCDCTL_ENDPOINT_FLAGS[@]}"} endpoint health --cluster> /dev/null 2>&1; then
    log_success -e "\\netcd is healthy"
    break
  fi
  log_warning -e "\\nWaiting for etcd to be healthy..."
  sleep 1
  if [[ $retry -eq 10 ]]; then
    log_error -e "\\nFailed to confirm etcd health after $retry attempts. Check /tmp/etcd*.log for more information"
    exit 1
  fi
done

log_success -e "etcd process is running with PID $etcd_pid"

log_callout -e "\\nPerforming benchmark $BENCHMARK_NAME with arguments: $ARGS"
read -r -a TESTER_OPTIONS <<< "$ARGS"
if [[ -n "$ENDPOINTS" && "$ARGS" != *--endpoints* ]]; then
  TESTER_OPTIONS+=("--endpoints=$ENDPOINTS")
fi
log_callout "Running: benchmark $BENCHMARK_NAME ${TESTER_OPTIONS[*]} $COMMON_BENCHMARK_FLAGS"
benchmark "$BENCHMARK_NAME" "${TESTER_OPTIONS[@]}" $COMMON_BENCHMARK_FLAGS
log_callout "Completed: benchmark $BENCHMARK_NAME ${TESTER_OPTIONS[*]} $COMMON_BENCHMARK_FLAGS"
