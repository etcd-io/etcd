#!/usr/bin/env bash
#
# Generate all etcd protobuf bindings.
# Run from repository root.
#
set -e

PREFIX="github.com/coreos/etcd/Godeps/_workspace/src"
DIRS="./wal/walpb ./etcdserver/etcdserverpb ./snap/snappb ./raft/raftpb ./storage/storagepb"

SHA="64f27bf06efee53589314a6e5a4af34cdd85adf6"

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
	make
popd

export PATH="${GOBIN}:${PATH}"

# copy all proto dependencies inside etcd to gopath
for dir in ${DIRS}; do
	mkdir -p ${GOPATH}/src/github.com/coreos/etcd/${dir}
	pushd ${dir}
		cp *.proto ${GOPATH}/src/github.com/coreos/etcd/${dir}
	popd
done

for dir in ${DIRS}; do
	pushd ${dir}
		protoc --gogofast_out=plugins=grpc:. -I=.:${GOPATH}/src/github.com/gogo/protobuf/protobuf:${GOPATH}/src *.proto
		sed -i".bak" -e "s|github.com/gogo/protobuf/proto|${PREFIX}/github.com/gogo/protobuf/proto|" *.go
		rm -f *.bak
	popd
done
