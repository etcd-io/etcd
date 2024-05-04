#!/usr/bin/env bash
# Measures test flakiness and create issues for flaky tests

set -euo pipefail

if [[ -z ${GITHUB_TOKEN:-} ]]
then
    echo "Please set the \$GITHUB_TOKEN environment variable for the script to work"
    exit 1
fi

pushd ./tools/testgrid-analysis
# ci-etcd-e2e-amd64 and ci-etcd-unit-test-amd64 runs 6 times a day. Keeping a rolling window of 14 days.
go run main.go flaky --auto-create-issues --dashboard=sig-etcd-periodics --tab=ci-etcd-e2e-amd64 --max-days=14
go run main.go flaky --auto-create-issues --dashboard=sig-etcd-periodics --tab=ci-etcd-unit-test-amd64 --max-days=14

go run main.go flaky --auto-create-issues --dashboard=sig-etcd-presubmits --tab=pull-etcd-e2e-amd64 --max-days=14
go run main.go flaky --auto-create-issues --dashboard=sig-etcd-presubmits --tab=pull-etcd-unit-test --max-days=14

go run main.go auto-close-stale-issues --days-before-auto-close=14

popd
