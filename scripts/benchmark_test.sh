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

# This utility script helps to set up the test environment for benchmark tests and runs the provided tests sequentially.

set -euo pipefail

source ./scripts/test_lib.sh

declare -A BENCHMARK_FLAGS
BENCHMARKS_TO_RUN=()
COMMON_BENCHMARK_FLAGS=(--report-perfdash)

parse_args() {
  for arg in "$@"; do
    if [[ "$arg" == *:* ]]; then
      key="${arg%%:*}"
      value="${arg#*:}"
      BENCHMARKS_TO_RUN+=("$key")
      BENCHMARK_FLAGS["$key"]="$value"
    else
      BENCHMARKS_TO_RUN+=("$arg")
      BENCHMARK_FLAGS["$arg"]=""
    fi
  done

  if [ ${#BENCHMARKS_TO_RUN[@]} -eq 0 ]; then
    log_error "Usage: ./benchmark_test.sh 'benchmark-operation:\"test_arguments\"'"
    log_error "Example: ./benchmark_test.sh 'put:\"--clients=1000\" range:\"--conns=100\"'"
    exit 1
  fi
}

start_etcd() {
  log_callout "Setup: Starting the etcd server..."
  # Create a directory for etcd data under /tmp/etcd
  mkdir -p /tmp/etcd
  DATA_DIR=$(mktemp -d /tmp/etcd/data-XXXXXX)
  ARTIFACTS_DIR="${ARTIFACTS:-./_artifacts}"
  mkdir -p "$ARTIFACTS_DIR"
  ETCD_LOG_FILE="${ARTIFACTS_DIR}/etcd-${bench}-$(date +%Y%m%d-%H%M%S).log"
  log_callout -e "Setup: etcd log file path set to $ETCD_LOG_FILE. DATA_DIR set to $DATA_DIR"
  ./bin/etcd --data-dir="$DATA_DIR" > "${ETCD_LOG_FILE}" 2>&1 &
  ETCD_PID=$!
  RETRY=0
  MAX_RETRY_ATTEMPTS=10

  # Set up the trap to handle errors/interrupts which leads to cleaning up of the DATADIR and stopping the etcd server.
  trap 'stop_and_cleanup_etcd' EXIT

  # Poll until etcd is healthy
  until curl -fs http://127.0.0.1:2379/health | grep -q '"health":"true"'; do
    RETRY=$((RETRY + 1))
    if [[ $RETRY -gt $MAX_RETRY_ATTEMPTS ]]; then
      log_error -e "Setup: Failed to confirm etcd health after $MAX_RETRY_ATTEMPTS attempts."
      exit 1
    fi
    log_warning -e "Setup: Waiting for etcd to be healthy... (retry: $RETRY/$MAX_RETRY_ATTEMPTS)"
    sleep 1
  done
  log_success -e "Setup: etcd is healthy and running on pid $ETCD_PID"
}

stop_and_cleanup_etcd() {
  trap - EXIT
  log_warning -e "Cleanup: Stopping etcd server - PID $ETCD_PID"
  kill "$ETCD_PID" 2>/dev/null || true
  rm -rf "$DATA_DIR"
  log_success "Cleanup: Deleted the DATA_DIR contents from $DATA_DIR related to benchmark test"
}

run_benchmark() {
  local bench=$1
  local args="${BENCHMARK_FLAGS[$bench]:-}"

  if [[ -z "$args" ]]; then
    log_callout -e "\\nPerforming benchmark $bench with default arguments"
    benchmark "$bench" "${COMMON_BENCHMARK_FLAGS[@]}"
  else
    log_callout -e "\\nPerforming benchmark $bench with arguments: $args"
    read -r -a TESTER_OPTIONS <<< "$args"

    printf "Running: benchmark %s %s %s\n" \
      "$bench" "${TESTER_OPTIONS[*]}" "${COMMON_BENCHMARK_FLAGS[*]}"

    benchmark "$bench" "${TESTER_OPTIONS[@]}" "${COMMON_BENCHMARK_FLAGS[@]}"
  fi
}

main() {
  parse_args "$@"

  for bench in "${BENCHMARKS_TO_RUN[@]}"; do
    start_etcd
    run_benchmark "$bench"
    stop_and_cleanup_etcd
  done
}

main "$@"
