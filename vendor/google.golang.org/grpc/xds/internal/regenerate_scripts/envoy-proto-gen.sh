#!/bin/bash
set -ex
DATA_PLANE_API_VERSION=965c278c10fa90ff34cb4d4890141863f4437b4a

git clone git@github.com:envoyproxy/data-plane-api.git
git clone git@github.com:envoyproxy/protoc-gen-validate.git

cd data-plane-api
git checkout $DATA_PLANE_API_VERSION
cp ../utils/WORKSPACE .
bazel clean --expunge

# We download a local copy of the protoc-gen-validate repo to be used by bazel
# for customizing proto generated code import path.
# And we do a simple grep here to get the release version of the
# proto-gen-validate that gets used by data-plane-api.
PROTOC_GEN_VALIDATE=$(grep "PGV_GIT_SHA =" ./bazel/repository_locations.bzl  | sed -r 's/.*"(.*)".*/\1/')

cd ../protoc-gen-validate
git checkout $PROTOC_GEN_VALIDATE
git apply ../utils/protoc-gen-validate.patch

cd ../data-plane-api

# cleanup.sh remove all gogo proto related imports and labels.
../utils/cleanup.sh

git apply ../utils/data-plane-api.patch
# proto-gen.sh build all packages required for grpc xds implementation and move
# proto generated code to grpc/xds/internal/proto subdirectory.
../utils/proto-gen.sh

