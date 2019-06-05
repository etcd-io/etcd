#!/usr/bin/env bash

if ! [[ "$0" =~ "tests/semaphore.test.bash" ]]; then
  echo "must be run from repository root"
  exit 255
fi

<<COMMENT
# amd64-e2e
tests/semaphore.test.bash
sudo HOST_TMP_DIR=/tmp TEST_OPTS="PASSES='build release e2e' MANUAL_VER=v3.3.13" make docker-test

# 386-e2e
sudo HOST_TMP_DIR=/tmp TEST_OPTS="GOARCH=386 PASSES='build e2e'" make docker-test
COMMENT

sudo HOST_TMP_DIR=/tmp TEST_OPTS="PASSES='build release e2e' MANUAL_VER=v3.3.13" make docker-test
