#!/usr/bin/env bash

<<COMMENT
# to run with different Go version
# requires prebuilt Docker image
#   GO_VERSION=1.10 make build-docker-functional-tester -f ./hack/scripts-dev/Makefile
GO_VERSION=1.10 ./scripts/docker-local-tester.sh

# to run only 1 tester round
LIMIT=1 ./scripts/docker-local-tester.sh

# to run long-running tests with no limit
LIMIT=1 ./scripts/docker-local-tester.sh

# to run only 1 tester round with election runner and others
# default is STRESSER="keys,lease"
LIMIT=1 \
  STRESSER="keys,lease,election-runner,watch-runner,lock-racer-runner,lease-runner" \
  ./scripts/docker-local-tester.sh

# TODO: make stresser QPS configurable
COMMENT

if ! [[ "${0}" =~ "scripts/docker-local-tester.sh" ]]; then
  echo "must be run from tools/functional-tester"
  exit 255
fi

if [[ -z "${GO_VERSION}" ]]; then
  GO_VERSION=1.10
fi
echo "Running with GO_VERSION:" ${GO_VERSION}

if [[ "${LIMIT}" ]]; then
  LIMIT_FLAG="--limit ${LIMIT}"
  echo "Running with:" ${LIMIT_FLAG}
else
  echo "Running with no limit"
fi

if [[ "${STRESSER}" ]]; then
  STRESSER_FLAG="--stresser ${STRESSER}"
else
  STRESSER_FLAG="--stresser keys,lease"
fi
echo "Running with:" ${STRESSER_FLAG}

docker run \
  --rm \
  --net=host \
  --name tester \
  gcr.io/etcd-development/etcd-functional-tester:go${GO_VERSION} \
  /bin/bash -c "/etcd-tester \
    --agent-endpoints '127.0.0.1:19027,127.0.0.1:29027,127.0.0.1:39027' \
    --client-ports 1379,2379,3379 \
    --advertise-client-ports 13790,23790,33790 \
    --peer-ports 1380,2380,3380 \
    --advertise-peer-ports 13800,23800,33800 \
    ${LIMIT_FLAG} \
    --etcd-runner /etcd-runner \
    --stress-qps=2500 \
    --stress-key-txn-count 100 \
    --stress-key-txn-ops 10 \
    ${STRESSER_FLAG} \
    --exit-on-failure"
