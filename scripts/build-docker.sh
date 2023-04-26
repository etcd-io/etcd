#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 VERSION" >&2
  exit 1
fi

VERSION=${1}
if [ -z "$VERSION" ]; then
  echo "Usage: ${0} VERSION" >&2
  exit 1
fi

ARCH=$(go env GOARCH)
VERSION="${VERSION}-${ARCH}"
DOCKERFILE="Dockerfile"

if [ -z "${BINARYDIR:-}" ]; then
  RELEASE="etcd-${1}"-$(go env GOOS)-${ARCH}
  BINARYDIR="${RELEASE}"
  TARFILE="${RELEASE}.tar.gz"
  TARURL="https://github.com/etcd-io/etcd/releases/download/${1}/${TARFILE}"
  if ! curl -f -L -o "${TARFILE}" "${TARURL}" ; then
    echo "Failed to download ${TARURL}."
    exit 1
  fi
  tar -zvxf "${TARFILE}"
fi

BINARYDIR=${BINARYDIR:-.}
BUILDDIR=${BUILDDIR:-.}

IMAGEDIR=${BUILDDIR}/image-docker

mkdir -p "${IMAGEDIR}"/var/etcd
mkdir -p "${IMAGEDIR}"/var/lib/etcd
cp "${BINARYDIR}"/etcd "${BINARYDIR}"/etcdctl "${BINARYDIR}"/etcdutl "${IMAGEDIR}"

cat ./"${DOCKERFILE}" > "${IMAGEDIR}"/Dockerfile

if [ -z "${TAG:-}" ]; then
    # Fix incorrect image "Architecture" using buildkit
    # From https://stackoverflow.com/q/72144329/
    DOCKER_BUILDKIT=1 docker build --build-arg="ARCH=${ARCH}" -t "gcr.io/etcd-development/etcd:${VERSION}" "${IMAGEDIR}"
    DOCKER_BUILDKIT=1 docker build --build-arg="ARCH=${ARCH}" -t "quay.io/coreos/etcd:${VERSION}" "${IMAGEDIR}"
else
    docker build -t "${TAG}:${VERSION}" "${IMAGEDIR}"
fi
