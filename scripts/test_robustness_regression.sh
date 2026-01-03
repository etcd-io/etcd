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
#
# Script to run all robustness regression tests
# Usage: ./scripts/test_robustness_regression.sh [OPTIONS]
#
# This script ensures that refactoring doesn't break the ability to reproduce
# previously discovered bugs, as documented in tests/robustness/README.md
# section "Maintaining Bug Reproducibility During Refactoring"

set -uo pipefail

# ============================================================================
# Configuration and Defaults
# ============================================================================

ETCD_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
source "${ETCD_ROOT}/scripts/test_utils.sh"

DEFAULT_RESULTS_DIR="/tmp/robustness-regression-results"
DEFAULT_TIMEOUT="45m"
DEFAULT_PARALLEL=1

# Test definitions: "TestName:MakeTarget:Description:Version"
declare -a REGRESSION_TESTS=(
  "Issue13766:test-robustness-issue13766:Inconsistent revision caused by crash during high load:v3.5.2"
  "Issue14370:test-robustness-issue14370:Single node cluster can lose a write on crash:v3.5.4"
  "Issue14685:test-robustness-issue14685:Inconsistent revision caused by crash during defrag:v3.5.5"
  "Issue15220:test-robustness-issue15220:Watch progress notification not synced with stream:v3.5.7"
  "Issue15271:test-robustness-issue15271:Watch traveling back in time after network partition:v3.5.7"
  "Issue17529:test-robustness-issue17529:Watch events lost during stream starvation:v3.5.12-patched"
  "Issue17780:test-robustness-issue17780:Revision decreasing caused by crash during compaction:v3.5.13-patched"
  "Issue18089:test-robustness-issue18089:Watch dropping an event when compacting on delete:v3.5.12-patched"
  "Issue19179:test-robustness-issue19179:Missing delete event on watch opened on same revision as compaction:v3.5.17"
)

# Global state
declare -a FAILED_TESTS=()
declare -a PASSED_TESTS=()
declare -a job_pids=()
declare -a job_names=()
declare -A job_status_files=()

# Configuration variables (set by parse_arguments)
RESULTS_DIR="${RESULTS_DIR:-$DEFAULT_RESULTS_DIR}"
PARALLEL_JOBS="${PARALLEL_JOBS:-$DEFAULT_PARALLEL}"
TEST_TIMEOUT="$DEFAULT_TIMEOUT"
VERBOSE=false
PERSIST_RESULTS_FLAG=false

# Check for timeout command availability
if command -v timeout >/dev/null 2>&1; then
  HAS_TIMEOUT=true
elif command -v gtimeout >/dev/null 2>&1; then
  HAS_TIMEOUT=true
  TIMEOUT_CMD="gtimeout"
else
  HAS_TIMEOUT=false
fi

# ============================================================================
# Helper Functions
# ============================================================================

function show_usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Run all robustness regression tests to ensure refactoring doesn't break
bug reproducibility.

Options:
  --parallel=N         Number of tests to run in parallel (default: 1)
  --results-dir=PATH   Directory to store test results
                       (default: /tmp/robustness-regression-results)
  --persist-results    Keep all test artifacts (passed and failed)
  --verbose            Enable verbose output
  --timeout=DURATION   Timeout per test, e.g., 45m, 1h (default: 45m)
  --help, -h           Show this help message

Environment Variables (preserved if set):
  EXPECT_DEBUG         Enable debug logs from etcd cluster
  GO_TEST_FLAGS        Additional flags passed to go test
  PERSIST_RESULTS      Keep all test artifacts (same as --persist-results)
  RESULTS_DIR          Alternative to --results-dir flag

Examples:
  # Sequential execution (default)
  ./scripts/test_robustness_regression.sh

  # Parallel execution with 3 jobs
  ./scripts/test_robustness_regression.sh --parallel=3

  # With custom results directory and verbose mode
  ./scripts/test_robustness_regression.sh --verbose --results-dir=/tmp/results

  # For CI with parallel execution
  ./scripts/test_robustness_regression.sh --parallel=4

