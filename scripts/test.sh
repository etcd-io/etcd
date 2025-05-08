#!/usr/bin/env bash
#
# Run all etcd tests
# ./scripts/test.sh
# ./scripts/test.sh -v
#
#
# Run specified test pass
#
# $ PASSES=unit ./scripts/test.sh
# $ PASSES=integration ./scripts/test.sh
#
#
# Run tests for one package
# Each pass has different default timeout, if you just run tests in one package or 1 test case then you can set TIMEOUT
# flag for different expectation
#
# $ PASSES=unit PKG=./wal TIMEOUT=1m ./scripts/test.sh
# $ PASSES=integration PKG=./clientv3 TIMEOUT=1m ./scripts/test.sh
#
# Run specified unit tests in one package
# To run all the tests with prefix of "TestNew", set "TESTCASE=TestNew ";
# to run only "TestNew", set "TESTCASE="\bTestNew\b""
#
# $ PASSES=unit PKG=./wal TESTCASE=TestNew TIMEOUT=1m ./scripts/test.sh
# $ PASSES=unit PKG=./wal TESTCASE="\bTestNew\b" TIMEOUT=1m ./scripts/test.sh
# $ PASSES=integration PKG=./client/integration TESTCASE="\bTestV2NoRetryEOF\b" TIMEOUT=1m ./scripts/test.sh
#
# KEEP_GOING_SUITE must be set to true to keep going with the next suite execution, passed to PASSES variable when there is a failure
# in a particular suite.
# KEEP_GOING_MODULE must be set to true to keep going with execution when there is failure in any module.
#
# Run code coverage
# COVERDIR must either be a absolute path or a relative path to the etcd root
# $ COVERDIR=coverage PASSES="build cov" ./scripts/test.sh
# $ go tool cover -html ./coverage/cover.out
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
SHELLCHECK_VERSION=${SHELLCHECK_VERSION:-"v0.10.0"}
MARKDOWN_MARKER_VERSION=${MARKDOWN_MARKER_VERSION:="v0.10.0"}

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

function build_pass {
  log_callout "Building etcd"
  run_for_modules run go build "${@}" || return 2
  GO_BUILD_FLAGS="-v" etcd_build "${@}"
  GO_BUILD_FLAGS="-v" tools_build "${@}"
}

################# REGULAR TESTS ################################################

# run_unit_tests [pkgs] runs unit tests for a current module and givesn set of [pkgs]
function run_unit_tests {
  local pkgs="${1:-./...}"
  shift 1
  # shellcheck disable=SC2068 #For context see - https://github.com/etcd-io/etcd/pull/16433#issuecomment-1684312755
  GOLANG_TEST_SHORT=true go_test "${pkgs}" "parallel" : -short -timeout="${TIMEOUT:-3m}" ${COMMON_TEST_FLAGS[@]:-} ${RUN_ARG[@]:-} "$@"
}

function unit_pass {
  run_for_modules run_unit_tests "$@"
}

function integration_extra {
  if [ -z "${PKG}" ] ; then
    # shellcheck disable=SC2068
    run_for_module "tests"  go_test "./integration/v2store/..." "keep_going" : -timeout="${TIMEOUT:-5m}" ${COMMON_TEST_FLAGS[@]:-} ${RUN_ARG[@]:-} "$@" || return $?
  else
    log_warning "integration_extra ignored when PKG is specified"
  fi
}

function integration_pass {
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./integration/..." "parallel" : -timeout="${TIMEOUT:-15m}" ${COMMON_TEST_FLAGS[@]:-} ${RUN_ARG[@]:-} -p=2 "$@" || return $?
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./common/..." "parallel" : --tags=integration -timeout="${TIMEOUT:-15m}" ${COMMON_TEST_FLAGS[@]:-} ${RUN_ARG[@]:-} -p=2 "$@" || return $?
  integration_extra "$@"
}

function e2e_pass {
  # e2e tests are running pre-build binary. Settings like --race,-cover,-cpu does not have any impact.
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./e2e/..." "keep_going" : -timeout="${TIMEOUT:-30m}" ${RUN_ARG[@]:-} "$@" || return $?
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./common/..." "keep_going" : --tags=e2e -timeout="${TIMEOUT:-30m}" ${RUN_ARG[@]:-} "$@"
}

