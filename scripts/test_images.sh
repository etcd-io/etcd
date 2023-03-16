#!/usr/bin/env bash

# Exit on error
set -e

# Consider command as failed when any component of the pipe fails:
# https://stackoverflow.com/questions/1221833/pipe-output-and-capture-exit-status-in-bash
set -o pipefail

source ./scripts/test_lib.sh
source ./scripts/build_lib.sh

function startContainer {
    # run docker in the background
    docker run -d --rm --name "${RUN_NAME}" "${TAG}" &

    # wait for etcd daemon to bootstrap
    sleep 5
}

function stopContainer {
    docker stop "${RUN_NAME}"
    docker image rm "${TAG}"
}

# Can't proceed without docker
if ! command -v docker >/dev/null; then
    log_error "cannot find docker"
    exit 1
fi

# You can't run darwin binaries in linux containers
if [[ $(go env GOOS) == "darwin" ]]; then
    echo "Please use linux machine for release builds."
    exit 1
fi

# Docker build
ARCH=$(go env GOARCH)
DOCKERFILE="Dockerfile-release.${ARCH}"
BINARYDIR=${BINARYDIR:-"bin"}
# Pick defaults based on release workflow
IMAGE=${IMAGE:-"gcr.io/etcd-development/etcd"}
VERSION=${VERSION:-"3.6.99"}
TAG="${IMAGE}:${VERSION}"

# ETCD related values
RUN_NAME="test_etcd"
KEY="foo"
VALUE="bar"

# Build if binaries are not present
if [ -z "$BINARYDIR" ]; then
    run ./scripts/build.sh
fi

# Build only if image is not present
if [[ "$(docker images -q "${TAG}" 2> /dev/null)" == "" ]]; then
    echo "${TAG} not present locally, building it..."
    # Build a local image from bin directory
    if ! docker build -t "${TAG}" -f "${DOCKERFILE}" "${BINARYDIR}"; then
        echo "Docker build unsuccessful. Exit code $?"
        exit 1
    fi
fi

startContainer

# Do the checks
PUT=$(docker exec "${RUN_NAME}" /usr/local/bin/etcdctl put "${KEY}" "${VALUE}")
if [ "${PUT}" != "OK" ]; then
    echo "Problem with Putting in etcd"
    stopContainer
    exit 1
fi

GET=$(docker exec "${RUN_NAME}" /usr/local/bin/etcdctl get "$KEY" --print-value-only)
if [ "${GET}" != "${VALUE}" ]; then
    echo "Problem with getting foo bar in etcd. Got ${GET}"
    stopContainer
    exit 1
fi

stopContainer

echo "Succesfully tested etcd local image ${TAG}"

