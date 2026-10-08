#!/usr/bin/env bash

# Copyright 2026 The etcd Authors
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

VERSION=${1:-"main"}
OUTPUT_DIR=${2:-"."}

function generate_modern_sbom {
  local version="$1"
  local output_dir="$2"

  mkdir -p "${output_dir}"
  log_callout "generating modern SBOM files for version ${version}"

  log_callout "generating SPDX and CycloneDX SBOMs..."
  # syft records the runtime GOOS/GOARCH/CGO into module metadata, which feeds
  # the SPDXID hash; pin them so output is byte-stable across hosts. Disable
  # file catalogers (absolute paths) and exclude build outputs and dev tooling.
  #
  # syft lives in tools/sbom, outside the go workspace; GOWORK=off resolves its
  # pinned deps. The binary is built for the host arch; the pinned GOARCH below
  # only controls what syft records.
  local syft_bin
  ( cd "${ETCD_ROOT_DIR}/tools/sbom" && run env GOWORK=off go install github.com/anchore/syft/cmd/syft )
  syft_bin=$( cd "${ETCD_ROOT_DIR}/tools/sbom" && GOWORK=off go list -f '{{.Target}}' github.com/anchore/syft/cmd/syft )

  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    SYFT_FILE_METADATA_SELECTION=none \
    run "${syft_bin}" \
    scan dir:. \
    --select-catalogers '-file' \
    --exclude './bin/**' \
    --exclude './release/**' \
    --exclude './covdir/**' \
    --exclude './coverage/**' \
    --exclude './tools/**' \
    --source-name "etcd" \
    --source-version "${version}" \
    -o "spdx-json=${output_dir}/sbom.spdx.json" \
    -o "cyclonedx-json=${output_dir}/sbom.cdx.json"

  dedup_sbom "${output_dir}"
  augment_sbom "${version}" "${output_dir}"
  validate_sbom_files "${output_dir}"

  log_success "modern SBOM files generated successfully"
  log_callout "  - ${output_dir}/sbom.spdx.json"
  log_callout "  - ${output_dir}/sbom.cdx.json"
}

