#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

tmpfile=$(mktemp)
go run ./tools/proto-annotations/main.go --annotation=etcd_version > "${tmpfile}"
if diff -u ./scripts/etcd_version_annotations.txt "${tmpfile}"; then
  echo "PASSED proto-annotations verification!"
  exit 0
fi
echo "Failed proto-annotations-verification!" >&2
echo "If you are adding new proto fields/messages that will be included in raft log:" >&2
echo "* Please add etcd_version annotation in *.proto file with next etcd version" >&2
echo "* Run ./scripts/getproto.sh" >&2
echo "* Run ./scripts/update_proto_annotations.sh" >&2
exit 1
