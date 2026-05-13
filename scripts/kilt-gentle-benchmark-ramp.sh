#!/usr/bin/env bash

set -uo pipefail

BENCH_BIN="${BENCH_BIN:-${HOME}/benchmark-2pc-linux}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${HOME}/bench-artifacts}"
RUN_GROUP="${RUN_GROUP:-kilt-gentle}"
KEY_PREFIX="${KEY_PREFIX:-kilt-bench/${RUN_GROUP}/}"
AUTO_RUN="${AUTO_RUN:-false}"
START_AT="${START_AT:-}"
ENDPOINTS="${ENDPOINTS:-10.201.162.151:2379,10.201.47.57:2379,10.201.98.50:2379,10.201.5.177:2379,10.201.117.10:2379}"
SAMPLE_INTERVAL="${SAMPLE_INTERVAL:-5s}"
REMOTE_METRICS_HOSTS="${REMOTE_METRICS_HOSTS:-opc@kilt-phx-dev-1-etcdn1-1,opc@kilt-phx-dev-1-etcdn2-1,opc@kilt-phx-dev-1-etcdn3-1,opc@kilt-phx-dev-1-etcdn4-1,opc@kilt-phx-dev-1-etcdn5-1}"
REMOTE_METRICS_TIMEOUT="${REMOTE_METRICS_TIMEOUT:-4s}"
SCENARIOS="${SCENARIOS:-put,txn-put,watch-latency,mixed,range}"
DURATION_SECONDS="${DURATION_SECONDS:-30}"
FULL_SUMMARY_PATH="${FULL_SUMMARY_PATH:-${ARTIFACT_DIR}/${RUN_GROUP}-full-summary.md}"
FULL_SUMMARY_HTML_PATH="${FULL_SUMMARY_HTML_PATH:-${ARTIFACT_DIR}/${RUN_GROUP}-full-summary.html}"
RECOVER_ON_NOSPACE="${RECOVER_ON_NOSPACE:-false}"
RECOVER_ON_NOSPACE_MAX_RETRIES="${RECOVER_ON_NOSPACE_MAX_RETRIES:-1}"
RECOVER_CLEAN_SCRIPT="${RECOVER_CLEAN_SCRIPT:-${HOME}/kilt-etcd-clean-benchmark-data.sh}"
RECOVER_CLEAN_PREFIX="${RECOVER_CLEAN_PREFIX:-${KEY_PREFIX}}"
RECOVER_COMMAND_TIMEOUT="${RECOVER_COMMAND_TIMEOUT:-300s}"
RECOVER_RESTART_AFTER_DEFRAG="${RECOVER_RESTART_AFTER_DEFRAG:-false}"

TXN_OPS="${TXN_OPS:-1}"
TXN_KEY_SIZE="${TXN_KEY_SIZE:-256}"
TXN_VAL_SIZE="${TXN_VAL_SIZE:-512}"
WATCH_STREAMS="${WATCH_STREAMS:-2}"
WATCHERS_PER_STREAM="${WATCHERS_PER_STREAM:-5}"
WATCH_KEY_SIZE="${WATCH_KEY_SIZE:-64}"
WATCH_VAL_SIZE="${WATCH_VAL_SIZE:-512}"
MIXED_RANGE_RATE_DIVISOR="${MIXED_RANGE_RATE_DIVISOR:-10}"
MIXED_RANGE_KEY_TOTAL="${MIXED_RANGE_KEY_TOTAL:-1000}"
MIXED_RANGE_LIMIT="${MIXED_RANGE_LIMIT:-50}"
MIXED_RANGE_CONSISTENCY="${MIXED_RANGE_CONSISTENCY:-s}"
RANGE_LIMIT="${RANGE_LIMIT:-50}"
RANGE_CONSISTENCY="${RANGE_CONSISTENCY:-s}"

PUT_STEPS=(
	"rate-25|25|750|4|2|10000"
	"rate-50|50|1500|4|2|10000"
	"rate-100|100|3000|8|4|20000"
)

TXN_STEPS=(
	"rate-25|25|750|4|2|1000"
	"rate-50|50|1500|4|2|2000"
	"rate-100|100|3000|8|4|5000"
)

