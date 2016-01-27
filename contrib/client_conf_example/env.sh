#! /bin/bash

export ETCDCLIENT_ENDPOINTS="http://localhost:12379 http://localhost:22379 http://localhost:32379"
export ETCDCLIENT_HEADER_TIMEOUT_PER_REQUEST="1.5s"
export ETCDCLIENT_SELECTION_MODE="PrioritizeLeader"

./example --verbose
./example --verbose --get