Tests run by this script (from tests/robustness/Makefile):
EOF

  for test_config in "${REGRESSION_TESTS[@]}"; do
    local test_name=$(echo "$test_config" | cut -d: -f1)
    local description=$(echo "$test_config" | cut -d: -f3)
    local version=$(echo "$test_config" | cut -d: -f4)
    printf "  %-15s %-55s %s\n" "$test_name" "$description" "($version)"
  done

  echo ""
}

function parse_arguments() {
  while [[ $# -gt 0 ]]; do
    case $1 in
      --parallel=*)
        PARALLEL_JOBS="${1#*=}"
        shift
        ;;
      --results-dir=*)
        RESULTS_DIR="${1#*=}"
        shift
        ;;
      --persist-results)
        PERSIST_RESULTS_FLAG=true
        export PERSIST_RESULTS=true
        shift
        ;;
      --verbose)
        VERBOSE=true
        shift
        ;;
      --timeout=*)
        TEST_TIMEOUT="${1#*=}"
        shift
        ;;
      --help|-h)
        show_usage
        exit 0
        ;;
      *)
        log_error "Unknown option: $1"
        echo ""
        show_usage
        exit 2
        ;;
    esac
  done
}

function validate_arguments() {
  # Validate parallel jobs
  if ! [[ "$PARALLEL_JOBS" =~ ^[0-9]+$ ]] || [ "$PARALLEL_JOBS" -lt 1 ]; then
    log_error "Invalid --parallel value: $PARALLEL_JOBS (must be a positive integer)"
    exit 2
  fi

  # Check for make command
  if ! command -v make >/dev/null 2>&1; then
    log_error "make command not found. Please install make."
    exit 3
  fi
}

function setup_results_dir() {
  if [ ! -d "$RESULTS_DIR" ]; then
    if ! mkdir -p "$RESULTS_DIR"; then
      log_error "Failed to create results directory: $RESULTS_DIR"
      exit 3
    fi
  fi

  # Verify directory is writable
  if [ ! -w "$RESULTS_DIR" ]; then
    log_error "Results directory is not writable: $RESULTS_DIR"
    exit 3
  fi
}

function format_duration() {
  local seconds=$1
  local hours=$((seconds / 3600))
  local minutes=$(((seconds % 3600) / 60))
  local secs=$((seconds % 60))

  if [ $hours -gt 0 ]; then
    printf "%dh %dm %ds" $hours $minutes $secs
  elif [ $minutes -gt 0 ]; then
    printf "%dm %ds" $minutes $secs
  else
    printf "%ds" $secs
  fi
}

# ============================================================================
# Test Execution Functions
# ============================================================================

function show_test_progress() {
  local test_name="$1"
  local status="$2"  # "RUNNING", "PASSED", "FAILED"

  case "$status" in
    RUNNING)
      if [ "$PARALLEL_JOBS" -eq 1 ] && [ "$VERBOSE" = "true" ]; then
        log_callout "========================================="
        log_callout "Starting: $test_name"
        log_callout "========================================="
      else
        log_callout "[$(date +%H:%M:%S)] Started: $test_name"
      fi
      ;;
    PASSED)
      log_success "[$(date +%H:%M:%S)] ✓ PASSED: $test_name"
      ;;
    FAILED)
      log_error "[$(date +%H:%M:%S)] ✗ FAILED: $test_name"
      log_error "           Log: ${RESULTS_DIR}/${test_name}.log"
      ;;
  esac
}

function record_test_result() {
  local test_name="$1"
  local exit_code="$2"

  if [ "$exit_code" -eq 0 ]; then
    PASSED_TESTS+=("$test_name")
  else
    FAILED_TESTS+=("$test_name")
  fi
}

