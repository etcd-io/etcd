#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

#Input filter
if [ $# -ne 4 ]; then
  echo "Usage $0 [build|push] image_name directory go_version"
  exit 1
fi

IMAGE=$2
DIR=$3
GOVERSION=$4

ETCD_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
declare -A QEMUARCHS=( ["amd64"]="x86_64" ["arm64"]="aarch64" )
QEMUVERSION="v4.1.0-1"
REGISTRY="gcr.io/etcd-development"

# Returns list of all supported architectures from BASEIMAGE file
listArchs() {
  cut -d "=" -f 1 "${DIR}"/BASEIMAGE
}

# Returns baseimage need to used in Dockerfile for any given architecture
getBaseImage() {
  arch=$1
  grep "${arch}=" BASEIMAGE | cut -d= -f2
}

# Download qemu binary for architecture
getQemuStatic() {
  local arch
  local qemuarch
  arch=$1
  qemuarch=${QEMUARCHS[${arch}]}

  echo "Downloading qemu-${qemuarch}-static"
  if ! curl -OLs https://github.com/multiarch/qemu-user-static/releases/download/"${QEMUVERSION}"/x86_64_qemu-"${qemuarch}"-static.tar.gz; then
    echo "Error downloading qemu-${qemuarch}-static"
  fi
  tar -xvf x86_64_qemu-"${qemuarch}"-static.tar.gz
  rm -f x86_64_qemu-*
}

# Download manifest tool
getManifest() {
  local version
  version=$(curl -s https://api.github.com/repos/estesp/manifest-tool/tags | jq -r '.[0].name')
  echo "Downloading manifest-tool"
  if ! curl -OLs "https://github.com/estesp/manifest-tool/releases/download/$version/manifest-tool-linux-amd64"; then
    echo "Error downloading manifest-tool"
    exit
  fi
  mv manifest-tool-linux-amd64 manifest-tool
  chmod +x manifest-tool
}

# Base on the architecture, tailer Dockerfile
makeDockerfile() {
  local arch
  local dockerfile

  arch=$1
  dockerfile="Dockerfile-${arch}"
  /bin/cp -f Dockerfile "$dockerfile"

  local baseimage
  local qemuarch
  baseimage=$(getBaseImage "${arch}")
  qemuarch=${QEMUARCHS[${arch}]}

  echo "${qemuarch}"

  if [[ "${arch}" == amd64 ]]; then
      sed -i "/CROSS_BUILD_/d" "${dockerfile}"
      sed -i "s|AMDONLY||g" "${dockerfile}"
  else
      sed -i "s/CROSS_BUILD_//g" "${dockerfile}"
      sed -i "/AMDONLY /d" "${dockerfile}"
      sed -i "s|QEMUARCH|${qemuarch}|g" "${dockerfile}"
  fi

  sed -i "s|BASEIMAGE|${baseimage}|g" "${dockerfile}"
  sed -i "s|ARCH|${arch}|g" "${dockerfile}"
  sed -i "s|REPLACE_ME_GO_VERSION|${GOVERSION}|g" "${dockerfile}"
}

# Build docker image for multi-architecture
build() {
  local registry
  local tag
  local platforms
  registry=$1
  tag="go$2"
  platforms=()

  if [[ -f ${DIR}/BASEIMAGE ]]; then
    archs=$(listArchs)
  else
    archs=${!QEMUARCHS[*]}
  fi

    mkdir -p "${ETCD_ROOT}"/_tmp
    tmp_dir=$(mktemp -d "${ETCD_ROOT}"/_tmp/test-images-build.XXXXXX)
    cp -r "${DIR}"/* "${tmp_dir}"
    pushd "${tmp_dir}"
    pwd
  for arch in ${archs}; do
    echo "Build image for ${IMAGE} ARCH: ${arch}..."
    if [[ "${arch}" != amd64  ]]; then
      getQemuStatic "${arch}"
    fi
    makeDockerfile "${arch}"
    docker build --file "Dockerfile-$arch" \
	         --tag "${registry}/$IMAGE:${tag}-${arch}" .
  done
}

# Push multi-arch docker image
push() {
  local registry
  local tag
  local platforms
  registry=$1
  tag="go$2"
  platforms=()


  if [[ -f ${DIR}/BASEIMAGE ]]; then
    archs=$(listArchs)
  else
    archs=${!QEMUARCHS[*]}
  fi

  for arch in ${archs}; do
    echo "Push ${registry}/$IMAGE:${tag}-${arch}"
    docker push "${registry}/$IMAGE:${tag}-${arch}"
    platforms+=("linux/${arch}")
  done
  # We use docker manifest when docker version is larger than 18.06.0
  # Otherwise, we use manifest-tool
  docker_version=$(docker version --format '{{.Client.Version}}' | cut -d"-" -f1)
  if [[ ${docker_version} != 18.06.0 && ${docker_version} < 18.06.0 ]]; then
    getManifest
    platformslist=$( IFS=','; echo "${platforms[*]}" )
    ./manifest-tool push from-args \
	        --platforms "${platformslist}" \
	        --template "${registry}/$IMAGE:${tag}-ARCH" \
	        --target "$registry/$IMAGE:${tag}"
    rm ./manifest-tool
  else
    DOCKER_CLI_EXPERIMENTAL="enabled"
    export DOCKER_CLI_EXPERIMENTAL
    while IFS='' read -r line; do manifest+=("${registry}/${IMAGE}:${tag}-$line"); done < <(echo "$archs")
    docker manifest create --amend "${registry}/${IMAGE}:${tag}" "${manifest[@]}"
    for arch in ${archs}; do
      docker manifest annotate --arch "${arch}" "${registry}/${IMAGE}:${tag}" "${registry}/${IMAGE}:${tag}-${arch}"
    done
    docker manifest push --purge "${registry}/${IMAGE}:${tag}"
  fi
}

cleanup() {
  local testfolder
  testfolder=$(pwd)
  echo "clean up ${testfolder}"
  cd "${ETCD_ROOT}"
  rm -rf "${testfolder}"
}

# Main
case "$1" in
  build)
          # Register binfmt_misc to run cross platform builds against non x86 architectures
          docker run --rm --privileged multiarch/qemu-user-static:register --reset
	  echo -n "Starting build image: "
	  build "${REGISTRY}" "${GOVERSION}"
	  cleanup
	  ;;
  push)
	  push "${REGISTRY}" "${GOVERSION}"
	  ;;
  *)
	  echo "Usage $0 [build|push] image_name directory go_version"
	  echo "e.g. ./scripts/image-builder.sh build|push etcd-test tests 1.13.1"
	  ;;
esac

exit 0
