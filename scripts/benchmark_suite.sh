#!/usr/bin/env bash
# Copyright 2026 The etcd Authors
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

# Run a suite of etcd benchmark workloads against an existing cluster.
#
# WARNING: This script writes data to the target cluster and runs compact/defrag.
# Do not run against production clusters.
#
# Usage:
#   ./scripts/benchmark_suite.sh --endpoints http://127.0.0.1:2379
#   cd scripts && ./benchmark_suite.sh --endpoints http://127.0.0.1:2379
#   ./scripts/benchmark_suite.sh --profile heavy --endpoints http://127.0.0.1:2379
#   ./scripts/benchmark_suite.sh --config ./my-load.conf --endpoints http://10.0.0.5:2379
#
# Configure load (precedence: CLI flags > env vars > config file > profile > defaults):
#   ./scripts/benchmark_suite.sh --clients 100 --put-total 3000000 --endpoints http://127.0.0.1:2379
#   CLIENTS=100 PUT_TOTAL=3000000 ./scripts/benchmark_suite.sh --endpoints http://127.0.0.1:2379
#
# Note: "benchmark mvcc put" is intentionally excluded. It benchmarks local MVCC
# storage and does not use cluster endpoints.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ETCD_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ETCD_ROOT}"

source "${SCRIPT_DIR}/test_utils.sh"

CONFIG_FILE=""
PROFILE=""
OPERATIONS_OVERRIDE=""
CLI_CLIENTS=""
CLI_CONNS=""
CLI_PUT_TOTAL=""
CLI_STM_TOTAL=""
CLI_TXN_MIXED_TOTAL=""
CLI_TXN_PUT_TOTAL=""
CLI_LEASE_KEEPALIVE_TOTAL=""
CLI_RANGE_TOTAL=""
CLI_OPERATIONS=""

# Capture environment overrides before applying defaults/profile/config.
env_overrides=()
for var in \
  ENDPOINTS OUTPUT_DIR ENABLE_PPROF TIMEOUT_DURATION PROFILE_DURATION \
  CLIENTS CONNS KEY_SIZE VAL_SIZE \
  PUT_TOTAL STM_TOTAL TXN_MIXED_TOTAL TXN_PUT_TOTAL LEASE_KEEPALIVE_TOTAL \
  RANGE_TOTAL RANGE_LIMIT RANGE_KEY_START RANGE_KEY_END \
  WATCH_STREAMS WATCH_PER_STREAM WATCHED_KEYS_TOTAL WATCH_PUT_RATE \
  BENCHMARK_BIN ETCDCTL_BIN TLS_CERT TLS_KEY TLS_CACERT BENCHMARK_USER OPERATIONS; do
  if [[ -n "${!var+x}" ]]; then
    env_overrides+=("${var}")
  fi
done

