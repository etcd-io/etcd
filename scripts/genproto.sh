#!/bin/sh -e
#
# Generate all etcd protobuf bindings.
# Run from repository root.
#

PREFIX="github.com/coreos/etcd/Godeps/_workspace/src"
DIRS="./wal/walpb ./etcdserver/etcdserverpb ./snap/snappb ./raft/raftpb"

SHA="20c42d4d4d776b60d32c2d35ecac40a60793f661"

if ! protoc --version > /dev/null; then
	echo "could not find protoc, is it installed + in PATH?"
	exit 255
fi

# Ensure we have the right version of protoc-gen-gogo by building it every time.
# TODO(jonboulle): vendor this instead of `go get`ting it.
export GOPATH=${PWD}/gopath
export GOBIN=${PWD}/bin
go get code.google.com/p/gogoprotobuf/{proto,protoc-gen-gogo,gogoproto}
pushd ${GOPATH}/src/code.google.com/p/gogoprotobuf
	git reset --hard ${SHA}
	make
popd

export PATH="${GOBIN}:${PATH}"

for dir in ${DIRS}; do
	pushd ${dir}
		protoc --gogo_out=. -I=.:${GOPATH}/src/code.google.com/p/gogoprotobuf/protobuf:${GOPATH}/src *.proto
		sed -i".bak" -e "s|code.google.com/p/gogoprotobuf/proto|${PREFIX}/code.google.com/p/gogoprotobuf/proto|" *.go
		rm -f *.bak
	popd
done