function dedup_sbom {
  local output_dir="$1"

  log_callout "deduplicating SBOM packages..."

  # syft catalogs each go module separately, so a dependency shared by N etcd
  # modules produces N package entries with distinct SPDXIDs. etcd pins one
  # version repo-wide, so the copies are redundant. Collapse to one package per
  # purl (the type-aware package identity, so a go module and a same-named
  # GitHub Action never merge) and repoint every relationship to the surviving
  # SPDXID. The canonical copy keeps a concluded license over NOASSERTION (etcd's
  # own modules only carry Apache-2.0 in their defining module's copy), then the
  # smallest SPDXID for a stable result. syft has no native dedup (anchore/syft#4324).
  jq '
    def purl: first(.externalRefs[]? | select(.referenceType == "purl") | .referenceLocator) // .SPDXID;
    def canonical:
      ([.[] | select((.licenseConcluded // "NOASSERTION") != "NOASSERTION")]) as $licensed
      | (if ($licensed | length) > 0 then $licensed else . end) | min_by(.SPDXID);
    ([.packages[] | select(.SPDXID | startswith("SPDXRef-Package-"))]
      | group_by(purl)) as $groups
    | ($groups | map(canonical)) as $canon
    | ([$groups | to_entries[] | .key as $i | .value[]
        | {key: .SPDXID, value: $canon[$i].SPDXID}] | from_entries) as $idmap
    | .packages = ([.packages[] | select(.SPDXID | startswith("SPDXRef-Package-") | not)]
                   + $canon | sort_by(.SPDXID))
    | .relationships = (.relationships
        | map(.spdxElementId |= ($idmap[.] // .) | .relatedSpdxElement |= ($idmap[.] // .))
        | map(select(.spdxElementId != .relatedSpdxElement))
        | unique | sort_by(.spdxElementId, .relatedSpdxElement, .relationshipType))
  ' "${output_dir}/sbom.spdx.json" > "${output_dir}/sbom.spdx.json.tmp"
  mv "${output_dir}/sbom.spdx.json.tmp" "${output_dir}/sbom.spdx.json"

  # CycloneDX equivalent: one component per purl, dependency graph repointed to
  # the surviving bom-ref, merging duplicate edges and dropping self-references
  # created by the collapse. Canonical pick mirrors SPDX (a licensed copy over an
  # unlicensed one, then smallest bom-ref). Components lacking a purl group by
  # their unique bom-ref, so they are never merged.
  jq '
    def canonical:
      ([.[] | select((.licenses // []) | length > 0)]) as $licensed
      | (if ($licensed | length) > 0 then $licensed else . end) | min_by(."bom-ref");
    (.components | group_by(.purl // ."bom-ref")) as $groups
    | ($groups | map(canonical)) as $canon
    | ([$groups | to_entries[] | .key as $i | .value[]
        | {key: ."bom-ref", value: $canon[$i]."bom-ref"}] | from_entries) as $idmap
    | .components = ($canon | sort_by(."bom-ref"))
    | .dependencies = (.dependencies
        | map(.ref |= ($idmap[.] // .)
            | if has("dependsOn") then .dependsOn |= (map($idmap[.] // .) | unique) else . end)
        | group_by(.ref)
        | map(.[0] + {dependsOn: ([.[].dependsOn[]?] | unique)})
        | map(.ref as $r | .dependsOn |= map(select(. != $r)))
        | sort_by(.ref))
  ' "${output_dir}/sbom.cdx.json" > "${output_dir}/sbom.cdx.json.tmp"
  mv "${output_dir}/sbom.cdx.json.tmp" "${output_dir}/sbom.cdx.json"
}

function augment_sbom {
  local version="$1"
  local output_dir="$2"

  log_callout "augmenting SBOMs with project metadata..."

  # syft installed via `go install` does not embed its module version; read it
  # from tools/sbom so the SBOM records the exact tool version.
  local syft_version
  syft_version=$( cd "${ETCD_ROOT_DIR}/tools/sbom" && GOWORK=off go list -m -f '{{.Version}}' github.com/anchore/syft)

  jq --arg ver "${version}" --arg syftver "${syft_version}" '
    .packages = [.packages[] |
      if (.name | startswith("go.etcd.io/etcd")) and .versionInfo == "UNKNOWN"
      then .versionInfo = $ver
      else . end
    ] |
    .name = "etcd-\($ver)" |
    .documentNamespace = "https://github.com/etcd-io/etcd/releases/tag/\($ver)/sbom.spdx.json" |
    .creationInfo.creators = (
      [.creationInfo.creators[] |
        if startswith("Tool: syft") then "Tool: syft-\($syftver)" else . end
      ] + ["Organization: etcd project (https://etcd.io/)"]
    )
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

function validate_sbom_files {
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

  # Dedup repoints references; a missed remap would leave a dangling ref. Any jq
  # failure (e.g. a missing relationships array) aborts under `set -e`, so a
  # structurally broken SBOM cannot pass validation silently.
  local dangling
  dangling=$(jq '
      ([.packages[].SPDXID] + [.files[]?.SPDXID] + ["SPDXRef-DOCUMENT"]) as $ids
      | any(.relationships[]; ([.spdxElementId, .relatedSpdxElement] - $ids | length) > 0)
    ' "${output_dir}/sbom.spdx.json")
  if [[ "${dangling}" != "false" ]]; then
    log_error "SPDX SBOM has relationships referencing unknown SPDXIDs"
    return 1
  fi

  dangling=$(jq '
      ([.components[]."bom-ref"] + [.metadata.component."bom-ref"]) as $ids
      | any(.dependencies[]; ([.ref] + (.dependsOn // []) - $ids | length) > 0)
    ' "${output_dir}/sbom.cdx.json")
  if [[ "${dangling}" != "false" ]]; then
    log_error "CycloneDX SBOM has dependencies referencing unknown bom-refs"
    return 1
  fi

  log_success "all SBOM files validated successfully"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  generate_modern_sbom "${VERSION}" "${OUTPUT_DIR}"
fi
