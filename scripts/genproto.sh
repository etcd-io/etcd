#!/usr/bin/env bash
#
# Generate all etcd protobuf bindings.
# Run from repository root.
#
set -e

if ! [[ "$0" =~ scripts/genproto.sh ]]; then
	echo "must be run from repository root"
	exit 255
fi

if [[ $(protoc --version | cut -f2 -d' ') != "3.7.1" ]]; then
	echo "could not find protoc 3.7.1, is it installed + in PATH?"
	exit 255
fi

# directories containing protos to be built
DIRS="./wal/walpb ./etcdserver/etcdserverpb ./snap/snappb ./raft/raftpb ./mvcc/mvccpb ./lease/leasepb ./auth/authpb ./etcdserver/api/v3lock/v3lockpb ./etcdserver/api/v3election/v3electionpb"

# disable go mod
export GO111MODULE=off

# exact version of packages to build
GOGO_PROTO_SHA="1adfc126b41513cc696b209667c8656ea7aac67c"
GRPC_GATEWAY_SHA="92583770e3f01b09a0d3e9bdf64321d8bebd48f2"
SCHWAG_SHA="b7d0fc9aadaaae3d61aaadfc12e4a2f945514912"

# set up self-contained GOPATH for building
export GOPATH=${PWD}/gopath.proto
export GOBIN=${PWD}/bin
export PATH="${GOBIN}:${PATH}"

ETCD_IO_ROOT="${GOPATH}/src/github.com/coreos/etcd"
ETCD_ROOT="${ETCD_IO_ROOT}/etcd"
GOGOPROTO_ROOT="${GOPATH}/src/github.com/gogo/protobuf"
SCHWAG_ROOT="${GOPATH}/src/github.com/hexfusion/schwag"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"
GRPC_GATEWAY_ROOT="${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway"

function cleanup {
  # Remove the whole fake GOPATH which can really confuse go mod.
  rm -rf "${PWD}/gopath.proto"
}

cleanup
trap cleanup EXIT

mkdir -p "${ETCD_IO_ROOT}"
ln -s "${PWD}" "${ETCD_ROOT}"

echo "Installing gogo/protobuf..."
GOGOPROTO_ROOT="$GOPATH/src/github.com/gogo/protobuf"
# rm -rf $GOGOPROTO_ROOT
mkdir -p $GOPATH/src/github.com/gogo
pushd $GOPATH/src/github.com/gogo
  git clone https://github.com/gogo/protobuf.git
popd
pushd "${GOGOPROTO_ROOT}"
  git reset --hard HEAD
  make install
popd

echo "Installing grpc-ecosystem/grpc-gateway..."
GRPC_GATEWAY_ROOT="$GOPATH/src/github.com/grpc-ecosystem/grpc-gateway"
# rm -rf $GRPC_GATEWAY_ROOT
mkdir -p $GOPATH/src/github.com/grpc-ecosystem
go get -v -d github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
go get -v -d github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
pushd $GOPATH/src/github.com/grpc-ecosystem
  rm -rf ./grpc-gateway
  git clone https://github.com/grpc-ecosystem/grpc-gateway.git
popd
pushd "${GRPC_GATEWAY_ROOT}"
	git reset --hard "${GRPC_GATEWAY_SHA}"
	go install -v ./protoc-gen-grpc-gateway
	go install -v ./protoc-gen-swagger
popd

for dir in ${DIRS}; do
	pushd "${dir}"
		protoc --gofast_out=plugins=grpc,import_prefix=github.com/coreos/:. -I=".:${GOGOPROTO_PATH}:${ETCD_IO_ROOT}:${GRPC_GATEWAY_ROOT}/third_party/googleapis" ./*.proto
		sed -i.bak -E "s/github\.com\/coreos\/(gogoproto|github\.com|golang\.org|google\.golang\.org)/\1/g" ./*.pb.go
		sed -i.bak -E 's/github\.com\/coreos\/(errors|fmt|io|context|math\/bits)/\1/g' ./*.pb.go
		sed -i.bak -E 's/import _ \"gogoproto\"//g' ./*.pb.go
		sed -i.bak -E 's/import fmt \"fmt\"//g' ./*.pb.go
		sed -i.bak -E 's/import _ \"github\.com\/coreos\/google\/api\"//g' ./*.pb.go
		sed -i.bak -E 's/import _ \"google\.golang\.org\/genproto\/googleapis\/api\/annotations\"//g' ./*.pb.go
		rm -f ./*.bak
		goimports -w ./*.pb.go
	popd
done
