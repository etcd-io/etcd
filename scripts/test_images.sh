#!/usr/bin/env bash

# http://redsymbol.net/articles/unofficial-bash-strict-mode/
set -euo pipefail
IFS=$'\n\t'

source ./scripts/test_lib.sh

function startContainer {
    # run docker in the background
    docker run -d --rm --name "${RUN_NAME}" "${IMAGE}"

    # wait for etcd daemon to bootstrap
    sleep 5
}

function runVersionCheck {
    Out=$(docker run --rm "${IMAGE}" "${@}")
    foundVersion=$(echo "$Out" | head -1 | rev  | cut -d" "  -f 1 | rev )
    if [[ "${foundVersion}" != "${VERSION}" ]]; then
        echo "error: Invalid Version. Got $foundVersion, expected $VERSION. Error: $Out"
        exit 1
    fi
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

# Pick defaults based on release workflow
ARCH=$(go env GOARCH)
REPOSITARY=${REPOSITARY:-"gcr.io/etcd-development/etcd"}
if [ -n "$VERSION" ]; then
    # Expected Format: v3.6.99-amd64
    TAG=v"${VERSION}"-"${ARCH}"
else
    echo "Terminating test, VERSION not supplied"
    exit 1
fi
IMAGE=${IMAGE:-"${REPOSITARY}:${TAG}"}

# ETCD related values
RUN_NAME="test_etcd"
KEY="foo"
VALUE="bar"

if [[ "$(docker images -q "${IMAGE}" 2> /dev/null)" == "" ]]; then
    echo "${IMAGE} not present locally"
    exit 1
fi

# Version check
runVersionCheck "/usr/local/bin/etcd" "--version"
runVersionCheck "/usr/local/bin/etcdctl" "version"
runVersionCheck "/usr/local/bin/etcdutl" "version"

startContainer
# stop container
trap 'docker stop "${RUN_NAME}"' EXIT


# Put/Get check
PUT=$(docker exec "${RUN_NAME}" /usr/local/bin/etcdctl put "${KEY}" "${VALUE}")
if [ "${PUT}" != "OK" ]; then
    echo "Problem with Putting in etcd"
    exit 1
fi

GET=$(docker exec "${RUN_NAME}" /usr/local/bin/etcdctl get "$KEY" --print-value-only)
if [ "${GET}" != "${VALUE}" ]; then
    echo "Problem with getting foo bar in etcd. Got ${GET}"
    exit 1
fi

echo "Succesfully tested etcd local image ${TAG}"

