#!/usr/bin/env bash
#
# Run only e2e tests

# $ PASSES=e2e ./scripts/test_e2e.sh
set -e

# Consider command as failed when any component of the pipe fails:
# https://stackoverflow.com/questions/1221833/pipe-output-and-capture-exit-status-in-bash
set -o pipefail
set -o nounset

# The test script is not supposed to make any changes to the files
# e.g. add/update missing dependencies. Such divergences should be
# detected and trigger a failure that needs explicit developer's action.
export GOFLAGS=-mod=readonly
export ETCD_VERIFY=all

source ./scripts/test_lib.sh
source ./scripts/build_lib.sh

OUTPUT_FILE=${OUTPUT_FILE:-""}

if [ -n "${OUTPUT_FILE}" ]; then
  log_callout "Dumping output to: ${OUTPUT_FILE}"
  exec > >(tee -a "${OUTPUT_FILE}") 2>&1
fi

PASSES=${PASSES:-"gofmt bom dep build unit"}
KEEP_GOING_SUITE=${KEEP_GOING_SUITE:-false}
PKG=${PKG:-}
SHELLCHECK_VERSION=${SHELLCHECK_VERSION:-"v0.8.0"}

if [ -z "${GOARCH:-}" ]; then
  GOARCH=$(go env GOARCH);
fi

# determine whether target supports race detection
if [ -z "${RACE:-}" ] ; then
  if [ "$GOARCH" == "amd64" ] || [ "$GOARCH" == "arm64" ]; then
    RACE="--race"
  else
    RACE="--race=false"
  fi
else
  RACE="--race=${RACE:-true}"
fi

# This options make sense for cases where SUT (System Under Test) is compiled by test.
COMMON_TEST_FLAGS=("${RACE}")
if [[ -n "${CPU:-}" ]]; then
  COMMON_TEST_FLAGS+=("--cpu=${CPU}")
fi

log_callout "Running with ${COMMON_TEST_FLAGS[*]}"

RUN_ARG=()
if [ -n "${TESTCASE:-}" ]; then
  RUN_ARG=("-run=${TESTCASE}")
fi

################Run e2e tests###############################################
function e2e_pass {
  # e2e tests are running pre-build binary. Settings like --race,-cover,-cpu does not have any impact.
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./e2e/..." "keep_going" : -timeout="${TIMEOUT:-30m}" ${RUN_ARG[@]:-} "$@" || return $?
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./common/..." "keep_going" : --tags=e2e -timeout="${TIMEOUT:-30m}" ${RUN_ARG[@]:-} "$@"
}
########### MAIN ###############################################################
function run_pass {
  local pass="${1}"
  shift 1
  log_callout -e "\\n'${pass}' started at $(date)"
  if "${pass}_pass" "$@" ; then
    log_success "'${pass}' PASSED and completed at $(date)"
    return 0
  else
    log_error "FAIL: '${pass}' FAILED at $(date)"
    if [ "$KEEP_GOING_SUITE" = true ]; then
      return 2
    else
      exit 255
    fi
  fi
}

log_callout "Starting at: $(date)"
fail_flag=false

  if run_pass "${PASSES}" "${@}"; then
    :
  else
    fail_flag=true
  fi

if [ "$fail_flag" = true ]; then
  log_error "There was FAILURE in the e2e test suite run. Look above log detail"
  exit 255
fi

log_success "SUCCESS"