WATCH_STEPS=(
	"rate-10|10|300|4|2|1000"
	"rate-25|25|750|4|2|2000"
)

MIXED_STEPS=(
	"rate-10|10|300|4|2|1000"
	"rate-25|25|750|4|2|2000"
)

RANGE_STEPS=(
	"rate-25|25|750|4|2|0"
	"rate-50|50|1500|4|2|0"
	"rate-100|100|3000|8|4|0"
)

usage() {
	cat <<EOF
Usage: $(basename "$0")

Interactive gentle KILT etcd scenario ramp. This is the quick smoke version
of the full capacity script and covers CAS writes, CAS txn-put, watch fan-out,
mixed watch/write/read, and pure range reads.

Environment overrides:
  BENCH_BIN=${BENCH_BIN}
  ARTIFACT_DIR=${ARTIFACT_DIR}
  RUN_GROUP=${RUN_GROUP}
  KEY_PREFIX=${KEY_PREFIX}
  AUTO_RUN=${AUTO_RUN}
  START_AT=${START_AT}
  ENDPOINTS=${ENDPOINTS}
  SAMPLE_INTERVAL=${SAMPLE_INTERVAL}
  REMOTE_METRICS_HOSTS=${REMOTE_METRICS_HOSTS}
  REMOTE_METRICS_TIMEOUT=${REMOTE_METRICS_TIMEOUT}
  SCENARIOS=${SCENARIOS}
  FULL_SUMMARY_PATH=${FULL_SUMMARY_PATH}
  FULL_SUMMARY_HTML_PATH=${FULL_SUMMARY_HTML_PATH}
  RECOVER_ON_NOSPACE=${RECOVER_ON_NOSPACE}
  RECOVER_ON_NOSPACE_MAX_RETRIES=${RECOVER_ON_NOSPACE_MAX_RETRIES}
  RECOVER_CLEAN_SCRIPT=${RECOVER_CLEAN_SCRIPT}
  RECOVER_CLEAN_PREFIX=${RECOVER_CLEAN_PREFIX}
  RECOVER_COMMAND_TIMEOUT=${RECOVER_COMMAND_TIMEOUT}
  RECOVER_RESTART_AFTER_DEFRAG=${RECOVER_RESTART_AFTER_DEFRAG}
  WATCH_STREAMS=${WATCH_STREAMS}
  WATCHERS_PER_STREAM=${WATCHERS_PER_STREAM}
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
	usage
	exit 0
fi

if [[ ! -x "${BENCH_BIN}" ]]; then
	echo "benchmark binary is not executable: ${BENCH_BIN}" >&2
	echo "copy benchmark-2pc-linux to the client and run: chmod +x ${BENCH_BIN}" >&2
	exit 1
fi

mkdir -p "${ARTIFACT_DIR}"

stop_requested=false
running_step=false
start_reached=false
if [[ -z "${START_AT}" ]]; then
	start_reached=true
fi

handle_interrupt() {
	if [[ "${running_step}" == "true" ]]; then
		stop_requested=true
		echo
		echo "Interrupt requested. Waiting for the benchmark to stop cleanly and write JSON/HTML/Markdown artifacts."
		echo "Press Ctrl-C again only if you want to force quit and risk losing the in-progress report."
		trap 'echo; echo "Forced exit requested."; exit 130' INT
	else
		echo
		echo "Stopping before the next benchmark step."
		exit 130
	fi
}

trap handle_interrupt INT

scenario_enabled() {
	local scenario="$1"
	local normalized=",${SCENARIOS// /},"
	[[ "${normalized}" == *",all,"* || "${normalized}" == *",${scenario},"* ]]
}

clear_proxy_env() {
	unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY all_proxy ALL_PROXY no_proxy NO_PROXY
}

should_run_step() {
	local scenario="$1"
	local step_name="$2"
	local marker="${scenario}/${step_name}"

	if [[ "${start_reached}" == "true" ]]; then
		return 0
	fi
	if [[ "${START_AT}" == "${marker}" ]]; then
		start_reached=true
		return 0
	fi
	echo "Skipping ${marker} before START_AT=${START_AT}."
	return 1
}

nospace_failure_seen() {
	local log_path="$1"
	grep -Eqi 'mvcc: database space exceeded|database space exceeded|NOSPACE|ResourceExhausted' "${log_path}"
}

recover_from_nospace() {
	local log_path="$1"

	clear_proxy_env
	if [[ ! -x "${RECOVER_CLEAN_SCRIPT}" ]]; then
		echo "RECOVER_ON_NOSPACE=true but cleanup script is not executable: ${RECOVER_CLEAN_SCRIPT}" | tee -a "${log_path}" >&2
		return 1
	fi

	echo "Proxy environment variables unset for NOSPACE recovery." | tee -a "${log_path}"
	echo "NOSPACE detected. Cleaning prefix ${RECOVER_CLEAN_PREFIX}, compacting, defragmenting, and disarming alarms." | tee -a "${log_path}"
	PREFIX="${RECOVER_CLEAN_PREFIX}" \
		COMMAND_TIMEOUT="${RECOVER_COMMAND_TIMEOUT}" \
		RESTART_AFTER_DEFRAG="${RECOVER_RESTART_AFTER_DEFRAG}" \
		"${RECOVER_CLEAN_SCRIPT}" 2>&1 | tee -a "${log_path}"
	return "${PIPESTATUS[0]}"
}

append_common_metrics_args() {
	local metrics_step="$1"
	local metrics_run_group="$2"
	local metrics_path="$3"
	local graph_path="$4"
	local table_path="$5"

	cmd_args+=(
		--capture-run-metrics
		--metrics-sample-interval="${SAMPLE_INTERVAL}"
		--metrics-step-name="${metrics_step}"
		--metrics-run-group="${metrics_run_group}"
		--metrics-output="${metrics_path}"
		--metrics-elbow-graph-output="${graph_path}"
		--metrics-summary-table-output="${table_path}"
		--metrics-full-summary-output="${FULL_SUMMARY_PATH}"
		--metrics-full-summary-html-output="${FULL_SUMMARY_HTML_PATH}"
		--metrics-aggregate-run-group="${RUN_GROUP}"
	)
	if [[ -n "${REMOTE_METRICS_HOSTS}" ]]; then
		cmd_args+=(
			--metrics-remote-hosts="${REMOTE_METRICS_HOSTS}"
			--metrics-remote-timeout="${REMOTE_METRICS_TIMEOUT}"
		)
	fi
}

build_command() {
	local scenario="$1"
	local step_name="$2"
	local rate="$3"
	local total="$4"
	local clients="$5"
	local conns="$6"
	local key_space_size="$7"
	local metrics_path="$8"
	local graph_path="$9"
	local table_path="${10}"
	local scenario_run_group="${RUN_GROUP}-${scenario}"
	local metrics_step="${scenario}-${step_name}"
	local range_rate range_total

	cmd_args=("${BENCH_BIN}" "--key-prefix=${KEY_PREFIX}")
	case "${scenario}" in
		put)
			cmd_args+=(put --endpoints="${ENDPOINTS}" --rate="${rate}" --total="${total}" --clients="${clients}" --conns="${conns}" --key-space-size="${key_space_size}")
			;;
		txn-put)
			cmd_args+=(txn-put --endpoints="${ENDPOINTS}" --rate="${rate}" --total="${total}" --clients="${clients}" --conns="${conns}" --key-space-size="${key_space_size}" --txn-ops="${TXN_OPS}" --key-size="${TXN_KEY_SIZE}" --val-size="${TXN_VAL_SIZE}")
			;;
		watch-latency)
			cmd_args+=(watch-latency --endpoints="${ENDPOINTS}" --put-rate="${rate}" --put-total="${total}" --clients="${clients}" --conns="${conns}" --streams="${WATCH_STREAMS}" --watchers-per-stream="${WATCHERS_PER_STREAM}" --key-size="${WATCH_KEY_SIZE}" --val-size="${WATCH_VAL_SIZE}")
			;;
		mixed)
			range_rate=$((rate / MIXED_RANGE_RATE_DIVISOR))
			if [[ "${range_rate}" -lt 1 ]]; then
				range_rate=1
			fi
			range_total=$((range_rate * DURATION_SECONDS))
			cmd_args+=(watch-latency --endpoints="${ENDPOINTS}" --put-rate="${rate}" --put-total="${total}" --clients="${clients}" --conns="${conns}" --streams="${WATCH_STREAMS}" --watchers-per-stream="${WATCHERS_PER_STREAM}" --key-size="${WATCH_KEY_SIZE}" --val-size="${WATCH_VAL_SIZE}" --range-total="${range_total}" --range-rate="${range_rate}" --range-key-total="${MIXED_RANGE_KEY_TOTAL}" --range-limit="${MIXED_RANGE_LIMIT}" --range-consistency="${MIXED_RANGE_CONSISTENCY}")
			;;
		range)
			cmd_args+=(range --endpoints="${ENDPOINTS}" --rate="${rate}" --total="${total}" --clients="${clients}" --conns="${conns}" --prefix --limit="${RANGE_LIMIT}" --consistency="${RANGE_CONSISTENCY}" "${KEY_PREFIX}")
			;;
		*)
			echo "unknown scenario: ${scenario}" >&2
			exit 2
			;;
	esac

	append_common_metrics_args "${metrics_step}" "${scenario_run_group}" "${metrics_path}" "${graph_path}" "${table_path}"
}