apply_defaults() {
  ENDPOINTS="${ENDPOINTS:-http://127.0.0.1:2379}"
  OUTPUT_DIR="${OUTPUT_DIR:-./benchmark-results}"
  ENABLE_PPROF="${ENABLE_PPROF:-false}"
  TIMEOUT_DURATION="${TIMEOUT_DURATION:-900}"
  PROFILE_DURATION="${PROFILE_DURATION:-120}"

  CLIENTS="${CLIENTS:-10}"
  CONNS="${CONNS:-1}"

  KEY_SIZE="${KEY_SIZE:-32}"
  VAL_SIZE="${VAL_SIZE:-512}"

  PUT_TOTAL="${PUT_TOTAL:-10000}"
  STM_TOTAL="${STM_TOTAL:-10000}"
  TXN_MIXED_TOTAL="${TXN_MIXED_TOTAL:-10000}"
  TXN_PUT_TOTAL="${TXN_PUT_TOTAL:-10000}"
  LEASE_KEEPALIVE_TOTAL="${LEASE_KEEPALIVE_TOTAL:-10000}"

  RANGE_TOTAL="${RANGE_TOTAL:-10000}"
  RANGE_LIMIT="${RANGE_LIMIT:-100}"
  RANGE_KEY_START="${RANGE_KEY_START:-1}"
  RANGE_KEY_END="${RANGE_KEY_END:-1000}"

  WATCH_STREAMS="${WATCH_STREAMS:-10}"
  WATCH_PER_STREAM="${WATCH_PER_STREAM:-100}"
  WATCHED_KEYS_TOTAL="${WATCHED_KEYS_TOTAL:-1000}"
  WATCH_PUT_RATE="${WATCH_PUT_RATE:-200}"

  BENCHMARK_BIN="${BENCHMARK_BIN:-}"
  ETCDCTL_BIN="${ETCDCTL_BIN:-}"

  TLS_CERT="${TLS_CERT:-}"
  TLS_KEY="${TLS_KEY:-}"
  TLS_CACERT="${TLS_CACERT:-}"
  BENCHMARK_USER="${BENCHMARK_USER:-}"

  if [[ -z "${OPERATIONS_OVERRIDE}" ]]; then
    OPERATIONS=(
      put
      stm
      txn-mixed
      txn-put
      range
      lease-keepalive
      watch
      watch-get
      watch-latency
    )
  fi
}

apply_profile() {
  case "${1:-}" in
    "")
      ;;
    quick)
      CLIENTS=10
      CONNS=1
      PUT_TOTAL=10000
      STM_TOTAL=10000
      TXN_MIXED_TOTAL=10000
      TXN_PUT_TOTAL=10000
      LEASE_KEEPALIVE_TOTAL=10000
      RANGE_TOTAL=10000
      WATCH_STREAMS=10
      WATCH_PER_STREAM=100
      WATCHED_KEYS_TOTAL=1000
      ;;
    heavy)
      CLIENTS=100
      CONNS=10
      PUT_TOTAL=3000000
      STM_TOTAL=1500000
      TXN_MIXED_TOTAL=1000000
      TXN_PUT_TOTAL=2000000
      LEASE_KEEPALIVE_TOTAL=10000
      RANGE_TOTAL=1500000
      WATCH_STREAMS=200
      WATCH_PER_STREAM=2000
      WATCHED_KEYS_TOTAL=1000000
      ;;
    *)
      log_error "unknown profile: $1 (supported: quick, heavy)"
      exit 1
      ;;
  esac
}

is_allowed_config_key() {
  case "$1" in
    ENDPOINTS | OUTPUT_DIR | ENABLE_PPROF | TIMEOUT_DURATION | PROFILE_DURATION \
      | CLIENTS | CONNS | KEY_SIZE | VAL_SIZE \
      | PUT_TOTAL | STM_TOTAL | TXN_MIXED_TOTAL | TXN_PUT_TOTAL | LEASE_KEEPALIVE_TOTAL \
      | RANGE_TOTAL | RANGE_LIMIT | RANGE_KEY_START | RANGE_KEY_END \
      | WATCH_STREAMS | WATCH_PER_STREAM | WATCHED_KEYS_TOTAL | WATCH_PUT_RATE \
      | BENCHMARK_BIN | ETCDCTL_BIN | TLS_CERT | TLS_KEY | TLS_CACERT | BENCHMARK_USER \
      | OPERATIONS)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

set_config_value() {
  local key="$1"
  local value="$2"
  case "${key}" in
    OPERATIONS)
      OPERATIONS_OVERRIDE="${value}"
      ;;
    *)
      printf -v "${key}" '%s' "${value}"
      ;;
  esac
}

load_config() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    log_error "config file not found: ${file}"
    exit 1
  fi

  log_callout "Loading config: ${file}"
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%%#*}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "${line}" ]] && continue

    if [[ "${line}" != *"="* ]]; then
      log_warning "ignoring invalid config line: ${line}"
      continue
    fi

    local key="${line%%=*}"
    local value="${line#*=}"
    key="${key%"${key##*[![:space:]]}"}"
    value="${value#"${value%%[![:space:]]*}"}"

    if ! is_allowed_config_key "${key}"; then
      log_warning "ignoring unknown config key: ${key}"
      continue
    fi

    set_config_value "${key}" "${value}"
  done <"${file}"
}

