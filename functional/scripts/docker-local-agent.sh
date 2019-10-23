#!/usr/bin/env bash

<<COMMENT
# run 3 agents for 3-node local etcd cluster
./scripts/docker-local-agent.sh 1
./scripts/docker-local-agent.sh 2
./scripts/docker-local-agent.sh 3
COMMENT

if ! [[ "${0}" =~ "scripts/docker-local-agent.sh" ]]; then
  echo "must be run from functional"
  exit 255
fi

if [[ -z "${GO_VERSION}" ]]; then
  GO_VERSION=1.12.12
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
  AGENT_ADDR_FLAG="--network tcp --address 127.0.0.1:${1}9027"
fi
echo "AGENT_NAME:" ${AGENT_NAME}
echo "AGENT_ADDR_FLAG:" ${AGENT_ADDR_FLAG}

docker run \
  --rm \
  --net=host \
  --name ${AGENT_NAME} \
  gcr.io/etcd-development/etcd-functional:go${GO_VERSION} \
  /bin/bash -c "./bin/etcd-agent ${AGENT_ADDR_FLAG}"
