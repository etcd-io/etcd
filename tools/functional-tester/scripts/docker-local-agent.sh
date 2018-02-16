#!/usr/bin/env bash

<<COMMENT
# run 3 agents for 3-node local etcd cluster
./scripts/docker-local-agent.sh 1
./scripts/docker-local-agent.sh 2
./scripts/docker-local-agent.sh 3
COMMENT

if ! [[ "${0}" =~ "scripts/docker-local-agent.sh" ]]; then
  echo "must be run from tools/functional-tester"
  exit 255
fi

if [[ -z "${GO_VERSION}" ]]; then
  GO_VERSION=1.10
fi
echo "Running with GO_VERSION:" ${GO_VERSION}

if [[ -z ${1} ]]; then
  echo "Expected second argument: 1, 2, or 3"
  exit 255
else
  case ${1} in
    1) ;;
    2) ;;
    3) ;;
    *) echo "Expected second argument 1, 2, or 3, got" \"${1}\"
       exit 255 ;;
  esac
  AGENT_NAME="agent-${1}"
  AGENT_PORT_FLAG="--port :${1}9027"
  FAILPOINT_ADDR_FLAG="--failpoint-addr :738${1}"
fi
echo "AGENT_NAME:" ${AGENT_NAME}
echo "AGENT_PORT_FLAG:" ${AGENT_PORT_FLAG}
echo "FAILPOINT_ADDR_FLAG:" ${FAILPOINT_ADDR_FLAG}

if [[ -z "${ETCD_EXEC_PATH}" ]]; then
  ETCD_EXEC_PATH=/etcd
elif [[ "${ETCD_EXEC_PATH}" != "/etcd-failpoints" ]]; then
  echo "Cannot find etcd executable:" ${ETCD_EXEC_PATH}
  exit 255
fi
echo "ETCD_EXEC_PATH:" ${ETCD_EXEC_PATH}

rm -rf `pwd`/${AGENT_NAME} && mkdir -p `pwd`/${AGENT_NAME}
docker run \
  --rm \
  --net=host \
  --name ${AGENT_NAME} \
  --mount type=bind,source=`pwd`/${AGENT_NAME},destination=/${AGENT_NAME} \
  gcr.io/etcd-development/etcd-functional-tester:go${GO_VERSION} \
  /bin/bash -c "/etcd-agent \
    --etcd-path ${ETCD_EXEC_PATH} \
    --etcd-log-dir /${AGENT_NAME} \
    ${AGENT_PORT_FLAG} \
    ${FAILPOINT_ADDR_FLAG}"
