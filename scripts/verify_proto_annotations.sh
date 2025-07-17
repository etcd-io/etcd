#!/usr/bin/env bash
# Copyright 2025 The etcd Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Verifies proto annotations to ensure all new proto fields and messages are annotated by comparing it with etcd_version_annotations.txt file.
# This scripts is automatically run by CI to prevent pull requests missing adding a proto annotation.

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
echo "* Run ./scripts/genproto.sh" >&2
echo "* Run ./scripts/update_proto_annotations.sh" >&2
exit 1
