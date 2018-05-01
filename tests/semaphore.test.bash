#!/usr/bin/env bash

if ! [[ "$0" =~ "tests/semaphore.test.bash" ]]; then
  echo "must be run from repository root"
  exit 255
fi

TEST_SUFFIX=$(date +%s | base64 | head -c 15)

TEST_OPTS="PASSES='build release e2e' MANUAL_VER=v3.3.3"
if [ "$TEST_ARCH" == "386" ]; then
  TEST_OPTS="GOARCH=386 PASSES='build e2e'"
fi

docker run \
  --rm \
  --volume=`pwd`:/go/src/github.com/coreos/etcd \
  gcr.io/etcd-development/etcd-test:go1.9.6 \
  /bin/bash -c "${TEST_OPTS} ./test 2>&1 | tee test-${TEST_SUFFIX}.log"

! egrep "(--- FAIL:|panic: test timed out|appears to have leaked)" -B50 -A10 test-${TEST_SUFFIX}.log
