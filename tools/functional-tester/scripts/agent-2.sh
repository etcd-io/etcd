#!/usr/bin/env bash

<<COMMENT
# to run agent
./scripts/agent-2.sh

# to run with failpoints
ETCD_EXEC_PATH=/etcd-failpoints ./scripts/agent-2.sh
COMMENT

if ! [[ "$0" =~ "scripts/agent-2.sh" ]]; then
  echo "must be run from tools/functional-tester"
  exit 255
fi

if [ -z "${ETCD_EXEC_PATH}" ]; then
  ETCD_EXEC_PATH=/etcd
	echo "Running agent without failpoints:" ${ETCD_EXEC_PATH}
elif [[ "${ETCD_EXEC_PATH}" == "/etcd-failpoints" ]]; then
	echo "Running agent with failpoints:" ${ETCD_EXEC_PATH}
else
  echo "Cannot find executable:" ${ETCD_EXEC_PATH}
  exit 255
fi

rm -rf `pwd`/agent-2 && mkdir -p `pwd`/agent-2
docker run \
  --rm \
  --net=host \
  --name agent-2 \
  --mount type=bind,source=`pwd`/agent-2,destination=/agent-2 \
  gcr.io/etcd-development/etcd-functional-tester:go1.9.3 \
  /bin/bash -c "/etcd-agent \
    --etcd-path ${ETCD_EXEC_PATH} \
    --etcd-log-dir /agent-2 \
    --port :29027 \
    --failpoint-addr :7382"