parse_operations_list() {
  local raw="$1"
  local item
  OPERATIONS=()
  raw="${raw//,/ }"
  for item in ${raw}; do
    OPERATIONS+=("${item}")
  done
}

restore_env_overrides() {
  local var
  if ((${#env_overrides[@]} == 0)); then
    return 0
  fi
  for var in "${env_overrides[@]}"; do
    set_config_value "${var}" "${!var}"
  done
}

apply_cli_overrides() {
  [[ -n "${CLI_CLIENTS}" ]] && CLIENTS="${CLI_CLIENTS}"
  [[ -n "${CLI_CONNS}" ]] && CONNS="${CLI_CONNS}"
  [[ -n "${CLI_PUT_TOTAL}" ]] && PUT_TOTAL="${CLI_PUT_TOTAL}"
  [[ -n "${CLI_STM_TOTAL}" ]] && STM_TOTAL="${CLI_STM_TOTAL}"
  [[ -n "${CLI_TXN_MIXED_TOTAL}" ]] && TXN_MIXED_TOTAL="${CLI_TXN_MIXED_TOTAL}"
  [[ -n "${CLI_TXN_PUT_TOTAL}" ]] && TXN_PUT_TOTAL="${CLI_TXN_PUT_TOTAL}"
  [[ -n "${CLI_LEASE_KEEPALIVE_TOTAL}" ]] && LEASE_KEEPALIVE_TOTAL="${CLI_LEASE_KEEPALIVE_TOTAL}"
  [[ -n "${CLI_RANGE_TOTAL}" ]] && RANGE_TOTAL="${CLI_RANGE_TOTAL}"
  if [[ -n "${CLI_OPERATIONS}" ]]; then
    parse_operations_list "${CLI_OPERATIONS}"
  elif [[ -n "${OPERATIONS_OVERRIDE}" ]]; then
    parse_operations_list "${OPERATIONS_OVERRIDE}"
  fi
}

print_effective_config() {
  log_callout "Effective load configuration:"
  log_info "  endpoints=${ENDPOINTS}"
  log_info "  clients=${CLIENTS} conns=${CONNS}"
  log_info "  key_size=${KEY_SIZE} val_size=${VAL_SIZE}"
  log_info "  put_total=${PUT_TOTAL} stm_total=${STM_TOTAL}"
  log_info "  txn_mixed_total=${TXN_MIXED_TOTAL} txn_put_total=${TXN_PUT_TOTAL}"
  log_info "  lease_keepalive_total=${LEASE_KEEPALIVE_TOTAL}"
  log_info "  range_total=${RANGE_TOTAL} range_limit=${RANGE_LIMIT}"
  log_info "  watch_streams=${WATCH_STREAMS} watch_per_stream=${WATCH_PER_STREAM}"
  log_info "  watched_keys_total=${WATCHED_KEYS_TOTAL} watch_put_rate=${WATCH_PUT_RATE}"
  log_info "  operations=${OPERATIONS[*]}"
}

usage() {
  cat <<EOF
Usage: $0 [options]

Run benchmark workloads against an existing etcd cluster.

Options:
  -e, --endpoints URL         etcd client endpoint (default: http://127.0.0.1:2379)
  -o, --output-dir DIR        directory for logs, profiles, and summary
  -c, --config FILE           load load settings from a config file (see benchmark_suite.conf.example)
      --profile NAME          built-in load profile: quick (default) or heavy
  -p, --enable-pprof          collect CPU profiles from /debug/pprof during each benchmark
      --operations LIST       comma-separated benchmarks to run (default: all)
      --clients N             number of gRPC clients
      --conns N               number of gRPC connections
      --put-total N           total put requests
      --stm-total N           total stm requests
      --txn-mixed-total N     total txn-mixed requests
      --txn-put-total N       total txn-put requests
      --lease-keepalive-total N
                              total lease-keepalive requests
      --range-total N         total range requests
  -h, --help                  show this help

Load precedence (highest to lowest):
  CLI flags > environment variables > --config > --profile > built-in defaults

Example config file: scripts/benchmark_suite.conf.example

EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -e | --endpoints)
      ENDPOINTS="$2"
      shift 2
      ;;
    -o | --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    -c | --config)
      CONFIG_FILE="$2"
      shift 2
      ;;
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    -p | --enable-pprof)
      ENABLE_PPROF=true
      shift
      ;;
    --operations)
      CLI_OPERATIONS="$2"
      shift 2
      ;;
    --clients)
      CLI_CLIENTS="$2"
      shift 2
      ;;
    --conns)
      CLI_CONNS="$2"
      shift 2
      ;;
    --put-total)
      CLI_PUT_TOTAL="$2"
      shift 2
      ;;
    --stm-total)
      CLI_STM_TOTAL="$2"
      shift 2
      ;;
    --txn-mixed-total)
      CLI_TXN_MIXED_TOTAL="$2"
      shift 2
      ;;
    --txn-put-total)
      CLI_TXN_PUT_TOTAL="$2"
      shift 2
      ;;
    --lease-keepalive-total)
      CLI_LEASE_KEEPALIVE_TOTAL="$2"
      shift 2
      ;;
    --range-total)
      CLI_RANGE_TOTAL="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      log_error "unknown argument: $1"
      usage
      exit 1
      ;;
  esac