run_step() {
	local scenario="$1"
	local step_name="$2"
	local rate="$3"
	local total="$4"
	local clients="$5"
	local conns="$6"
	local key_space_size="$7"
	local approx_seconds=$((total / rate))
	local artifact_prefix="${RUN_GROUP}-${scenario}-${step_name}"
	local log_path="${ARTIFACT_DIR}/${artifact_prefix}.log"
	local metrics_path="${ARTIFACT_DIR}/${artifact_prefix}.json"
	local graph_path="${ARTIFACT_DIR}/${RUN_GROUP}-${scenario}-elbow.html"
	local table_path="${ARTIFACT_DIR}/${RUN_GROUP}-${scenario}-summary.md"
	local status answer attempts

	if ! should_run_step "${scenario}" "${step_name}"; then
		return
	fi
	build_command "${scenario}" "${step_name}" "${rate}" "${total}" "${clients}" "${conns}" "${key_space_size}" "${metrics_path}" "${graph_path}" "${table_path}"

	echo "Next step: ${scenario}/${step_name}"
	echo "  rate=${rate}/sec total=${total} approx_duration=${approx_seconds}s clients=${clients} conns=${conns} key_space_size=${key_space_size}"
	echo "  log=${log_path}"
	echo "  metrics=${metrics_path}"
	printf "  command:"
	printf " %q" "${cmd_args[@]}"
	echo
	if [[ "${AUTO_RUN}" == "true" ]]; then
		answer=""
		echo "AUTO_RUN=true: running this step without prompting."
	else
		read -r -p "Press Enter to run this step, 's' to skip, or 'q' to quit: " answer
	fi

	case "${answer}" in
		q | Q)
			echo "Stopping before ${scenario}/${step_name}."
			exit 0
			;;
		s | S)
			echo "Skipping ${scenario}/${step_name}."
			echo
			return
			;;
	esac

	attempts=0
	while true; do
		clear_proxy_env
		if [[ "${attempts}" -eq 0 ]]; then
			echo "===== ${scenario}/${step_name} started $(date -u +"%Y-%m-%dT%H:%M:%SZ") =====" | tee "${log_path}"
		else
			echo "===== ${scenario}/${step_name} retry ${attempts} started $(date -u +"%Y-%m-%dT%H:%M:%SZ") =====" | tee -a "${log_path}"
		fi
		echo "Proxy environment variables unset for this step." | tee -a "${log_path}"
		running_step=true
		"${cmd_args[@]}" 2>&1 | tee -a "${log_path}"
		status="${PIPESTATUS[0]}"
		running_step=false
		trap handle_interrupt INT
		echo "===== ${scenario}/${step_name} finished $(date -u +"%Y-%m-%dT%H:%M:%SZ") status=${status} =====" | tee -a "${log_path}"

		if [[ "${status}" -eq 0 ]]; then
			break
		fi
		if [[ "${stop_requested}" == "true" ]]; then
			break
		fi
		if [[ "${RECOVER_ON_NOSPACE}" == "true" && "${attempts}" -lt "${RECOVER_ON_NOSPACE_MAX_RETRIES}" ]] && nospace_failure_seen "${log_path}"; then
			attempts=$((attempts + 1))
			if ! recover_from_nospace "${log_path}"; then
				echo "NOSPACE recovery failed. Stopping so we can inspect before applying more load." | tee -a "${log_path}" >&2
				break
			fi
			echo "Retrying ${scenario}/${step_name} after NOSPACE recovery." | tee -a "${log_path}"
			continue
		fi
		break
	done
	echo
	echo "Artifacts:"
	echo "  log:     ${log_path}"
	echo "  metrics: ${metrics_path}"
	echo "  graph:   ${graph_path}"
	echo "  table:   ${table_path}"
	echo

	if [[ "${status}" -ne 0 ]]; then
		echo "Step ${scenario}/${step_name} failed with status ${status}. Stopping so we can inspect before applying more load." >&2
		exit "${status}"
	fi
	if [[ "${stop_requested}" == "true" ]]; then
		echo "Stopping after interrupted step. Completed artifacts above are ready for inspection."
		exit 130
	fi
}