function robustness_pass {
  # e2e tests are running pre-build binary. Settings like --race,-cover,-cpu does not have any impact.
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./robustness" "keep_going" : -timeout="${TIMEOUT:-30m}" ${RUN_ARG[@]:-} "$@"
}

function integration_e2e_pass {
  run_pass "integration" "${@}"
  run_pass "e2e" "${@}"
}

# generic_checker [cmd...]
# executes given command in the current module, and clearly fails if it
# failed or returned output.
function generic_checker {
  local cmd=("$@")
  if ! output=$("${cmd[@]}"); then
    echo "${output}"
    log_error -e "FAIL: '${cmd[*]}' checking failed (!=0 return code)"
    return 255
  fi
  if [ -n "${output}" ]; then
    echo "${output}"
    log_error -e "FAIL: '${cmd[*]}' checking failed (printed output)"
    return 255
  fi
}

function grpcproxy_pass {
  run_pass "grpcproxy_integration" "${@}"
  run_pass "grpcproxy_e2e" "${@}"
}

function grpcproxy_integration_pass {
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./integration/..." "fail_fast" : -timeout=30m -tags cluster_proxy ${COMMON_TEST_FLAGS[@]:-} "$@"
}

function grpcproxy_e2e_pass {
  # shellcheck disable=SC2068
  run_for_module "tests" go_test "./e2e" "fail_fast" : -timeout=30m -tags cluster_proxy ${COMMON_TEST_FLAGS[@]:-} "$@"
}

################# COVERAGE #####################################################

# pkg_to_coverflag [prefix] [pkgs]
# produces name of .coverprofile file to be used for tests of this package
function pkg_to_coverprofileflag {
  local prefix="${1}"
  local pkgs="${2}"
  local pkgs_normalized
  prefix_normalized=$(echo "${prefix}" | tr "./ " "__+")
  if [ "${pkgs}" == "./..." ]; then
    pkgs_normalized="all"
  else
    pkgs_normalized=$(echo "${pkgs}" | tr "./ " "__+")
  fi
  mkdir -p "${coverdir}/${prefix_normalized}"
  echo -n "-coverprofile=${coverdir}/${prefix_normalized}/${pkgs_normalized}.coverprofile"
}

function not_test_packages {
  for m in $(modules); do
    if [[ $m =~ .*/etcd/tests/v3 ]]; then continue; fi
    if [[ $m =~ .*/etcd/v3 ]]; then continue; fi
    echo "${m}/..."
  done
}

# split_dir [dir] [num]
function split_dir {
  local d="${1}"
  local num="${2}"
  local i=0
  for f in "${d}/"*; do
    local g=$(( i % num ))
    mkdir -p "${d}_${g}"
    mv "${f}" "${d}_${g}/"
    (( i++ ))
  done
}

function split_dir_pass {
  split_dir ./covdir/integration 4
}


