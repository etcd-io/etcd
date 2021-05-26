#!/usr/bin/env bash

ROOT_MODULE="go.etcd.io/etcd"

if [[ "$(go list)" != "${ROOT_MODULE}/v3" ]]; then
  echo "must be run from '${ROOT_MODULE}/v3' module directory"
  exit 255
fi

function set_root_dir {
  ETCD_ROOT_DIR=$(go list -f '{{.Dir}}' "${ROOT_MODULE}/v3")
}

set_root_dir

####   Convenient IO methods #####

COLOR_RED='\033[0;31m'
COLOR_ORANGE='\033[0;33m'
COLOR_GREEN='\033[0;32m'
COLOR_LIGHTCYAN='\033[0;36m'
COLOR_BLUE='\033[0;94m'
COLOR_MAGENTA='\033[95m'
COLOR_BOLD='\033[1m'
COLOR_NONE='\033[0m' # No Color


function log_error {
  >&2 echo -n -e "${COLOR_BOLD}${COLOR_RED}"
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

function log_cmd {
  >&2 echo -n -e "${COLOR_BLUE}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_success {
  >&2 echo -n -e "${COLOR_GREEN}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

function log_info {
  >&2 echo -n -e "${COLOR_NONE}"
  >&2 echo "$@"
  >&2 echo -n -e "${COLOR_NONE}"
}

# From http://stackoverflow.com/a/12498485
function relativePath {
  # both $1 and $2 are absolute paths beginning with /
  # returns relative path to $2 from $1
  local source=$1
  local target=$2

  local commonPart=$source
  local result=""

  while [[ "${target#$commonPart}" == "${target}" ]]; do
    # no match, means that candidate common part is not correct
    # go up one level (reduce common part)
    commonPart="$(dirname "$commonPart")"
    # and record that we went back, with correct / handling
    if [[ -z $result ]]; then
      result=".."
    else
      result="../$result"
    fi
  done

  if [[ $commonPart == "/" ]]; then
    # special case for root (no common path)
    result="$result/"
  fi

  # since we now have identified the common part,
  # compute the non-common part
  local forwardPart="${target#$commonPart}"

  # and now stick all parts together
  if [[ -n $result ]] && [[ -n $forwardPart ]]; then
    result="$result$forwardPart"
  elif [[ -n $forwardPart ]]; then
    # extra slash removal
    result="${forwardPart:1}"
  fi

  echo "$result"
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

# Prints subdirectory (from the repo root) for the current module.
function module_subdir {
  relativePath "${ETCD_ROOT_DIR}" "${PWD}"
}

####    Running actions against multiple modules ####

# run [command...] - runs given command, printing it first and
# again if it failed (in RED). Use to wrap important test commands
# that user might want to re-execute to shorten the feedback loop when fixing
# the test.
function run {
  local rpath
  local command
  rpath=$(module_subdir)
  # Quoting all components as the commands are fully copy-parsable:
  command=("${@}")
  command=("${command[@]@Q}")
  if [[ "${rpath}" != "." && "${rpath}" != "" ]]; then
    repro="(cd ${rpath} && ${command[*]})"
  else 
    repro="${command[*]}"
  fi

  log_cmd "% ${repro}"
  "${@}" 2> >(while read -r line; do echo -e "${COLOR_NONE}stderr: ${COLOR_MAGENTA}${line}${COLOR_NONE}">&2; done)
  local error_code=$?
  if [ ${error_code} -ne 0 ]; then
    log_error -e "FAIL: (code:${error_code}):\\n  % ${repro}"
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
    cd "${ETCD_ROOT_DIR}/${module}" && "$@"
  )
}

function module_dirs() {
  echo "api pkg raft client/pkg client/v2 client/v3 server etcdutl etcdctl tests ."
}

# maybe_run [cmd...] runs given command depending on the DRY_RUN flag.
function maybe_run() {
  if ${DRY_RUN}; then
    log_warning -e "# DRY_RUN:\\n  % ${*}"
  else
    run "${@}"
  fi
}

function modules() {
  modules=(
    "${ROOT_MODULE}/api/v3"
    "${ROOT_MODULE}/pkg/v3"
    "${ROOT_MODULE}/raft/v3"
    "${ROOT_MODULE}/client/pkg/v3"
    "${ROOT_MODULE}/client/v2"
    "${ROOT_MODULE}/client/v3"
    "${ROOT_MODULE}/server/v3"
    "${ROOT_MODULE}/etcdutl/v3"
    "${ROOT_MODULE}/etcdctl/v3"
    "${ROOT_MODULE}/tests/v3"
    "${ROOT_MODULE}/v3")
  echo "${modules[@]}"
}

function modules_exp() {
  for m in $(modules); do
    echo -n "${m}/... "
  done
}

#  run_for_modules [cmd]
#  run given command across all modules and packages
#  (unless the set is limited using ${PKG} or / ${USERMOD})
function run_for_modules {
  local pkg="${PKG:-./...}"
  if [ -z "${USERMOD:-}" ]; then
    for m in $(module_dirs); do
      run_for_module "${m}" "$@" "${pkg}" || return "$?"
    done
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

    # shellcheck disable=SC2086
    if ! run env ${goTestEnv} "${cmd[@]}" ; then
      if [ "${mode}" != "keep_going" ]; then
        return 2
      else
        failures=("${failures[@]}" "${pkg}")
      fi
    fi
  done

  if [ -n "${failures[*]}" ] ; then
    log_error -e "ERROR: Tests for following packages failed:\\n  ${failures[*]}"
    return 2
  fi
}

#### Other ####

# tool_exists [tool] [instruction]
# Checks whether given [tool] is installed. In case of failure,
# prints a warning with installation [instruction] and returns !=0 code.
#
# WARNING: This depend on "any" version of the 'binary' that might be tricky
# from hermetic build perspective. For go binaries prefer 'tool_go_run'
function tool_exists {
  local tool="${1}"
  local instruction="${2}"
  if ! command -v "${tool}" >/dev/null; then
    log_warning "Tool: '${tool}' not found on PATH. ${instruction}"
    return 255
  fi
}

# Ensure gobin is available, as it runs majority of the tools
if ! command -v "gobin" >/dev/null; then
    run env GO111MODULE=off go get github.com/myitcv/gobin || exit 1
fi

# tool_get_bin [tool] - returns absolute path to a tool binary (or returns error)
function tool_get_bin {
  tool_exists "gobin" "GO111MODULE=off go get github.com/myitcv/gobin" || return 2

  local tool="$1"
  if [[ "$tool" == *"@"* ]]; then
    # shellcheck disable=SC2086
    run gobin ${GOBINARGS:-} -p "${tool}" || return 2
  else
    # shellcheck disable=SC2086
    run_for_module ./tools/mod run gobin ${GOBINARGS:-} -p -m --mod=readonly "${tool}" || return 2
  fi
}

# tool_pkg_dir [pkg] - returns absolute path to a directory that stores given pkg.
# The pkg versions must be defined in ./tools/mod directory.
function tool_pkg_dir {
  run_for_module ./tools/mod run go list -f '{{.Dir}}' "${1}"
}

# tool_get_bin [tool]
function run_go_tool {
  local cmdbin
  if ! cmdbin=$(tool_get_bin "${1}"); then
    return 2
  fi
  shift 1
  run "${cmdbin}" "$@" || return 2
}

# assert_no_git_modifications fails if there are any uncommited changes.
function assert_no_git_modifications {
  log_callout "Making sure everything is committed."
  if ! git diff --cached --exit-code; then
    log_error "Found staged by uncommited changes. Do commit/stash your changes first."
    return 2
  fi
  if ! git diff  --exit-code; then
    log_error "Found unstaged and uncommited changes. Do commit/stash your changes first."
    return 2
  fi
}

# makes sure that the current branch is in sync with the origin branch:
#  - no uncommitted nor unstaged changes
#  - no differencing commits in relation to the origin/$branch
function git_assert_branch_in_sync {
  local branch
  branch=$(run git rev-parse --abbrev-ref HEAD)
  # TODO: When git 2.22 popular, change to:
  # branch=$(git branch --show-current)
  if [[ $(run git status --porcelain --untracked-files=no) ]]; then
    log_error "The workspace in '$(pwd)' for branch: ${branch} has uncommitted changes"
    log_error "Consider cleaning up / renaming this directory or (cd $(pwd) && git reset --hard)"
    return 2
  fi
  if [ -n "${branch}" ]; then
    ref_local=$(run git rev-parse "${branch}")
    ref_origin=$(run git rev-parse "origin/${branch}")
    if [ "x${ref_local}" != "x${ref_origin}" ]; then
      log_error "In workspace '$(pwd)' the branch: ${branch} diverges from the origin."
      log_error "Consider cleaning up / renaming this directory or (cd $(pwd) && git reset --hard origin/${branch})"
      return 2
    fi
  else
    log_warning "Cannot verify consistency with the origin, as git is on detached branch."
  fi
}
