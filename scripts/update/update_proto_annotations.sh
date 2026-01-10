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
# Updates etcd_version_annotations.txt based on state of annotations in proto files.
# Developers can run this script to avoid manually updating etcd_version_annotations.txt.
# Before running this script please ensure that fields/messages that you added are annotated with next etcd version.

set -o errexit
set -o nounset
set -o pipefail

tmpfile=$(mktemp)
go run ./tools/proto-annotations/main.go --annotation etcd_version > "${tmpfile}"
mv "${tmpfile}" ./scripts/etcd_version_annotations.txt