# merge_cov_files [coverdir] [outfile]
# merges all coverprofile files into a single file in the given directory.
function merge_cov_files {
  local coverdir="${1}"
  local cover_out_file="${2}"
  log_callout "Merging coverage results in: ${coverdir}"
  # gocovmerge requires not-empty test to start with:
  echo "mode: set" > "${cover_out_file}"

  local i=0
  local count
  count=$(find "${coverdir}"/*.coverprofile | wc -l)
  for f in "${coverdir}"/*.coverprofile; do
    # print once per 20 files
    if ! (( "${i}" % 20 )); then
      log_callout "${i} of ${count}: Merging file: ${f}"
    fi
    run_go_tool "github.com/alexfalkowski/gocovmerge" "${f}" "${cover_out_file}"  > "${coverdir}/cover.tmp" 2>/dev/null
    if [ -s "${coverdir}"/cover.tmp ]; then
      mv "${coverdir}/cover.tmp" "${cover_out_file}"
    fi
    (( i++ ))
  done
}

# merge_cov [coverdir]
function merge_cov {
  log_callout "[$(date)] Merging coverage files ..."
  coverdir="${1}"
  for d in "${coverdir}"/*/; do
    d=${d%*/}  # remove the trailing "/"
    merge_cov_files "${d}" "${d}.coverprofile" &
  done
  wait
  merge_cov_files "${coverdir}" "${coverdir}/all.coverprofile"
}

# https://docs.codecov.com/docs/unexpected-coverage-changes#reasons-for-indirect-changes
function cov_pass {
  # shellcheck disable=SC2153
  if [ -z "${COVERDIR:-}" ]; then
    log_error "COVERDIR undeclared"
    return 255
  fi

  local coverdir
  coverdir=$(readlink -f "${COVERDIR}")
  mkdir -p "${coverdir}"
  find "${coverdir}" -print0 -name '*.coverprofile' | xargs -0 rm

  local covpkgs
  covpkgs=$(not_test_packages)
  local coverpkg_comma
  coverpkg_comma=$(echo "${covpkgs[@]}" | xargs | tr ' ' ',')
  local gocov_build_flags=("-covermode=set" "-coverpkg=$coverpkg_comma")

  local failed=""

  log_callout "[$(date)] Collecting coverage from unit tests ..."
  for m in $(module_dirs); do
    GOLANG_TEST_SHORT=true run_for_module "${m}" go_test "./..." "parallel" "pkg_to_coverprofileflag unit_${m}" -short -timeout=30m \
       "${gocov_build_flags[@]}" "$@" || failed="$failed unit"
  done

  log_callout "[$(date)] Collecting coverage from integration tests ..."
  run_for_module "tests" go_test "./integration/..." "parallel" "pkg_to_coverprofileflag integration" \
      -timeout=30m "${gocov_build_flags[@]}" "$@" || failed="$failed integration"
  # integration-store-v2
  run_for_module "tests" go_test "./integration/v2store/..." "keep_going" "pkg_to_coverprofileflag store_v2" \
      -timeout=5m "${gocov_build_flags[@]}" "$@" || failed="$failed integration_v2"
  # integration_cluster_proxy
  run_for_module "tests" go_test "./integration/..." "parallel" "pkg_to_coverprofileflag integration_cluster_proxy" \
      -tags cluster_proxy -timeout=30m "${gocov_build_flags[@]}" || failed="$failed integration_cluster_proxy"

  local cover_out_file="${coverdir}/all.coverprofile"
  merge_cov "${coverdir}"

  # strip out generated files (using GNU-style sed)
  sed --in-place -E "/[.]pb[.](gw[.])?go/d" "${cover_out_file}" || true

  sed --in-place -E "s|go.etcd.io/etcd/api/v3/|api/|g" "${cover_out_file}" || true
  sed --in-place -E "s|go.etcd.io/etcd/client/v3/|client/v3/|g" "${cover_out_file}" || true
  sed --in-place -E "s|go.etcd.io/etcd/client/pkg/v3|client/pkg/v3/|g" "${cover_out_file}" || true
  sed --in-place -E "s|go.etcd.io/etcd/etcdctl/v3/|etcdctl/|g" "${cover_out_file}" || true
  sed --in-place -E "s|go.etcd.io/etcd/etcdutl/v3/|etcdutl/|g" "${cover_out_file}" || true
  sed --in-place -E "s|go.etcd.io/etcd/pkg/v3/|pkg/|g" "${cover_out_file}" || true
  sed --in-place -E "s|go.etcd.io/etcd/server/v3/|server/|g" "${cover_out_file}" || true

  # held failures to generate the full coverage file, now fail
  if [ -n "$failed" ]; then
    for f in $failed; do
      log_error "--- FAIL:" "$f"
    done
    log_warning "Despite failures, you can see partial report:"
    log_warning "  go tool cover -html ${cover_out_file}"
    return 255
  fi

  log_success "done :) [see report: go tool cover -html ${cover_out_file}]"
}

######### Code formatting checkers #############################################

function shellcheck_pass {
  SHELLCHECK=shellcheck
  if ! tool_exists "shellcheck" "https://github.com/koalaman/shellcheck#installing"; then
    log_callout "Installing shellcheck $SHELLCHECK_VERSION"
    wget -qO- "https://github.com/koalaman/shellcheck/releases/download/${SHELLCHECK_VERSION}/shellcheck-${SHELLCHECK_VERSION}.linux.x86_64.tar.xz" | tar -xJv -C /tmp/ --strip-components=1
    mkdir -p ./bin
    mv /tmp/shellcheck ./bin/
    SHELLCHECK=./bin/shellcheck
  fi
  generic_checker run ${SHELLCHECK} -fgcc scripts/*.sh
}

function shellws_pass {
  log_callout "Ensuring no tab-based indention in shell scripts"
  local files
  if files=$(find . -name '*.sh' -print0 | xargs -0 grep -E -n $'^\s*\t'); then
    log_error "FAIL: found tab-based indention in the following bash scripts. Use '  ' (double space):"
    log_error "${files}"
    log_warning "Suggestion: run \"make fix\" to address the issue."
    return 255
  fi
  log_success "SUCCESS: no tabulators found."
}

function markdown_marker_pass {
  local marker="marker"
  # TODO: check other markdown files when marker handles headers with '[]'
  if ! tool_exists "$marker" "https://crates.io/crates/marker"; then
    log_callout "Installing markdown marker $MARKDOWN_MARKER_VERSION"
    wget -qO- "https://github.com/crawford/marker/releases/download/${MARKDOWN_MARKER_VERSION}/marker-${MARKDOWN_MARKER_VERSION}-x86_64-unknown-linux-musl.tar.gz" | tar -xzv -C /tmp/ --strip-components=1 >/dev/null
    mkdir -p ./bin
    mv /tmp/marker ./bin/
    marker=./bin/marker
  fi

  generic_checker run "${marker}" --skip-http --allow-absolute-paths --root "${ETCD_ROOT_DIR}" -e ./CHANGELOG -e ./etcdctl -e etcdutl -e ./tools 2>&1
}

function govuln_pass {
  run_for_modules run govulncheck -show verbose
}

function govet_pass {
  run_for_modules generic_checker run go vet
}

function govet_shadow_per_package {
  local shadow
  shadow=$1

  # skip grpc_gateway packages because
  #
  # stderr: etcdserverpb/gw/rpc.pb.gw.go:2100:3: declaration of "ctx" shadows declaration at line 2005
  local skip_pkgs=(
    "go.etcd.io/etcd/api/v3/etcdserverpb/gw"
    "go.etcd.io/etcd/server/v3/etcdserver/api/v3lock/v3lockpb/gw"
    "go.etcd.io/etcd/server/v3/etcdserver/api/v3election/v3electionpb/gw"
  )

  local pkgs=()
  while IFS= read -r line; do
    local in_skip_pkgs="false"

    for pkg in "${skip_pkgs[@]}"; do
      if [ "${pkg}" == "${line}" ]; then
        in_skip_pkgs="true"
        break
      fi
    done

    if [ "${in_skip_pkgs}" == "true" ]; then
      continue
    fi

    pkgs+=("${line}")
  done < <(go list ./...)

  run go vet -all -vettool="${shadow}" "${pkgs[@]}"
}

function govet_shadow_pass {
  local shadow
  shadow=$(tool_get_bin "golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow")

  run_for_modules generic_checker govet_shadow_per_package "${shadow}"
}

function lint_pass {
  run_for_modules generic_checker run golangci-lint run --config "${ETCD_ROOT_DIR}/tools/.golangci.yaml"
}

function lint_fix_pass {
  run_for_modules generic_checker run golangci-lint run --config "${ETCD_ROOT_DIR}/tools/.golangci.yaml" --fix
}

function license_header_per_module {
  # bash 3.x compatible replacement of: mapfile -t gofiles < <(go_srcs_in_module)
  local gofiles=()
  while IFS= read -r line; do gofiles+=("$line"); done < <(go_srcs_in_module)
  run_go_tool "github.com/google/addlicense" --check "${gofiles[@]}"
}

function license_header_pass {
  run_for_modules generic_checker license_header_per_module
}

# goword_for_package package
# checks spelling and comments in the 'package' in the current module
#
function goword_for_package {
  # bash 3.x compatible replacement of: mapfile -t gofiles < <(go_srcs_in_module)
  local gofiles=()
  while IFS= read -r line; do gofiles+=("$line"); done < <(go_srcs_in_module)
  
  local gowordRes

  # spellchecking can be enabled with GOBINARGS="--tags=spell"
  # but it requires heavy dependencies installation, like:
  # apt-get install libaspell-dev libhunspell-dev hunspell-en-us aspell-en

  # only check for broke exported godocs
  if gowordRes=$(run_go_tool "github.com/chzchzchz/goword" -use-spell=false "${gofiles[@]}" | grep godoc-export | sort); then
    log_error -e "goword checking failed:\\n${gowordRes}"
    return 255
  fi
  if [ -n "$gowordRes" ]; then
    log_error -e "goword checking returned output:\\n${gowordRes}"
    return 255
  fi
}


function goword_pass {
  run_for_modules goword_for_package || return 255
}

function go_fmt_for_package {
  # We utilize 'go fmt' to find all files suitable for formatting,
  # but reuse full power gofmt to perform just RO check.
  go fmt -n "$1" | sed 's| -w | -d |g' | sh
}

function gofmt_pass {
  run_for_modules generic_checker go_fmt_for_package
}

function bom_pass {
  log_callout "Checking bill of materials..."
  # https://github.com/golang/go/commit/7c388cc89c76bc7167287fb488afcaf5a4aa12bf
  # shellcheck disable=SC2207
  modules=($(modules_for_bom))

  # Internally license-bill-of-materials tends to modify go.sum
  run cp go.sum go.sum.tmp || return 2
  run cp go.mod go.mod.tmp || return 2

  output=$(GOFLAGS=-mod=mod run_go_tool github.com/appscodelabs/license-bill-of-materials \
    --override-file ./bill-of-materials.override.json \
    "${modules[@]}")
  code="$?"

  run cp go.sum.tmp go.sum || return 2
  run cp go.mod.tmp go.mod || return 2

  if [ "${code}" -ne 0 ] ; then
    log_error -e "license-bill-of-materials (code: ${code}) failed with:\\n${output}"
    return 255
  else
    echo "${output}" > "bom-now.json.tmp"
  fi
  if ! diff ./bill-of-materials.json bom-now.json.tmp; then
    log_error "modularized licenses do not match given bill of materials"
    return 255
  fi
  rm bom-now.json.tmp
}

######## VARIOUS CHECKERS ######################################################

function dump_deps_of_module() {
  local module
  if ! module=$(run go list -m); then
    return 255
  fi
  run go mod edit -json | jq -r '.Require[] | .Path+","+.Version+","+if .Indirect then " (indirect)" else "" end+",'"${module}"'"'
}

# Checks whether dependencies are consistent across modules
function dep_pass {
  local all_dependencies
  local tools_mod_dependencies
  all_dependencies=$(run_for_modules dump_deps_of_module | sort) || return 2
  # tools/mod is a special case. It is a module that is not included in the
  # module list from test_lib.sh. However, we need to ensure that the
  # dependency versions match the rest of the project. Therefore, explicitly
  # execute the command for tools/mod, and append its dependencies to the list.
  tools_mod_dependencies=$(run_for_module "tools/mod" dump_deps_of_module "./...") || return 2
  all_dependencies="${all_dependencies}"$'\n'"${tools_mod_dependencies}"

  local duplicates
  duplicates=$(echo "${all_dependencies}" | cut -d ',' -f 1,2 | sort | uniq | cut -d ',' -f 1 | sort | uniq -d) || return 2

  for dup in ${duplicates}; do
    log_error "FAIL: inconsistent versions for dependency: ${dup}"
    echo "${all_dependencies}" | grep "${dup}," | sed 's|\([^,]*\),\([^,]*\),\([^,]*\),\([^,]*\)|  - \1@\2\3 from: \4|g'
  done
  if [[ -n "${duplicates}" ]]; then
    log_error "FAIL: inconsistent dependencies"
    return 2
  else
    log_success "SUCCESS: dependencies are consistent across modules"
  fi
}

function release_pass {
  rm -f ./bin/etcd-last-release

  # Work out the previous release based on the version reported by etcd binary
  binary_version=$(./bin/etcd --version | grep --only-matching --perl-regexp '(?<=etcd Version: )\d+\.\d+')
  binary_major=$(echo "${binary_version}" | cut -d '.' -f 1)
  binary_minor=$(echo "${binary_version}" | cut -d '.' -f 2)
  previous_minor=$((binary_minor - 1))

  # Handle the edge case where we go to a new major version
  # When this happens we obtain latest minor release of previous major
  if [ "${binary_minor}" -eq 0 ]; then
    binary_major=$((binary_major - 1))
    previous_minor=$(git ls-remote --tags https://github.com/etcd-io/etcd.git \
    | grep --only-matching --perl-regexp "(?<=v)${binary_major}.\d.[\d]+?(?=[\^])" \
    | sort --numeric-sort --key 1.3 | tail -1 | cut -d '.' -f 2)
  fi
  
  # This gets a list of all remote tags for the release branch in regex
  # Sort key is used to sort numerically by patch version
  # Latest version is then stored for use below
  UPGRADE_VER=$(git ls-remote --tags https://github.com/etcd-io/etcd.git \
    | grep --only-matching --perl-regexp "(?<=v)${binary_major}.${previous_minor}.[\d]+?(?=[\^])" \
    | sort --numeric-sort --key 1.5 | tail -1 | sed 's/^/v/')
  log_callout "Found latest release: ${UPGRADE_VER}."

  if [ -n "${MANUAL_VER:-}" ]; then
    # in case, we need to test against different version
    UPGRADE_VER=$MANUAL_VER
  fi
  if [[ -z ${UPGRADE_VER} ]]; then
    UPGRADE_VER="v3.5.0"
    log_warning "fallback to" ${UPGRADE_VER}
  fi

  local file
  if [[ "$(uname -s)" == 'Darwin' ]]; then
    file="etcd-$UPGRADE_VER-darwin-$GOARCH.zip"
  else
    file="etcd-$UPGRADE_VER-linux-$GOARCH.tar.gz"
  fi

  log_callout "Downloading $file"

  set +e
  curl --fail -L "https://github.com/etcd-io/etcd/releases/download/$UPGRADE_VER/$file" -o "/tmp/$file"
  local result=$?
  set -e
  case $result in
    0)  ;;
    *)  log_error "--- FAIL:" ${result}
      return $result
      ;;
  esac

  tar xzvf "/tmp/$file" -C /tmp/ --strip-components=1 --no-same-owner
  mkdir -p ./bin
  mv /tmp/etcd ./bin/etcd-last-release
}

function release_tests_pass {
  if [ -z "${VERSION:-}" ]; then
    VERSION=$(go list -m go.etcd.io/etcd/api/v3 2>/dev/null | \
     awk '{split(substr($2,2), a, "."); print a[1]"."a[2]".99"}')
  fi

  if [ -n "${CI:-}" ]; then
    git config user.email "prow@etcd.io"
    git config user.name "Prow"

    gpg --batch --gen-key <<EOF
%no-protection
Key-Type: 1
Key-Length: 2048
Subkey-Type: 1
Subkey-Length: 2048
Name-Real: Prow
Name-Email: prow@etcd.io
Expire-Date: 0
EOF

    git remote add origin https://github.com/etcd-io/etcd.git
  fi

  DRY_RUN=true run "${ETCD_ROOT_DIR}/scripts/release.sh" --no-upload --no-docker-push --no-gh-release --in-place "${VERSION}"
  VERSION="${VERSION}" run "${ETCD_ROOT_DIR}/scripts/test_images.sh"
}

function mod_tidy_for_module {
  run go mod tidy -diff
}

function mod_tidy_pass {
  run_for_modules generic_checker mod_tidy_for_module
}

function proto_annotations_pass {
  "${ETCD_ROOT_DIR}/scripts/verify_proto_annotations.sh"
}

function genproto_pass {
  "${ETCD_ROOT_DIR}/scripts/verify_genproto.sh"
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
for pass in $PASSES; do
  if run_pass "${pass}" "$@"; then
    continue
  else
    fail_flag=true
  fi
done
if [ "$fail_flag" = true ]; then
  log_error "There was FAILURE in the test suites ran. Look above log detail"
  exit 255
fi

log_success "SUCCESS"