done

apply_defaults
apply_profile "${PROFILE}"
if [[ -n "${CONFIG_FILE}" ]]; then
  load_config "${CONFIG_FILE}"
fi
restore_env_overrides
apply_cli_overrides

normalize_endpoint() {
  local endpoint="$1"
  if [[ "${endpoint}" == http://* || "${endpoint}" == https://* ]]; then
    echo "${endpoint}"
  elif [[ "${endpoint}" == http:* ]]; then
    echo "http://${endpoint#http:}"
  elif [[ "${endpoint}" == https:* ]]; then
    echo "https://${endpoint#https:}"
  else
    echo "http://${endpoint}"
  fi
}

first_endpoint() {
  local endpoints="${1%,*}"
  normalize_endpoint "${endpoints}"
}

resolve_benchmark_bin() {
  if [[ -n "${BENCHMARK_BIN}" ]]; then
    echo "${BENCHMARK_BIN}"
    return
  fi
  if command -v benchmark >/dev/null 2>&1; then
    command -v benchmark
    return
  fi
  local go_bin
  if command -v go >/dev/null 2>&1; then
    go_bin="$(go env GOPATH 2>/dev/null)/bin/benchmark"
    if [[ -x "${go_bin}" ]]; then
      echo "${go_bin}"
      return
    fi
  fi
  if [[ -x "${HOME}/go/bin/benchmark" ]]; then
    echo "${HOME}/go/bin/benchmark"
    return
  fi
  if [[ -x "./bin/tools/benchmark" ]]; then
    echo "./bin/tools/benchmark"
    return
  fi
  log_error "benchmark binary not found; install with: go install -v ./tools/benchmark"
  log_error "if already installed, add Go bin to PATH: export PATH=\"\$(go env GOPATH)/bin:\$PATH\""
  exit 1
}

resolve_etcdctl_bin() {
  if [[ -n "${ETCDCTL_BIN}" ]]; then
    echo "${ETCDCTL_BIN}"
    return
  fi
  if [[ -x "./bin/etcdctl" ]]; then
    echo "./bin/etcdctl"
    return
  fi
  if command -v etcdctl >/dev/null 2>&1; then
    command -v etcdctl
    return
  fi
  log_error "etcdctl binary not found; build with: make build"
  exit 1
}

common_benchmark_flags() {
  local flags=(
    --endpoints="${ENDPOINTS}"
    --conns="${CONNS}"
    --clients="${CLIENTS}"
  )
  if [[ -n "${TLS_CERT}" ]]; then
    flags+=(--cert="${TLS_CERT}")
  fi
  if [[ -n "${TLS_KEY}" ]]; then
    flags+=(--key="${TLS_KEY}")
  fi
  if [[ -n "${TLS_CACERT}" ]]; then
    flags+=(--cacert="${TLS_CACERT}")
  fi
  if [[ -n "${BENCHMARK_USER}" ]]; then
    flags+=(--user="${BENCHMARK_USER}")
  fi
  echo "${flags[@]}"
}

common_etcdctl_flags() {
  local flags=(--endpoints="${ENDPOINTS}")
  if [[ -n "${TLS_CERT}" ]]; then
    flags+=(--cert="${TLS_CERT}")
  fi
  if [[ -n "${TLS_KEY}" ]]; then
    flags+=(--key="${TLS_KEY}")
  fi
  if [[ -n "${TLS_CACERT}" ]]; then
    flags+=(--cacert="${TLS_CACERT}")
  fi
  if [[ -n "${BENCHMARK_USER}" ]]; then
    flags+=(--user="${BENCHMARK_USER}")
  fi
  echo "${flags[@]}"
}

wait_for_etcd() {
  local health_url="$1/health"
  log_callout "Waiting for etcd at ${health_url}..."
  for _ in $(seq 1 30); do
    if curl -sf "${health_url}" | grep -q '"health":"true"'; then
      log_success "etcd is healthy"
      return 0
    fi
    sleep 2
  done
  log_error "etcd did not become healthy at ${health_url}"
  exit 1
}

compact_and_defrag() {
  local etcdctl_flags
  read -r -a etcdctl_flags <<<"$(common_etcdctl_flags)"

  log_callout "Compacting and defragmenting etcd..."
  if ! command -v jq >/dev/null 2>&1; then
    log_warning "jq not found; skipping compact/defrag"
    return
  fi

  local rev
  rev=$("${ETCDCTL_BIN}" "${etcdctl_flags[@]}" endpoint status --write-out=json \
    | jq -r '.[0].Status.header.revision // empty')
  if [[ -z "${rev}" ]]; then
    log_warning "failed to determine revision; skipping compact/defrag"
    return
  fi

  "${ETCDCTL_BIN}" "${etcdctl_flags[@]}" compact "${rev}" --physical=true || true
  "${ETCDCTL_BIN}" "${etcdctl_flags[@]}" defrag || true
  log_success "compacted revision: ${rev}"
  sleep 3
}

start_cpu_profile() {
  local profile_file="$1"
  local profile_url="$2/profile?seconds=${PROFILE_DURATION}"
  log_callout "Collecting ${PROFILE_DURATION}s CPU profile from ${profile_url}"
  curl --silent "${profile_url}" --output "${profile_file}" &
  echo $!
}

benchmark_command_for_op() {
  local op="$1"
  local common_flags
  read -r -a common_flags <<<"$(common_benchmark_flags)"

  case "${op}" in
    put)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" put \
        --key-size="${KEY_SIZE}" --val-size="${VAL_SIZE}" --total="${PUT_TOTAL}"
      ;;
    stm)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" stm --total="${STM_TOTAL}"
      ;;
    txn-mixed)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" txn-mixed benchmark-suite \
        --key-size="${KEY_SIZE}" --val-size="${VAL_SIZE}" --total="${TXN_MIXED_TOTAL}" \
        --key-space-size=1000
      ;;
    txn-put)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" txn-put \
        --key-size="${KEY_SIZE}" --val-size="${VAL_SIZE}" --total="${TXN_PUT_TOTAL}"
      ;;
    range)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" range \
        "${RANGE_KEY_START}" "${RANGE_KEY_END}" \
        --total="${RANGE_TOTAL}" --limit="${RANGE_LIMIT}"
      ;;
    lease-keepalive)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" lease-keepalive \
        --total="${LEASE_KEEPALIVE_TOTAL}"
      ;;
    watch)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" watch \
        --key-size="${KEY_SIZE}" --streams="${WATCH_STREAMS}" \
        --watch-per-stream="${WATCH_PER_STREAM}"
      ;;
    watch-get)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" watch-get \
        --streams="${WATCH_STREAMS}" --watchers="${WATCHED_KEYS_TOTAL}"
      ;;
    watch-latency)
      echo "${BENCHMARK_BIN}" "${common_flags[@]}" watch-latency \
        --key-size="${KEY_SIZE}" --put-rate="${WATCH_PUT_RATE}" \
        --put-total="${WATCHED_KEYS_TOTAL}" --streams="${WATCH_STREAMS}" \
        --watchers-per-stream="${WATCH_PER_STREAM}"
      ;;
    *)
      log_error "unknown operation: ${op}"
      exit 1
      ;;
  esac
}

