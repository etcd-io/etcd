#!/usr/bin/env bash

# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

source ./scripts/test_lib.sh

SYFT_VERSION="v1.44.0"
VERSION=${1:-"dev"}
OUTPUT_DIR=${2:-"."}

function generate_modern_sbom() {
  local version="$1"
  local output_dir="$2"

  mkdir -p "${output_dir}"
  log_callout "generating modern SBOM files for version ${version}"

  log_callout "ensuring syft ${SYFT_VERSION} is installed..."
  GOARCH="" GOOS="" run go install "github.com/anchore/syft/cmd/syft@${SYFT_VERSION}" || return 2
  local syft_bin
  syft_bin="$(go env GOPATH)/bin/syft"

  log_callout "generating SPDX and CycloneDX SBOMs..."
  GOOS=linux run "${syft_bin}" \
    scan dir:. \
    --source-name "etcd" \
    --source-version "${version}" \
    -o "spdx-json=${output_dir}/sbom.spdx.json" \
    -o "cyclonedx-json=${output_dir}/sbom.cdx.json"

  augment_sbom "${version}" "${output_dir}"
  validate_sbom_files "${output_dir}"

  log_success "modern SBOM files generated successfully"
  log_callout "  - ${output_dir}/sbom.spdx.json"
  log_callout "  - ${output_dir}/sbom.cdx.json"
}

function augment_sbom() {
  local version="$1"
  local output_dir="$2"

  log_callout "augmenting SBOMs with project metadata..."

  jq --arg ver "${version}" '
    .packages = [.packages[] |
      if (.name | startswith("go.etcd.io/etcd")) and .versionInfo == "UNKNOWN"
      then .versionInfo = $ver
      else . end
    ] |
    .name = "etcd-\($ver)" |
    .documentNamespace = "https://github.com/etcd-io/etcd/releases/tag/\($ver)/sbom.spdx.json" |
    .creationInfo.creators = [
      "Organization: etcd project (https://etcd.io/)",
      "Tool: syft"
    ]
  ' "${output_dir}/sbom.spdx.json" > "${output_dir}/sbom.spdx.json.tmp"
  mv "${output_dir}/sbom.spdx.json.tmp" "${output_dir}/sbom.spdx.json"

  jq --arg ver "${version}" '
    .components = [.components[] |
      if (.name | startswith("go.etcd.io/etcd")) and .version == "UNKNOWN"
      then .version = $ver
      else . end
    ] |
    .metadata.supplier = {
      "name": "etcd project",
      "url": ["https://etcd.io/"]
    } |
    .metadata.component.licenses = [
      {"license": {"id": "Apache-2.0"}}
    ] |
    .metadata.component.author = "etcd maintainers (https://github.com/etcd-io/etcd/blob/main/OWNERS)"
  ' "${output_dir}/sbom.cdx.json" > "${output_dir}/sbom.cdx.json.tmp"
  mv "${output_dir}/sbom.cdx.json.tmp" "${output_dir}/sbom.cdx.json"
}

function validate_sbom_files() {
  local output_dir="$1"

  log_callout "validating generated SBOM files..."

  for file in "${output_dir}/sbom.spdx.json" "${output_dir}/sbom.cdx.json"; do
    if [[ ! -s "$file" ]]; then
      log_error "SBOM file missing or empty: $file"
      return 1
    fi
    if ! jq empty "$file" 2>/dev/null; then
      log_error "SBOM file contains invalid JSON: $file"
      return 1
    fi
  done

  if ! jq -e '.spdxVersion' "${output_dir}/sbom.spdx.json" >/dev/null 2>&1; then
    log_error "SPDX SBOM missing spdxVersion field"
    return 1
  fi

  if ! jq -e '.bomFormat' "${output_dir}/sbom.cdx.json" >/dev/null 2>&1; then
    log_error "CycloneDX SBOM missing bomFormat field"
    return 1
  fi

  if jq -e '[.packages[] | select(.versionInfo == "UNKNOWN")] | length > 0' "${output_dir}/sbom.spdx.json" >/dev/null 2>&1; then
    log_error "SPDX SBOM contains packages with UNKNOWN versions"
    return 1
  fi

  if jq -e '[.components[] | select(.version == "UNKNOWN")] | length > 0' "${output_dir}/sbom.cdx.json" >/dev/null 2>&1; then
    log_error "CycloneDX SBOM contains components with UNKNOWN versions"
    return 1
  fi

  log_success "all SBOM files validated successfully"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  generate_modern_sbom "${VERSION}" "${OUTPUT_DIR}"
fi
