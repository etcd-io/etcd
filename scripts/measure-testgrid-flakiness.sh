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

# Measures test flakiness and create issues for flaky tests

set -euo pipefail

if [[ -z ${GITHUB_TOKEN:-} ]]
then
    echo "Please set the \$GITHUB_TOKEN environment variable for the script to work"
    exit 1
fi

pushd ./tools/testgrid-analysis
# ci-etcd-e2e-amd64 and ci-etcd-unit-test-amd64 runs 6 times a day. Keeping a rolling window of 14 days.
go run main.go flaky --create-issue --dashboard=sig-etcd-periodics --tab=ci-etcd-e2e-amd64 --max-days=14
go run main.go flaky --create-issue --dashboard=sig-etcd-periodics --tab=ci-etcd-unit-test-amd64 --max-days=14

# do not create issues for presubmit tests
go run main.go flaky --dashboard=sig-etcd-presubmits --tab=pull-etcd-e2e-amd64
go run main.go flaky --dashboard=sig-etcd-presubmits --tab=pull-etcd-unit-test

popd
