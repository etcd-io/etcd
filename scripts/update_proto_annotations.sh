#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

tmpfile=$(mktemp)
go run ./tools/proto-annotations/main.go --annotation etcd_version > "${tmpfile}"
mv "${tmpfile}" ./scripts/etcd_version_annotations.txt
