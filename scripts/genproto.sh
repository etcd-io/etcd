#!/usr/bin/env bash
#
# Generate all etcd protobuf bindings.
# Run from repository root.
#
set -e

PREFIX="github.com/coreos/etcd/Godeps/_workspace/src"
DIRS="./wal/walpb ./etcdserver/etcdserverpb ./snap/snappb ./raft/raftpb ./storage/storagepb"

SHA="932b70afa8b0bf4a8e167fdf0c3367cebba45903"

if ! protoc --version > /dev/null; then
	echo "could not find protoc, is it installed + in PATH?"
	exit 255
fi

# Ensure we have the right version of protoc-gen-gogo by building it every time.
# TODO(jonboulle): vendor this instead of `go get`ting it.
export GOPATH=${PWD}/gopath
export GOBIN=${PWD}/bin
go get github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto}
pushd ${GOPATH}/src/github.com/gogo/protobuf/
	git reset --hard ${SHA}
	make install
popd

export PATH="${GOBIN}:${PATH}"

# copy all proto dependencies inside etcd to gopath
for dir in ${DIRS}; do
	mkdir -p ${GOPATH}/src/github.com/coreos/etcd/${dir}
	pushd ${dir}
		cp *.proto ${GOPATH}/src/github.com/coreos/etcd/${dir}
	popd
done

COREOS_ROOT="${GOPATH}/src/github.com/coreos"
GOGOPROTO_ROOT="${GOPATH}/src/github.com/gogo/protobuf"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"

ESCAPED_PREFIX=$(echo $PREFIX | sed -e 's/[\/&]/\\&/g')

for dir in ${DIRS}; do
	pushd ${dir}
		protoc --gogofast_out=plugins=grpc,import_prefix=github.com/coreos/:. -I=.:"${GOGOPROTO_PATH}":"${COREOS_ROOT}" *.proto
		sed -i.bak -E "s/github\.com\/coreos\/(gogoproto|github\.com|golang\.org|google\.golang\.org)/${ESCAPED_PREFIX}\/\1/g" *.pb.go
		sed -i.bak -E 's/github\.com\/coreos\/(errors|fmt|io)/\1/g' *.pb.go
		rm -f *.bak
	popd
done
