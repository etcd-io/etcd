#!/usr/bin/env bash

if ! [[ "$0" =~ "tests/semaphore.test.bash" ]]; then
  echo "must be run from repository root"
  exit 255
fi

<<COMMENT
# amd64-e2e
bash tests/semaphore.test.bash

# 386-e2e
TEST_ARCH=386 bash tests/semaphore.test.bash

# grpc-proxy
TEST_OPTS="PASSES='build grpcproxy'" bash tests/semaphore.test.bash

# coverage
TEST_OPTS="coverage" bash tests/semaphore.test.bash
COMMENT

if [ -z "${TEST_OPTS}" ]; then
	TEST_OPTS="PASSES='build release e2e' MANUAL_VER=v3.3.7"
fi
if [ "${TEST_ARCH}" == "386" ]; then
  TEST_OPTS="GOARCH=386 PASSES='build e2e'"
fi

echo "Running tests with" ${TEST_OPTS}
if [ "${TEST_OPTS}" == "PASSES='build grpcproxy'" ]; then
  echo "Skip proxy tests for this branch!"
  exit 0
elif [ "${TEST_OPTS}" == "coverage" ]; then
  echo "Skip coverage tests for this branch!"
  exit 0
else
  sudo HOST_TMP_DIR=/tmp TEST_OPTS="${TEST_OPTS}" make docker-test
fi
