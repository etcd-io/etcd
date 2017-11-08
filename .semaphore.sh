#!/usr/bin/env bash

TEST_SUFFIX=$(date +%s | base64 | head -c 15)

TEST_OPTS="RELEASE_TEST=y INTEGRATION=y PASSES='build unit release integration_e2e functional' MANUAL_VER=v3.1.10"
if [ "$TEST_ARCH" == "386" ]; then
	TEST_OPTS="GOARCH=386 PASSES='build unit integration_e2e'"
fi

docker run \
	--rm \
	--volume=`pwd`:/go/src/github.com/coreos/etcd \
	gcr.io/etcd-development/etcd-test:go1.8.5 \
	/bin/bash -c "${TEST_OPTS} ./test 2>&1 | tee test-${TEST_SUFFIX}.log"

! egrep "(--- FAIL:|leak)" -A10 -B50 test-${TEST_SUFFIX}.log
