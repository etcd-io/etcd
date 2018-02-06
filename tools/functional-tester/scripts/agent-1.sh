#!/usr/bin/env bash

<<COMMENT
# to run agent
./scripts/agent-1.sh

# to run with failpoints
ETCD_EXEC_PATH=/etcd-failpoints ./scripts/agent-1.sh
COMMENT

if ! [[ "$0" =~ "scripts/agent-1.sh" ]]; then
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

rm -rf `pwd`/agent-1 && mkdir -p `pwd`/agent-1
docker run \
  --rm \
  --net=host \
  --name agent-1 \
  --mount type=bind,source=`pwd`/agent-1,destination=/agent-1 \
  gcr.io/etcd-development/etcd-functional-tester:go1.9.3 \
  /bin/bash -c "/etcd-agent \
    --etcd-path ${ETCD_EXEC_PATH} \
    --etcd-log-dir /agent-1 \
    --port :19027 \
    --failpoint-addr :7381"
