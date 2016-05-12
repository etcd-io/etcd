#!/usr/bin/env bash
set -e

REGISTRY_NAME=$1
if [ -z "${REGISTRY_NAME}" ]; then
	echo "Usage: ${0} REGISTRY_NAME VERSION" >> /dev/stderr
	exit 255
fi

VERSION=$2
if [ -z "${VERSION}" ]; then
	echo "Usage: ${0} REGISTRY_NAME VERSION" >> /dev/stderr
	exit 255
fi

if ! command -v docker >/dev/null; then
    echo "cannot find docker"
    exit 1
fi

# TODO:
# build binaries, ACI images, and serve in Google Cloud Storage

source ./build
sudo docker build -t $REGISTRY_NAME:${VERSION} .
