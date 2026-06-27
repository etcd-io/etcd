#!/bin/bash -e

# Copyright 2024 gRPC authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Uncomment to enable debugging.
# set -x

WORKDIR="$(dirname $0)"
TEMPDIR=$(mktemp -d)

trap "rm -rf ${TEMPDIR}" EXIT

# Build protoc-gen-go-grpc binary and add to $PATH.
pushd "${WORKDIR}"
go build -o "${TEMPDIR}" .
PATH="${TEMPDIR}:${PATH}"
popd

protoc \
    --go-grpc_out="${TEMPDIR}" \
    --go-grpc_opt=paths=source_relative \
    "examples/route_guide/routeguide/route_guide.proto"

GOLDENFILE="examples/route_guide/routeguide/route_guide_grpc.pb.go"
GENFILE="${TEMPDIR}/examples/route_guide/routeguide/route_guide_grpc.pb.go"

# diff is piped to [[ $? == 1 ]] to avoid exiting on diff but exit on error
# (like if the file was not found). See man diff for more info.
DIFF=$(diff "${GOLDENFILE}" "${GENFILE}" || [[ $? == 1 ]])
if [[ -n "${DIFF}" ]]; then
    echo -e "ERROR: Generated file differs from golden file:\n${DIFF}"
    echo -e "If you have made recent changes to protoc-gen-go-grpc," \
     "please regenerate the golden files by running:" \
     "\n\t go generate google.golang.org/grpc/..." >&2
    exit 1
fi

echo SUCCESS
