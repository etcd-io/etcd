#!/usr/bin/env bash

# http://redsymbol.net/articles/unofficial-bash-strict-mode/
set -euo pipefail
IFS=$'\n\t'

source ./scripts/test_lib.sh
source ./scripts/build_lib.sh
source ./scripts/docker_lib.sh

function runVersionCheck() {
    out=$(docker exec "${1}" "${@:2}")
    foundVersion=$(echo "${out}" | head -1 | rev  | cut -d" "  -f 1 | rev )
    if [[ "${foundVersion}" != "${VERSION}" ]]; then
        log_error "ERROR: Invalid version. Got ${foundVersion}, expected ${VERSION}. Error: ${out}"
        exit 1
    fi
}

# Tests can't proceed without docker
checkContainerEngine "docker"

# Pick defaults based on release workflow
ARCH=$(go env GOARCH)
REPOSITORY=${REPOSITORY:-"gcr.io/etcd-development/etcd"}
if [ -n "${VERSION}" ]; then
    # Expected Format: v3.6.99-amd64
    TAG=v"${VERSION}"-"${ARCH}"
else
    log_error "ERROR: Terminating test, VERSION not supplied"
    exit 1
fi
IMAGE=${IMAGE:-"${REPOSITORY}:${TAG}"}

# Etcd related values
CONTAINER_NAME="test_etcd"
KEY="foo"
VALUE="bar"

# Start an etcd container
startContainer "${CONTAINER_NAME}" "${IMAGE}"

# Test versions in container
runVersionCheck "${CONTAINER_NAME}" "etcd" "--version"
runVersionCheck "${CONTAINER_NAME}" "etcdctl" "version"
runVersionCheck "${CONTAINER_NAME}" "etcdutl" "version"

# Test etcd kv working
PUT=$(docker exec "${CONTAINER_NAME}" etcdctl put "${KEY}" "${VALUE}")
if [ "${PUT}" != "OK" ]; then
    log_error "ERROR: Etcd PUT test failed in container ${CONTAINER_NAME}. Got ${PUT}"
    exit 1
fi

GET=$(docker exec "${CONTAINER_NAME}" etcdctl get "${KEY}" --print-value-only)
if [ "${GET}" != "${VALUE}" ]; then
    log_error "ERROR: Etcd GET test failed in container ${CONTAINER_NAME}. Got ${GET}"
    exit 1
fi

# Test we can assemble the multi arch etcd image
buildMultiArchImage "v${VERSION}"

# Test local image manifest contains all expected architectures
if [ -n "${IMAGE_ARCH_LIST}" ]; then
    for TARGET_ARCH in ${IMAGE_ARCH_LIST}; do
        checkImageHasArch "${REPOSITORY}:${VERSION}" "${TARGET_ARCH}"
    done
else
    log_error "ERROR: IMAGE_ARCH_LIST not populated."
    exit 1
fi

log_success "SUCCESS: Etcd local image tests for ${IMAGE} passed."