function run_single_test() {
  local test_config="$1"
  local test_name=$(echo "$test_config" | cut -d: -f1)
  local make_target=$(echo "$test_config" | cut -d: -f2)
  local description=$(echo "$test_config" | cut -d: -f3)
  local version=$(echo "$test_config" | cut -d: -f4)

  local log_file="${RESULTS_DIR}/${test_name}.log"
  local status_file="${RESULTS_DIR}/${test_name}.status"

  # Record start time
  local start_time=$(date +%s)
  {
    echo "START:$start_time"
    echo "TEST:$test_name"
    echo "VERSION:$version"
    echo "DESCRIPTION:$description"
  } > "$status_file"

  show_test_progress "$test_name" "RUNNING"

  # Execute make target with timeout
  local exit_code=0
  if [ "$HAS_TIMEOUT" = "true" ]; then
    local timeout_cmd="${TIMEOUT_CMD:-timeout}"
    if [ "$PARALLEL_JOBS" -eq 1 ] && [ "$VERBOSE" = "true" ]; then
      # Sequential + verbose: stream output
      $timeout_cmd --preserve-status "${TEST_TIMEOUT}" \
        make -C "$ETCD_ROOT" -f "${ETCD_ROOT}/tests/robustness/Makefile" \
        "$make_target" 2>&1 | tee "$log_file"
      exit_code=${PIPESTATUS[0]}
    else
      # Parallel or non-verbose: buffer to file
      $timeout_cmd --preserve-status "${TEST_TIMEOUT}" \
        make -C "$ETCD_ROOT" -f "${ETCD_ROOT}/tests/robustness/Makefile" \
        "$make_target" > "$log_file" 2>&1
      exit_code=$?
    fi
  else
    # No timeout available
    if [ "$PARALLEL_JOBS" -eq 1 ] && [ "$VERBOSE" = "true" ]; then
      make -C "$ETCD_ROOT" -f "${ETCD_ROOT}/tests/robustness/Makefile" \
        "$make_target" 2>&1 | tee "$log_file"
      exit_code=${PIPESTATUS[0]}
    else
      make -C "$ETCD_ROOT" -f "${ETCD_ROOT}/tests/robustness/Makefile" \
        "$make_target" > "$log_file" 2>&1
      exit_code=$?
    fi
  fi

  local end_time=$(date +%s)
  local duration=$((end_time - start_time))

  # Record results
  {
    echo "END:$end_time"
    echo "DURATION:$duration"
    echo "EXIT_CODE:$exit_code"
  } >> "$status_file"

  # Show progress
  if [ $exit_code -eq 0 ]; then
    show_test_progress "$test_name" "PASSED"
  else
    show_test_progress "$test_name" "FAILED"
  fi

  record_test_result "$test_name" "$exit_code"

  return $exit_code
}

function run_tests_sequential() {
  log_callout "Running tests sequentially..."
  echo ""

  for test_config in "${REGRESSION_TESTS[@]}"; do
    run_single_test "$test_config"
    echo ""
  done
}

function run_tests_parallel() {
  log_callout "Running tests with parallelism: $PARALLEL_JOBS"
  echo ""

  local active_jobs=0

  for test_config in "${REGRESSION_TESTS[@]}"; do
    # Wait if at max capacity
    while [ $active_jobs -ge $PARALLEL_JOBS ]; do
      # Check for completed jobs
      for i in "${!job_pids[@]}"; do
        local pid="${job_pids[$i]}"
        if ! kill -0 "$pid" 2>/dev/null; then
          # Job finished, collect status
          wait "$pid" 2>/dev/null || true
          # Result already recorded in run_single_test
          unset 'job_pids[$i]'
          unset 'job_names[$i]'
          ((active_jobs--))
        fi
      done
      sleep 0.5  # Brief pause before rechecking
    done

    # Start new job in background
    run_single_test "$test_config" &
    local pid=$!
    job_pids+=("$pid")
    local test_name=$(echo "$test_config" | cut -d: -f1)
    job_names+=("$test_name")
    ((active_jobs++))
  done

  # Wait for all remaining jobs
  for pid in "${job_pids[@]}"; do
    wait "$pid" 2>/dev/null || true
  done

  echo ""
}

# ============================================================================
# Output Functions
# ============================================================================