echo "KILT gentle benchmark ramp"
echo "  benchmark: ${BENCH_BIN}"
echo "  endpoints: ${ENDPOINTS}"
echo "  scenarios: ${SCENARIOS}"
echo "  remote metric hosts: ${REMOTE_METRICS_HOSTS}"
echo "  artifacts: ${ARTIFACT_DIR}"
echo "  full summary: ${FULL_SUMMARY_PATH}"
echo "  full summary html: ${FULL_SUMMARY_HTML_PATH}"
echo "  run group: ${RUN_GROUP}"
if [[ -n "${START_AT}" ]]; then
	echo "  start at: ${START_AT}"
fi
if [[ "${RECOVER_ON_NOSPACE}" == "true" ]]; then
	echo "  recover on NOSPACE: true"
	echo "  recover clean prefix: ${RECOVER_CLEAN_PREFIX}"
	echo "  recover max retries per step: ${RECOVER_ON_NOSPACE_MAX_RETRIES}"
fi
echo

clear_proxy_env
echo "Proxy environment variables unset for direct etcd/client metrics traffic."
echo

if scenario_enabled put; then
	for step in "${PUT_STEPS[@]}"; do
		IFS="|" read -r step_name rate total clients conns key_space_size <<<"${step}"
		run_step "put" "${step_name}" "${rate}" "${total}" "${clients}" "${conns}" "${key_space_size}"
	done
