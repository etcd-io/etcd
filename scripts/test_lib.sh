#!/usr/bin/env bash

ETCD_ROOT_DIR="$(pwd)"

####   Convenient IO methods #####

COLOR_RED='\033[0;31m'
COLOR_ORANGE='\033[0;33m'
COLOR_GREEN='\033[0;32m'
COLOR_LIGHTCYAN='\033[0;36m'

COLOR_NONE='\033[0m' # No Color

function log_error {
  >&2 echo -n -e "${COLOR_RED}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_warning {
  >&2 echo -n -e "${COLOR_ORANGE}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_callout {
  >&2 echo -n -e "${COLOR_LIGHTCYAN}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_success {
  >&2 echo -n -e "${COLOR_GREEN}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}


####   Discovery of files/packages within a go module #####

# go_srcs_in_module [package]
# returns list of all not-generated go sources in the current (dir) module.
function go_srcs_in_module {
  go fmt -n "$1"  | grep -Eo "([^ ]*)$" | grep -vE "(\\_test.go|\\.pb\\.go|\\.pb\\.gw.go)"
}

# pkgs_in_module [optional:package_pattern]
# returns list of all packages in the current (dir) module.
# if the package_pattern is given, its being resolved.
function pkgs_in_module {
  go list -mod=mod "${1:-./...}";
}

####    Running actions against multiple modules ####

# run [command...] - runs given command, printing it first and
# again if it failed (in RED). Use to wrap important test commands
# that user might want to re-execute to shorten the feedback loop when fixing
# the test.
function run {
  local rpath
  rpath=$(realpath "--relative-to=${ETCD_ROOT_DIR}" "${PWD}")
  local repro="$*"
  if [ "${rpath}" != "." ]; then
    repro="(cd ${rpath} && ${repro})"
  fi

  log_callout "% ${repro}"
  "${@}"
  local error_code=$?
  if [ ${error_code} -ne 0 ]; then
    log_error -e "FAIL: (code:${error_code}):\n  % ${repro}"
    return ${error_code}
  fi
}

# run_for_module [module] [cmd]
# executes given command in the given module for given pkgs.
#   module_name - "." (in future: tests, client, server)
#   cmd         - cmd to be executed - that takes package as last argument
function run_for_module {
  local module=${1:-"."}
  shift 1
  (
    cd "${module}" && "$@"
  )
}

#  run_for_modules [cmd]
#  run given command across all modules and packages
#  (unless the set is limited using ${PKG} or / ${USERMOD})
function run_for_modules {
  local pkg="${PKG:-./...}"
  if [ -z "${USERMOD}" ]; then
    run_for_module "api" "$@" "${pkg}" || return "$?"
    run_for_module "." "$@" "${pkg}" || return "$?"
    run_for_module "tests" "$@" "${pkg}" || return "$?"
  else
    run_for_module "${USERMOD}" "$@" "${pkg}" || return "$?"
  fi
}


####    Running go test  ########

# go_test [packages] [mode] [flags_for_package_func] [$@]
# [mode] supports 3 states:
#   - "parallel": fastest as concurrently processes multiple packages, but silent
#                 till the last package. See: https://github.com/golang/go/issues/2731
#   - "keep_going" : executes tests package by package, but postpones reporting error to the last
#   - "fail_fast"  : executes tests packages 1 by 1, exits on the first failure.
#
# [flags_for_package_func] is a name of function that takes list of packages as parameter
#   and computes additional flags to the go_test commands.
#   Use 'true' or ':' if you dont need additional arguments.
#
#  depends on the VERBOSE top-level variable.
#
#  Example:
#    go_test "./..." "keep_going" ":" --short
#
#  The function returns != 0 code in case of test failure.
function go_test {
  local packages="${1}"
  local mode="${2}"
  local flags_for_package_func="${3}"

  shift 3

  local goTestFlags=""
  local goTestEnv=""
  if [ "${VERBOSE}" == "1" ]; then
    goTestFlags="-v"
  fi
  if [ "${VERBOSE}" == "2" ]; then
    goTestFlags="-v"
    goTestEnv="CLIENT_DEBUG=1"
  fi

  # Expanding patterns (like ./...) into list of packages

  local unpacked_packages=("${packages}")
  if [ "${mode}" != "parallel" ]; then
    # shellcheck disable=SC2207
    # shellcheck disable=SC2086
    if ! unpacked_packages=($(go list ${packages})); then
      log_error "Cannot resolve packages: ${packages}"
      return 255
    fi
  fi

  local failures=""

  # execution of tests against packages:
  for pkg in "${unpacked_packages[@]}"; do
    local additional_flags
    # shellcheck disable=SC2086
    additional_flags=$(${flags_for_package_func} ${pkg})

    # shellcheck disable=SC2206
    local cmd=( go test ${goTestFlags} ${additional_flags} "$@" ${pkg} )

    if ! run env ${goTestEnv} "${cmd[@]}" ; then
      if [ "${mode}" != "keep_going" ]; then
        return 2
      else
        failures=("${failures[@]}" "${pkg}")
      fi
    fi
    echo
  done

  if [ -n "${failures[*]}" ] ; then
    log_error -e "ERROR: Tests for following packages failed:\n  ${failures[*]}"
    return 2
  fi
}

#### Other ####

# tool_exists [tool] [instruction]
# Checks whether given [tool] is installed. In case of failure,
# prints a warning with installation [instruction] and returns !=0 code.
function tool_exists {
  local tool="${1}"
  local instruction="${2}"
  if ! command -v "${tool}" >/dev/null; then
    log_warning "Tool: '${tool}' not found on PATH. ${instruction}"
    return 255
  fi
}

