#!/usr/bin/env bash
# Updates etcd_version_annotations.txt based on state of annotations in proto files.
# Developers can run this script to avoid manually updating etcd_version_annotations.txt.
# Before running this script please ensure that fields/messages that you added are annotated with next etcd version.

set -o errexit
set -o nounset
set -o pipefail

tmpfile=$(mktemp)
go run ./tools/proto-annotations/main.go --annotation etcd_version > "${tmpfile}"
mv "${tmpfile}" ./scripts/etcd_version_annotations.txt
