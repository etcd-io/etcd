#!/usr/bin/env bash

# Script used to collect and upload test coverage (mostly by travis).
# Usage ./test_coverage_upload.sh [log_file]

set -o pipefail

: "${COVERDIR?"Need to set COVERDIR"}"
: "${TAG?"Need to set TAG"}"
cover_out_file="${COVERDIR}/${TAG}.coverprofile"
source ./scripts/test_lib.sh

# We collect the coverage files and merge them
merge_cov "${COVERDIR}" "${cover_out_file}"
# strip out generated files (using GNU-style sed)
sed --in-place -E "/[.]pb[.](gw[.])?go/d" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/api/v3/|api/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/client/v3/|client/v3/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/client/v2/|client/v2/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/client/pkg/v3|client/pkg/v3/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/etcdctl/v3/|etcdctl/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/etcdutl/v3/|etcdutl/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/pkg/v3/|pkg/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/raft/v3/|raft/|g" "${cover_out_file}" || true
sed --in-place -E "s|go.etcd.io/etcd/server/v3/|server/|g" "${cover_out_file}" || true

# We try to upload whatever we have:
bash <(curl -s https://codecov.io/bash) -f "${cover_out_file}" -F "${TAG}" || exit 2
