#!/usr/bin/env bash

set -o pipefail

: "${COVERDIR?"Need to set COVERDIR"}"
: "${TAG?"Need to set TAG"}"
cover_out_file="${COVERDIR}/${TAG}.coverprofile"

bash <(curl -s https://codecov.io/bash) -f "${cover_out_file}" -F "${TAG}" || exit 2