function generate_summary() {
  local summary_file="${RESULTS_DIR}/summary.txt"
  local total_tests=${#REGRESSION_TESTS[@]}
  local passed_count=${#PASSED_TESTS[@]}
  local failed_count=${#FAILED_TESTS[@]}
  local total_duration=0

  # Calculate total duration
  for test_config in "${REGRESSION_TESTS[@]}"; do
    local test_name=$(echo "$test_config" | cut -d: -f1)
    local status_file="${RESULTS_DIR}/${test_name}.status"
    if [ -f "$status_file" ]; then
      local duration=$(grep "^DURATION:" "$status_file" | cut -d: -f2)
      if [ -n "$duration" ]; then
        total_duration=$((total_duration + duration))
      fi
    fi
  done

  # Generate table
  {
    echo "================================================================================"
    echo "                  ROBUSTNESS REGRESSION TEST SUMMARY"
    echo "================================================================================"
    printf "%-20s %-10s %-12s %s\n" "Test Name" "Status" "Duration" "Version"
    echo "--------------------------------------------------------------------------------"

    for test_config in "${REGRESSION_TESTS[@]}"; do
      local test_name=$(echo "$test_config" | cut -d: -f1)
      local version=$(echo "$test_config" | cut -d: -f4)
      local status_file="${RESULTS_DIR}/${test_name}.status"

      if [ -f "$status_file" ]; then
        local exit_code=$(grep "^EXIT_CODE:" "$status_file" | cut -d: -f2)
        local duration=$(grep "^DURATION:" "$status_file" | cut -d: -f2)
        local duration_fmt=$(format_duration "$duration")

        if [ "$exit_code" = "0" ]; then
          printf "%-20s %-10s %-12s %s\n" "$test_name" "PASSED" "$duration_fmt" "$version"
        else
          printf "%-20s %-10s %-12s %s\n" "$test_name" "FAILED" "$duration_fmt" "$version"
        fi
      else
        printf "%-20s %-10s %-12s %s\n" "$test_name" "PENDING" "-" "$version"
      fi
    done

    echo "--------------------------------------------------------------------------------"
    printf "Total: %-2d  Passed: %-2d  Failed: %-2d  Total Time: %s\n" \
      "$total_tests" "$passed_count" "$failed_count" "$(format_duration "$total_duration")"
    echo "================================================================================"

    if [ "$failed_count" -gt 0 ]; then
      echo ""
      echo "Failed Tests Details:"
      for test_name in "${FAILED_TESTS[@]}"; do
        echo "  - $test_name: ${RESULTS_DIR}/${test_name}.log"
      done
      echo ""
      echo "Overall Result: FAILED ($failed_count test(s) failed)"
      echo "Exit Code: 1"
    else
      echo ""
      echo "Overall Result: SUCCESS"
      echo "Exit Code: 0"
    fi
    echo "================================================================================"
    echo ""
    echo "Results saved to: $RESULTS_DIR"
  } | tee "$summary_file"
}

# ============================================================================
# Signal Handling
# ============================================================================

function cleanup() {
  log_warning "\nReceived interrupt signal. Cleaning up..."

  # Kill all background jobs if running in parallel
  if [ "${#job_pids[@]}" -gt 0 ]; then
    for pid in "${job_pids[@]}"; do
      kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
  fi

  # Generate partial summary if possible
  log_callout "\nGenerating partial summary..."
  echo ""
  generate_summary

  exit 130
}

# Register signal handlers
trap cleanup INT TERM

# ============================================================================
# Main Execution
# ============================================================================

function main() {
  parse_arguments "$@"
  validate_arguments
  setup_results_dir

  log_callout "================================================================================"
  log_callout "             ROBUSTNESS REGRESSION TEST SUITE"
  log_callout "================================================================================"
  log_callout "Tests to run:    ${#REGRESSION_TESTS[@]}"
  log_callout "Parallel jobs:   $PARALLEL_JOBS"
  log_callout "Results dir:     $RESULTS_DIR"
  log_callout "Test timeout:    $TEST_TIMEOUT"
  if [ "$HAS_TIMEOUT" = "false" ]; then
    log_warning "Note: timeout command not available, tests will run without time limit"
  fi
  log_callout "================================================================================"
  echo ""

  # Run tests (parallel or sequential)
  if [ "$PARALLEL_JOBS" -eq 1 ]; then
    run_tests_sequential
  else
    run_tests_parallel
  fi

  # Generate summary
  generate_summary

  # Exit with appropriate code
  if [ "${#FAILED_TESTS[@]}" -gt 0 ]; then
    exit 1
  else
    exit 0
  fi
}

main "$@"
