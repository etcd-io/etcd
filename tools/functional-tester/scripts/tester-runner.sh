#!/usr/bin/env bash

if ! [[ "$0" =~ "scripts/tester-runner.sh" ]]; then
  echo "must be run from tools/functional-tester"
  exit 255
fi

# to run with etcd-runner
docker run \
  --rm \
  --net=host \
  --name tester \
  gcr.io/etcd-development/etcd-functional-tester:go1.9.3 \
  /bin/bash -c "/etcd-tester \
    --agent-endpoints '127.0.0.1:19027,127.0.0.1:29027,127.0.0.1:39027' \
    --client-ports 1379,2379,3379 \
    --advertise-client-ports 13790,23790,33790 \
    --peer-ports 1380,2380,3380 \
    --advertise-peer-ports 13800,23800,33800 \
    --stress-qps=2500 \
    --stress-key-txn-count 100 \
    --stress-key-txn-ops 10 \
    --etcd-runner /etcd-runner \
    --stresser=keys,lease,election-runner,watch-runner,lock-racer-runner,lease-runner \
    --exit-on-failure"
