#!/usr/bin/env bash

# http://redsymbol.net/articles/unofficial-bash-strict-mode/
set -euo pipefail
IFS=$'\n\t'

source ./scripts/test_lib.sh
source ./scripts/build_lib.sh

# Can't run darwin binaries in linux containers.
if [[ $(go env GOOS) == "darwin" ]]; then
  log_error "Error: Please use a Linux machine to test release images builds."
  exit 1
fi

# Can't proceed without Docker.
if ! command -v docker >/dev/null; then
  log_error "Error: Cannot find docker. Please follow the installation instructions at: https://docs.docker.com/get-docker/"
  exit 1
fi

# Start a container with the given Docker image.
function start_container {
  local container_name=$1
  local image=$2

    # run docker in the background
    docker run --detach --rm --name "${container_name}" "${image}"

    # wait for etcd daemon to bootstrap
    local attempts=0
    while ! docker exec "${container_name}" /usr/local/bin/etcdctl endpoint health --command-timeout=1s; do
      sleep 1
      attempts=$((attempts + 1))
      if [ "${attempts}" -gt 10 ]; then
        log_error "Error: etcd daemon failed to start."
        exit 1
      fi
    done
  }

# Run a version check for the given Docker image.
function run_version_check {
  local output
  local found_version
  local image=$1
  shift
  local expected_version=$1
  shift

  output=$(docker run --rm "${image}" "${@}")
  found_version=$(echo "${output}" | head -1 | rev | cut -d" " -f 1 | rev)
  if [[ "${found_version}" != "${expected_version}" ]]; then
    log_error "Error: Invalid Version."
    log_error "Got ${found_version}, expected ${expected_version}."
    log_error "Output: ${output}."
    exit 1
  fi
}

# Put a key-value pair in the etcd container and check if it can be retrieved,
# and has the expected value.
function put_get_check {
  local container_name=$1
  local key="foo"
  local value="bar"
  local result

  result=$(docker exec "${container_name}" /usr/local/bin/etcdctl put "${key}" "${value}")
  if [ "${result}" != "OK" ]; then
    log_error "Error: Storing key failed. Result: ${result}."
    exit 1
  fi

  result=$(docker exec "${container_name}" /usr/local/bin/etcdctl get "${key}" --print-value-only)
  if [ "${result}" != "${value}" ]; then
    log_error "Error: Problem with getting key. Got: ${result}, expected: ${value}."
    exit 1
  fi
}

# Verify that the images have the correct architecture.
function verify_images_architecture {
  local repository=$1
  local version=$2
  local target_arch
  local arch_tag
  local img_arch

  for target_arch in "amd64" "arm64" "ppc64le" "s390x"; do
    arch_tag="v${version}-${target_arch}"
    img_arch=$(docker inspect --format '{{.Architecture}}' "${repository}:${arch_tag}")
    if [ "${img_arch}" != "${target_arch}" ];then
      log_error "Error: Incorrect Docker image architecture. Got ${img_arch}, expected: ${arch_tag}."
      exit 1
    fi
    log_success "Correct architecture for ${arch_tag}."
  done
}

function main {
  local version="$1"
  local repository=${REPOSITORY:-"gcr.io/etcd-development/etcd"}
  local arch
  arch=$(go env GOARCH)
  local tag="v${version}-${arch}"
  local image="${TEST_IMAGE:-"${repository}:${tag}"}"
  local container_name="test_etcd"

  if [[ "$(docker images -q "${image}" 2> /dev/null)" == "" ]]; then
    log_error "Error: ${image} not present locally."
    exit 1
  fi

  log_callout "Running version check."
  run_version_check "${image}" "${version}" "/usr/local/bin/etcd" "--version"
  run_version_check "${image}" "${version}" "/usr/local/bin/etcdctl" "version"
  run_version_check "${image}" "${version}" "/usr/local/bin/etcdutl" "version"
  log_success "Successfully ran version check."

  log_callout "Running sanity check in Docker image."
  start_container "${container_name}" "${image}"
  # stop container
  trap 'docker stop '"${container_name}" EXIT
  put_get_check "${container_name}"
  log_success "Successfully tested etcd local image ${tag}."

  log_callout "Verifying images architecture."
  verify_images_architecture "${repository}" "${version}"
  log_success "Successfully tested images architecture."
}

if [ -z "$VERSION" ]; then
  log_error "Error: VERSION not supplied."
  exit 1
fi

main "${VERSION}"
