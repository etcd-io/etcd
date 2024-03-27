#!/usr/bin/env bash
# Measures test flakiness and create issues for flaky tests

set -euo pipefail

if [[ -z ${GITHUB_TOKEN:-} ]]
then
    echo "Please set the \$GITHUB_TOKEN environment variable for the script to work"
    exit 1
fi

pushd ./tools/testgrid-analysis
go run main.go flaky --create-issue --dashboard=sig-etcd-periodics --tab=ci-etcd-e2e-amd64
go run main.go flaky --create-issue --dashboard=sig-etcd-periodics --tab=ci-etcd-unit-test-amd64
popd
