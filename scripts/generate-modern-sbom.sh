#!/usr/bin/env bash

set -euo pipefail

source ./scripts/test_lib.sh

VERSION=${1:-"dev"}
OUTPUT_DIR=${2:-"."}

function install_syft() {
    local syft_bin="./bin/syft"
    if ! command -v syft >/dev/null && [ ! -f "${syft_bin}" ]; then
        log_callout "installing syft to local bin directory..."
        mkdir -p ./bin
        curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b ./bin
        syft_bin="./bin/syft"
    elif [ -f "${syft_bin}" ]; then
        log_callout "syft is already installed locally: $(${syft_bin} version --output json | jq -r '.version')"
    else
        log_callout "syft is already installed: $(syft version --output json | jq -r '.version')"
        syft_bin="syft"
    fi
    
    # export the syft binary path for use in other functions
    export SYFT_BIN="${syft_bin}"
}

function install_sbomasm() {
    local sbomasm_bin="./bin/sbomasm"
    if ! command -v sbomasm >/dev/null && [ ! -f "${sbomasm_bin}" ]; then
        log_callout "installing sbomasm to local bin directory..."
        mkdir -p ./bin
        
        # determine architecture and OS for sbomasm download
        local os arch
        os=$(uname -s | tr '[:upper:]' '[:lower:]')
        arch=$(uname -m)
        
        case "${arch}" in
            x86_64) arch="amd64" ;;
            aarch64|arm64) arch="arm64" ;;
            *) log_error "unsupported architecture: ${arch}"; return 1 ;;
        esac
        
        # download latest sbomasm release
        local download_url="https://github.com/interlynk-io/sbomasm/releases/latest/download/sbomasm-${os}-${arch}"
        if curl -sSfL "${download_url}" -o "${sbomasm_bin}"; then
            chmod +x "${sbomasm_bin}"
            log_success "sbomasm installed successfully"
        else
            log_error "Failed to download sbomasm"
            return 1
        fi
    elif [ -f "${sbomasm_bin}" ]; then
        log_callout "sbomasm is already installed locally"
    else
        log_callout "sbomasm is already installed: $(sbomasm version 2>/dev/null || echo 'system')"
        sbomasm_bin="sbomasm"
    fi
    
    # export the sbomasm binary path for use in other functions
    export SBOMASM_BIN="${sbomasm_bin}"
}

function augment_sbom() {
    local version="$1"
    local output_dir="$2"
    
    log_callout "augmenting SBOM files with enhanced metadata..."
    
    local sbomasm_cmd="${SBOMASM_BIN:-sbomasm}"
    
    # augment SPDX SBOM with etcd-specific metadata
    log_callout "augmenting SPDX SBOM..."
    if "${sbomasm_cmd}" edit --append --subject Document \
        --author 'etcd maintainers (https://github.com/etcd-io/etcd/blob/main/MAINTAINERS)' \
        --supplier 'etcd project (https://etcd.io/)' \
        --repository 'https://github.com/etcd-io/etcd' \
        --license 'Apache-2.0 (https://raw.githubusercontent.com/etcd-io/etcd/main/LICENSE)' \
        "${output_dir}/sbom.spdx.json" > "${output_dir}/sbom.spdx.json.tmp"; then
        mv "${output_dir}/sbom.spdx.json.tmp" "${output_dir}/sbom.spdx.json"
        log_success "SPDX SBOM augmented successfully"
    else
        log_error "failed to augment SPDX SBOM"
        rm -f "${output_dir}/sbom.spdx.json.tmp"
        return 1
    fi
    
    # augment CycloneDX SBOM with etcd-specific metadata
    log_callout "augmenting CycloneDX SBOM..."
    if "${sbomasm_cmd}" edit --append --subject Document \
        --supplier 'etcd project (https://etcd.io/)' \
        --repository 'https://github.com/etcd-io/etcd' \
        --license 'Apache-2.0 (https://raw.githubusercontent.com/etcd-io/etcd/main/LICENSE)' \
        "${output_dir}/sbom.cdx.json" > "${output_dir}/sbom.cdx.json.tmp" 2>/dev/null; then
        mv "${output_dir}/sbom.cdx.json.tmp" "${output_dir}/sbom.cdx.json"
        log_success "CycloneDX SBOM augmented successfully"
    else
        log_warning "failed to augment CycloneDX SBOM with sbomasm, using original file"
        rm -f "${output_dir}/sbom.cdx.json.tmp"
        # FIXME: should we fail if CDX augmentation fails or proceed with original file?
    fi
    
    log_success "SBOM augmentation completed successfully"
}

function generate_modern_sbom() {
    local version="$1"
    local output_dir="$2"
    
    log_callout "generating modern SBOM files for version ${version}"
    
    # ensure output directory exists
    mkdir -p "${output_dir}"
    
    # use the syft binary determined in install_syft
    local syft_cmd="${SYFT_BIN:-syft}"
    
    # generate SPDX format with source metadata
    log_callout "generating SPDX SBOM..."
    if GOOS=linux "${syft_cmd}" scan dir:. --source-name "etcd" --source-version "${version}" -o spdx-json="${output_dir}/sbom.spdx.json"; then
        log_success "SPDX SBOM generated: ${output_dir}/sbom.spdx.json"
    else
        log_error "failed to generate SPDX SBOM"
        return 1
    fi
    
    # generate CycloneDX format
    log_callout "generating CycloneDX SBOM..."
    if GOOS=linux "${syft_cmd}" scan dir:. --source-name "etcd" --source-version "${version}" -o cyclonedx-json="${output_dir}/sbom.cdx.json"; then
        log_success "CycloneDX SBOM generated: ${output_dir}/sbom.cdx.json"
    else
        log_error "failed to generate CycloneDX SBOM"
        return 1
    fi
    
    # augment SBOMs with enhanced metadata
    augment_sbom "${version}" "${output_dir}"
    
    log_success "modern SBOM files generated successfully"
    log_callout "files created:"
    log_callout "  - ${output_dir}/sbom.spdx.json"
    log_callout "  - ${output_dir}/sbom.cdx.json"
}

function validate_sbom_files() {
    local output_dir="$1"
    
    log_callout "validating generated SBOM files..."
    
    # check if files exist and are not empty
    for file in "${output_dir}/sbom.spdx.json" "${output_dir}/sbom.cdx.json"; do
        if [[ ! -f "$file" ]]; then
            log_error "SBOM file not found: $file"
            return 1
        fi
        
        if [[ ! -s "$file" ]]; then
            log_error "SBOM file is empty: $file"
            return 1
        fi
        
        # basic JSON validation
        if ! jq empty "$file" 2>/dev/null; then
            log_error "SBOM file contains invalid JSON: $file"
            return 1
        fi
        
        log_success "Validated: $file"
    done
    
    # check for expected content in SPDX
    if ! jq -e '.spdxVersion' "${output_dir}/sbom.spdx.json" >/dev/null; then
        log_error "SPDX SBOM missing spdxVersion field"
        return 1
    fi
    
    # check for expected content in CycloneDX
    if ! jq -e '.bomFormat' "${output_dir}/sbom.cdx.json" >/dev/null; then
        log_error "CycloneDX SBOM missing bomFormat field"
        return 1
    fi
    
    log_success "all SBOM files validated successfully"
}

function main() {
    install_syft
    install_sbomasm
    generate_modern_sbom "${VERSION}" "${OUTPUT_DIR}"
    validate_sbom_files "${OUTPUT_DIR}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
