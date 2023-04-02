#!/usr/bin/env bash

# Source of truth, these should not be modified elsewhere
readonly IMAGE_ARCH_LIST="amd64 arm64 ppc64le s390x"
readonly CONTAINER_REGISTRY_LIST="quay.io/coreos gcr.io/etcd-development"

function compareSemanticVersions() {
  
    v1=$(echo "$1" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+') 
    v2=$(echo "$2" | grep -oE '^[0-9]+\.[0-9]+\.[0-9]+')

    if [ -z "$v1" ] || [ -z "$v2" ]; then
        log_error "ERROR: Invalid version string(s) provided ${v1} and ${v2}."
        exit 1
    fi

    read -ra ver1 <<< "${v1//./ }"
    read -ra ver2 <<< "${v2//./ }"

    len=${#ver1[@]}
    if ((len < ${#ver2[@]})); then
        len=${#ver2[@]}
    fi

    # compare each component of the versions
    for ((i=0; i<len; i++)); do

        # set missing components to 0
        num1=${ver1[i]:-0}
        num2=${ver2[i]:-0}

        # compare the components
        if ((num1 < num2)); then
            log_info "INFO: ${v2} is greater than ${v1}."
            return 0
        elif ((num1 > num2)); then
            log_info "INFO: ${v1} is greater than ${v2}."
            return 0
        fi
    done

    # if we reach this point, the versions are equal
    log_info "INFO: ${v1} and ${v2} are equal."
    return 0
}

function checkContainerEngine() {

    # Validate args
    if [ "${#}" -ne 1 ]; then
        log_error "ERROR: checkContainerEngine requires exactly one parameter."
        exit 1
    fi

    ENGINE="${1}"

    # Etcd releases require a container engine like docker
    if ! command -v "${ENGINE}" >/dev/null; then
        log_error "ERROR: Cannot find container engine ${ENGINE}."
        exit 1
    fi

    # Etcd releases require a modern version of docker
    # TODO Consider adding support for a recent version of podman
    MINIMUM_DOCKER_VERSION="20.10.0"
    CURRENT_VERSION=$("${ENGINE}" version --format '{{.Server.Version}}' | cut -d '+' -f1)

    if compareSemanticVersions "${MINIMUM_DOCKER_VERSION}" "${CURRENT_VERSION}"; then
        log_success "SUCCESS: ${ENGINE} version ${CURRENT_VERSION} is >= minimum version ${MINIMUM_DOCKER_VERSION}."
    else
        log_error "ERROR: ${ENGINE} version ${CURRENT_VERSION} is less than the minimum version ${MINIMUM_DOCKER_VERSION}."
        exit 1
    fi

    # Etcd releases require a linux host
    if [[ $(go env GOOS) == "darwin" ]]; then
        log_error "ERROR: Please use linux machine for release builds."
        exit 1
    fi
}

function checkImageExists() {

    # Validate args
    if [ "${#}" -ne 1 ]; then
        log_error "ERROR: checkImageExists requires exactly one parameter."
        exit 1
    fi

    IMAGE="${1}"
    log_info "INFO: Checking image ${IMAGE} is present locally."
    if [[ "$(docker images --quiet "${IMAGE}" 2> /dev/null)" == "" ]]; then
        log_error "ERROR: Image ${IMAGE} not present locally."
        exit 1
    fi

    log_success "SUCCESS: Image ${IMAGE} exists locally."
}


function checkImageHasArch() {

    # Validate args
    if [ "${#}" -ne 2 ]; then
        log_error "ERROR: checkImageHasArch requires exactly two parameters."
        exit 1
    fi

    IMAGE="${1}"
    EXPECTED_ARCH="${2}"

    # Validate the image exists
    checkImageExists "${IMAGE}"

    # Validate that manifest exists
    MANIFEST=$(docker manifest inspect "${IMAGE}")
    if [ -z "${MANIFEST}" ]; then
        log_error "ERROR: Manifest not found for image ${IMAGE}."
        exit 1
    fi

    # Check that expected architecture is included
    PLATFORMS=$(echo "${MANIFEST}" | sed -n 's/.*"platform":{\(.*\)},/\1/p' | tr -d ' ')
    if [[ "${PLATFORMS}" != *"${EXPECTED_ARCH}"* ]]; then
        log_error "ERROR: Image ${IMAGE} does not contain the expected architecture ${EXPECTED_ARCH}."
        exit 1
    fi

    log_success "SUCCESS: Image ${IMAGE} contains the expected architecture ${EXPECTED_ARCH}."
    return 0
}

function buildMultiArchImage() {

    # Validate args
    if [ "${#}" -ne 1 ]; then
        log_error "ERROR: buildMultiArchImage requires exactly one parameter."
        exit 1
    fi

    RELEASE_VERSION="${1}"

    log_info "INFO: Release version: ${RELEASE_VERSION}"
    log_info "INFO: Image arch list: ${IMAGE_ARCH_LIST}"
    log_info "INFO: Container registries: ${CONTAINER_REGISTRY_LIST}"

    IFS=' ' read -ra image_arch_array <<< "${IMAGE_ARCH_LIST}"
    IFS=' ' read -ra registries_array <<< "${CONTAINER_REGISTRY_LIST}"

    for TARGET_ARCH in "${image_arch_array[@]}"; do
        for REGISTRY in "${registries_array[@]}"; do

            # Update the manifest
            log_info "INFO: Updating manifest ${REGISTRY}/etcd:${RELEASE_VERSION} with ${REGISTRY}/etcd:${RELEASE_VERSION}-${TARGET_ARCH}."
            docker manifest create --amend "${REGISTRY}/etcd:${RELEASE_VERSION}" "${REGISTRY}/etcd:${RELEASE_VERSION}-${TARGET_ARCH}"
            docker manifest annotate "${REGISTRY}/etcd:${RELEASE_VERSION}" "${REGISTRY}/etcd:${RELEASE_VERSION}-${TARGET_ARCH}" --arch "${TARGET_ARCH}"
        done
    done
}

function startContainer {

    # Validate args
    if [ "${#}" -ne 2 ]; then
        log_error "ERROR: startContainer requires exactly two parameters."
        exit 1
    fi

    CONTAINER_NAME="${1}"
    IMAGE="${2}"

    # Run docker in the background
    log_info "INFO: Starting container named: ${CONTAINER_NAME} with image: ${IMAGE}."
    docker run --detach --rm --name "${CONTAINER_NAME}" "${IMAGE}"

    # Wait for etcd daemon to bootstrap
    sleep 5

    # Cleanup in case of failures
    trap 'docker stop "${CONTAINER_NAME}"' EXIT
}