fi

if scenario_enabled txn-put; then
	for step in "${TXN_STEPS[@]}"; do
		IFS="|" read -r step_name rate total clients conns key_space_size <<<"${step}"
		run_step "txn-put" "${step_name}" "${rate}" "${total}" "${clients}" "${conns}" "${key_space_size}"
	done
fi

if scenario_enabled watch-latency; then
	for step in "${WATCH_STEPS[@]}"; do
		IFS="|" read -r step_name rate total clients conns key_space_size <<<"${step}"
		run_step "watch-latency" "${step_name}" "${rate}" "${total}" "${clients}" "${conns}" "${key_space_size}"
	done
fi

if scenario_enabled mixed; then
	for step in "${MIXED_STEPS[@]}"; do
		IFS="|" read -r step_name rate total clients conns key_space_size <<<"${step}"
		run_step "mixed" "${step_name}" "${rate}" "${total}" "${clients}" "${conns}" "${key_space_size}"
	done
fi

if scenario_enabled range; then
	for step in "${RANGE_STEPS[@]}"; do
		IFS="|" read -r step_name rate total clients conns key_space_size <<<"${step}"
		run_step "range" "${step_name}" "${rate}" "${total}" "${clients}" "${conns}" "${key_space_size}"
	done
fi

if [[ -n "${START_AT}" && "${start_reached}" != "true" ]]; then
	echo "START_AT=${START_AT} did not match any enabled scenario step." >&2
	exit 2
fi

echo "All requested gentle ramp steps completed."
