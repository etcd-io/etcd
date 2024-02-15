#!/usr/bin/env bash

set -euo pipefail

source ./scripts/test_lib.sh

GIT_SHA=$(git rev-parse --short HEAD || echo "GitNotFound")
VERSION_SYMBOL="${ROOT_MODULE}/api/v3/version.GitSHA"

# use go env if noset
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}

GO_BUILD_FLAGS=${GO_BUILD_FLAGS:-}

CGO_ENABLED="${CGO_ENABLED:-0}"

# Set GO_LDFLAGS="-s" for building without symbols for debugging.
# shellcheck disable=SC2206
GO_LDFLAGS=(${GO_LDFLAGS:-} "-X=${VERSION_SYMBOL}=${GIT_SHA}")
GO_BUILD_ENV=("CGO_ENABLED=${CGO_ENABLED}" "GO_BUILD_FLAGS=${GO_BUILD_FLAGS}" "GOOS=${GOOS}" "GOARCH=${GOARCH}")

etcd_build() {
  out="bin"
  if [[ -n "${BINDIR:-}" ]]; then out="${BINDIR}"; fi

  run rm -f "${out}/etcd"
  (
    cd ./server
    # Static compilation is useful when etcd is run in a container. $GO_BUILD_FLAGS is OK
    # shellcheck disable=SC2086
    run env "${GO_BUILD_ENV[@]}" go build $GO_BUILD_FLAGS \
      -trimpath \
      -installsuffix=cgo \
      "-ldflags=${GO_LDFLAGS[*]}" \
      -o="../${out}/etcd" . || return 2
  ) || return 2

  run rm -f "${out}/etcdutl"
  # shellcheck disable=SC2086
  (
    cd ./etcdutl
    run env GO_BUILD_FLAGS="${GO_BUILD_FLAGS}" "${GO_BUILD_ENV[@]}" go build $GO_BUILD_FLAGS \
      -trimpath \
      -installsuffix=cgo \
      "-ldflags=${GO_LDFLAGS[*]}" \
      -o="../${out}/etcdutl" . || return 2
  ) || return 2

  run rm -f "${out}/etcdctl"
  # shellcheck disable=SC2086
  (
    cd ./etcdctl
    run env GO_BUILD_FLAGS="${GO_BUILD_FLAGS}" "${GO_BUILD_ENV[@]}" go build $GO_BUILD_FLAGS \
      -trimpath \
      -installsuffix=cgo \
      "-ldflags=${GO_LDFLAGS[*]}" \
      -o="../${out}/etcdctl" . || return 2
  ) || return 2
  # Verify whether symbol we overwrote exists
  # For cross-compiling we cannot run: ${out}/etcd --version | grep -q "Git SHA: ${GIT_SHA}"

  # We need symbols to do this check:
  if [[ "${GO_LDFLAGS[*]}" != *"-s"* ]]; then
    go tool nm "${out}/etcd" | grep "${VERSION_SYMBOL}" > /dev/null
    if [[ "${PIPESTATUS[*]}" != "0 0" ]]; then
      log_error "FAIL: Symbol ${VERSION_SYMBOL} not found in binary: ${out}/etcd"
      return 2
    fi
  fi
}

tools_build() {
  out="bin"
  if [[ -n "${BINDIR:-}" ]]; then out="${BINDIR}"; fi
  tools_path="tools/benchmark
    tools/etcd-dump-db
    tools/etcd-dump-logs
    tools/local-tester/bridge"
  for tool in ${tools_path}
  do
    echo "Building" "'${tool}'"...
    run rm -f "${out}/${tool}"
    # shellcheck disable=SC2086
    run env GO_BUILD_FLAGS="${GO_BUILD_FLAGS}" CGO_ENABLED=${CGO_ENABLED} go build ${GO_BUILD_FLAGS} \
      -trimpath \
      -installsuffix=cgo \
      "-ldflags=${GO_LDFLAGS[*]}" \
      -o="${out}/${tool}" "./${tool}" || return 2
  done
}

run_build() {
  echo Running "$1"
  if $1; then
    log_success "SUCCESS: $1 (GOARCH=${GOARCH})"
  else
    log_error "FAIL: $1 (GOARCH=${GOARCH})"
    exit 2
  fi
}