extract_requests_per_sec() {
  local log_file="$1"
  grep "Requests/sec" "${log_file}" | awk '{print $2}' | paste -sd ';' - || true
}

ENDPOINTS="$(first_endpoint "${ENDPOINTS}")"
BENCHMARK_BIN="$(resolve_benchmark_bin)"
ETCDCTL_BIN="$(resolve_etcdctl_bin)"
PPROF_BASE="${ENDPOINTS}/debug/pprof"
SUMMARY_FILE="${OUTPUT_DIR}/summary.csv"

mkdir -p "${OUTPUT_DIR}"
echo "operation,duration_sec,exit_code,requests_per_sec" >"${SUMMARY_FILE}"

log_warning "This script writes data to the target cluster. Do not run against production."
print_effective_config
wait_for_etcd "${ENDPOINTS}"

for op in "${OPERATIONS[@]}"; do
  log_callout "Running benchmark: ${op}"

  op_dir="${OUTPUT_DIR}/${op//-/_}"
  mkdir -p "${op_dir}"

  log_file="${op_dir}/${op}.log"
  profile_file="${op_dir}/${op}-cpu.pprof"

  if [[ "${op}" == watch* ]]; then
    compact_and_defrag
  fi

  cmd=()
  read -r -a cmd <<<"$(benchmark_command_for_op "${op}")"
  cmd=(timeout "${TIMEOUT_DURATION}s" "${cmd[@]}")

  cpu_pid=""
  if [[ "${ENABLE_PPROF}" == "true" ]]; then
    cpu_pid="$(start_cpu_profile "${profile_file}" "${PPROF_BASE}")"
  fi

  start_ts=$(date +%s)
  log_callout "Running: ${cmd[*]}"

  set +e
  "${cmd[@]}" >"${log_file}" 2>&1
  exit_code=$?
  set -e

  end_ts=$(date +%s)
  elapsed=$((end_ts - start_ts))

  if [[ -n "${cpu_pid}" ]]; then
    wait "${cpu_pid}" 2>/dev/null || true
  fi

  rps="$(extract_requests_per_sec "${log_file}")"
  echo "${op},${elapsed},${exit_code},${rps}" >>"${SUMMARY_FILE}"

  log_callout "Duration: ${elapsed}s"
  if [[ -n "${rps}" ]]; then
    log_callout "Requests/sec: ${rps}"
  else
    log_warning "Requests/sec not found in ${log_file}"
  fi

  if [[ "${exit_code}" -eq 0 ]]; then
    log_success "Completed: ${op}"
  elif [[ "${exit_code}" -eq 124 ]]; then
    log_warning "Timeout reached for: ${op}"
  else
    log_error "Failed: ${op} (exit code: ${exit_code})"
  fi
done

log_success "Benchmark suite completed. Summary: ${SUMMARY_FILE}"