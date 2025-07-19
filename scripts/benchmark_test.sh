#!/usr/bin/env bash

# This scripts runs benchmark tests on the locally spun-up etcd instance

set -euo pipefail

source ./scripts/test_lib.sh

declare -A BENCHMARK_FLAGS
BENCHMARKS_TO_RUN=()
COMMON_BENCHMARK_FLAGS="--report-perfdash"

# Supported benchmarks
ALL_BENCHMARKS=(put lease-keepalive range stm txn-mixed txn-put watch watch-get watch-latency)

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

# If there are no specific benchmarks mentioned, run benchmarks of all types apart from mvcc.
if [ ${#BENCHMARKS_TO_RUN[@]} -eq 0 ]; then
  BENCHMARKS_TO_RUN=("${ALL_BENCHMARKS[@]}")
fi

# Set default "key" flag only if benchmark range needs to be run and no flags were passed
if [[ " ${BENCHMARKS_TO_RUN[*]} " == *" range "* ]] && [[ -z "${BENCHMARK_FLAGS[range]+set}" ]]; then
  BENCHMARK_FLAGS[range]="key"
fi

echo "Starting the etcd server..."
./bin/etcd > /tmp/etcd.log 2>&1 &
etcd_pid=$!

# Poll on the health endpoint while the etcd server is still setting up before running benchmarks
for retry in {1..10}; do
  if curl -fs http://127.0.0.1:2379/health | grep -q '"health":"true"'; then
    log_success -e "\\netcd is healthy"
    break
  fi
  log_warning -e "\\nWaiting for etcd to be healthy..."
  sleep 1
  # Exit if the 10th attempt is unsuccessful.
  if [[ $retry -eq 10 ]]; then
    log_error -e "\\nFailed to confirm etcd health after $retry attempts. Check /tmp/etcd.log for more information"
    exit 1
  fi
done

log_success -e "etcd process is running with pid $etcd_pid"
trap 'log_warning -e "Stopping etcd server - PID:$etcd_pid"; kill $etcd_pid 2>/dev/null' EXIT

for bench in "${BENCHMARKS_TO_RUN[@]}"; do
  args="${BENCHMARK_FLAGS[$bench]:-}"
  if [[ -z "$args" ]]; then
    log_callout -e "\\nPerforming benchmark $bench with default arguments"
  else
    log_callout -e "\\nPerforming benchmark $bench with arguments: $args"
  fi
  read -r -a TESTER_OPTIONS <<< "$args"
  printf "Running: benchmark %s %s %s\n" "$bench" "${TESTER_OPTIONS[*]}" "$COMMON_BENCHMARK_FLAGS"
  benchmark "$bench" "${TESTER_OPTIONS[@]}" $COMMON_BENCHMARK_FLAGS
done
