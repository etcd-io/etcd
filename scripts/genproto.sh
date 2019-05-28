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
DIRS="./wal/walpb ./etcdserver/etcdserverpb ./etcdserver/api/snap/snappb ./raft/raftpb ./mvcc/mvccpb ./lease/leasepb ./auth/authpb ./etcdserver/api/v3lock/v3lockpb ./etcdserver/api/v3election/v3electionpb"

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

ETCD_IO_ROOT="${GOPATH}/src/go.etcd.io"
ETCD_ROOT="${ETCD_IO_ROOT}/etcd"
GOGOPROTO_ROOT="${GOPATH}/src/github.com/gogo/protobuf"
SCHWAG_ROOT="${GOPATH}/src/github.com/hexfusion/schwag"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"
GRPC_GATEWAY_ROOT="${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway"

rm -f "${ETCD_ROOT}"
mkdir -p "${ETCD_IO_ROOT}"
ln -s "${PWD}" "${ETCD_ROOT}"

# Ensure we have the right version of protoc-gen-gogo by building it every time.
# TODO(jonboulle): vendor this instead of `go get`ting it.
go get -u github.com/gogo/protobuf/{proto,protoc-gen-gogo,gogoproto}
go get -u golang.org/x/tools/cmd/goimports
pushd "${GOGOPROTO_ROOT}"
	git reset --hard "${GOGO_PROTO_SHA}"
	make install
popd

# generate gateway code
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger
pushd "${GRPC_GATEWAY_ROOT}"
	git reset --hard "${GRPC_GATEWAY_SHA}"
	go install ./protoc-gen-grpc-gateway
popd

for dir in ${DIRS}; do
	pushd "${dir}"
		protoc --gofast_out=plugins=grpc,import_prefix=go.etcd.io/:. -I=".:${GOGOPROTO_PATH}:${ETCD_IO_ROOT}:${GRPC_GATEWAY_ROOT}/third_party/googleapis" ./*.proto
		# shellcheck disable=SC1117
		sed -i.bak -E 's/go\.etcd\.io\/(gogoproto|github\.com|golang\.org|google\.golang\.org)/\1/g' ./*.pb.go
		# shellcheck disable=SC1117
		sed -i.bak -E 's/go\.etcd\.io\/(errors|fmt|io)/\1/g' ./*.pb.go
		# shellcheck disable=SC1117
		sed -i.bak -E 's/import _ \"gogoproto\"//g' ./*.pb.go
		# shellcheck disable=SC1117
		sed -i.bak -E 's/import fmt \"fmt\"//g' ./*.pb.go
		# shellcheck disable=SC1117
		sed -i.bak -E 's/import _ \"go\.etcd\.io\/google\/api\"//g' ./*.pb.go
		# shellcheck disable=SC1117
		sed -i.bak -E 's/import _ \"google\.golang\.org\/genproto\/googleapis\/api\/annotations\"//g' ./*.pb.go
		rm -f ./*.bak
		goimports -w ./*.pb.go
	popd
done

# remove old swagger files so it's obvious whether the files fail to generate
rm -rf Documentation/dev-guide/apispec/swagger/*json
for pb in etcdserverpb/rpc api/v3lock/v3lockpb/v3lock api/v3election/v3electionpb/v3election; do
	protobase="etcdserver/${pb}"
	protoc -I. \
	    -I"${GRPC_GATEWAY_ROOT}"/third_party/googleapis \
	    -I"${GOGOPROTO_PATH}" \
	    -I"${ETCD_IO_ROOT}" \
	    --grpc-gateway_out=logtostderr=true:. \
	    --swagger_out=logtostderr=true:./Documentation/dev-guide/apispec/swagger/. \
	    ${protobase}.proto
	# hack to move gw files around so client won't include them
	pkgpath=$(dirname "${protobase}")
	pkg=$(basename "${pkgpath}")
	gwfile="${protobase}.pb.gw.go"
	sed -i.bak -E "s/package $pkg/package gw/g" ${gwfile}
	# shellcheck disable=SC1117
	sed -i.bak -E "s/protoReq /&$pkg\./g" ${gwfile}
	sed -i.bak -E "s/, client /, client $pkg./g" ${gwfile}
	sed -i.bak -E "s/Client /, client $pkg./g" ${gwfile}
	sed -i.bak -E "s/[^(]*Client, runtime/${pkg}.&/" ${gwfile}
	sed -i.bak -E "s/New[A-Za-z]*Client/${pkg}.&/" ${gwfile}
	# darwin doesn't like newlines in sed...
	# shellcheck disable=SC1117
	sed -i.bak -E "s|import \(|& \"go.etcd.io/etcd/${pkgpath}\"|" ${gwfile}
	mkdir -p  "${pkgpath}"/gw/
	go fmt ${gwfile}
	mv ${gwfile} "${pkgpath}/gw/"
	rm -f ./etcdserver/${pb}*.bak
	swaggerName=$(basename ${pb})
	mv	Documentation/dev-guide/apispec/swagger/etcdserver/${pb}.swagger.json \
		Documentation/dev-guide/apispec/swagger/"${swaggerName}".swagger.json
done
rm -rf Documentation/dev-guide/apispec/swagger/etcdserver/

# append security to swagger spec
go get -u "github.com/hexfusion/schwag"
pushd "${SCHWAG_ROOT}"
	git reset --hard "${SCHWAG_SHA}"
	go install .
popd
schwag -input=Documentation/dev-guide/apispec/swagger/rpc.swagger.json

# install protodoc
# go get -v -u go.etcd.io/protodoc
#
# run './scripts/genproto.sh --skip-protodoc'
# to skip protodoc generation
#
if [ "$1" != "--skip-protodoc" ]; then
	echo "protodoc is auto-generating grpc API reference documentation..."
	go get -v -u go.etcd.io/protodoc
	SHA_PROTODOC="484ab544e116302a9a6021cc7c427d334132e94a"
	PROTODOC_PATH="${GOPATH}/src/go.etcd.io/protodoc"
	pushd "${PROTODOC_PATH}"
		git reset --hard "${SHA_PROTODOC}"
		go install
		echo "protodoc is updated"
	popd

	protodoc --directories="etcdserver/etcdserverpb=service_message,mvcc/mvccpb=service_message,lease/leasepb=service_message,auth/authpb=service_message" \
		--title="etcd API Reference" \
		--output="Documentation/dev-guide/api_reference_v3.md" \
		--message-only-from-this-file="etcdserver/etcdserverpb/rpc.proto" \
		--disclaimer="This is a generated documentation. Please read the proto files for more."

	protodoc --directories="etcdserver/api/v3lock/v3lockpb=service_message,etcdserver/api/v3election/v3electionpb=service_message,mvcc/mvccpb=service_message" \
		--title="etcd concurrency API Reference" \
		--output="Documentation/dev-guide/api_concurrency_reference_v3.md" \
		--disclaimer="This is a generated documentation. Please read the proto files for more."

	echo "protodoc is finished..."
else
	echo "skipping grpc API reference document auto-generation..."
fi
