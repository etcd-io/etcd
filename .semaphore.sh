#!/usr/bin/env bash

TEST_SUFFIX=$(date +%s | base64 | head -c 15)

TEST_OPTS="PASSES='build unit release integration_e2e functional' MANUAL_VER=v3.3.1"
if [ "$TEST_ARCH" == "386" ]; then
	TEST_OPTS="GOARCH=386 PASSES='build unit integration_e2e'"
fi

docker run \
	--rm \
	--volume=`pwd`:/go/src/github.com/coreos/etcd \
	gcr.io/etcd-development/etcd-test:go1.9.4 \
	/bin/bash -c "${TEST_OPTS} ./test 2>&1 | tee test-${TEST_SUFFIX}.log"

! egrep "(--- FAIL:|panic: test timed out|appears to have leaked)" -B50 -A10 test-${TEST_SUFFIX}.log
