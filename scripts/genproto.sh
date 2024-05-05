#!/usr/bin/env bash
#
# Generate all etcd protobuf bindings.
# Run from repository root directory named etcd.
#
set -euo pipefail

shopt -s globstar

if ! [[ "$0" =~ scripts/genproto.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# Set SED variable
if LANG=C sed --help 2>&1 | grep -q GNU; then
  SED="sed"
elif command -v gsed &>/dev/null; then
  SED="gsed"
else
  echo "Failed to find GNU sed as sed or gsed. If you are on Mac: brew install gnu-sed." >&2
  exit 1
fi

source ./scripts/test_lib.sh

if [[ $(protoc --version | cut -f2 -d' ') != "3.20.3" ]]; then
  echo "Could not find protoc 3.20.3, installing now..."

  arch=$(go env GOARCH)

  case ${arch} in
    "amd64") file="x86_64" ;;
    "arm64") file="aarch_64" ;;
    *)
      echo "Unsupported architecture: ${arch}"
      exit 255
      ;;
  esac

  download_url="https://github.com/protocolbuffers/protobuf/releases/download/v3.20.3/protoc-3.20.3-linux-${file}.zip"
  echo "Running on ${arch}."
  mkdir -p bin
  wget ${download_url} && unzip -p protoc-3.20.3-linux-${file}.zip bin/protoc > tmpFile && mv tmpFile bin/protoc 
  rm protoc-3.20.3-linux-${file}.zip
  chmod +x bin/protoc
  PATH=$PATH:$(pwd)/bin
  export PATH
  echo "Now running: $(protoc --version)"

fi

GOFAST_BIN=$(tool_get_bin github.com/gogo/protobuf/protoc-gen-gofast)
GRPC_GATEWAY_BIN=$(tool_get_bin github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway)
OPENAPIV2_BIN=$(tool_get_bin github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2)
GOGOPROTO_ROOT="$(tool_pkg_dir github.com/gogo/protobuf/proto)/.."
GRPC_GATEWAY_ROOT="$(tool_pkg_dir github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway)/.."
RAFT_ROOT="$(tool_pkg_dir go.etcd.io/raft/v3/raftpb)/.."
GOOGLEAPI_ROOT=$(mktemp -d -t 'googleapi.XXXXX')

readonly googleapi_commit=0adf469dcd7822bf5bc058a7b0217f5558a75643

function cleanup_googleapi() {
  rm -rf "${GOOGLEAPI_ROOT}"
}

trap cleanup_googleapi EXIT

# TODO(ahrtr): use buf (https://github.com/bufbuild/buf) to manage the protobuf dependencies?
function download_googleapi() {
  run pushd "${GOOGLEAPI_ROOT}"
  run git init
  run git remote add upstream https://github.com/googleapis/googleapis.git
  run git fetch upstream "${googleapi_commit}"
  run git reset --hard FETCH_HEAD
  run popd
}

download_googleapi

echo
echo "Resolved binary and packages versions:"
echo "  - protoc-gen-gofast:       ${GOFAST_BIN}"
echo "  - protoc-gen-grpc-gateway: ${GRPC_GATEWAY_BIN}"
echo "  - openapiv2:               ${OPENAPIV2_BIN}"
echo "  - gogoproto-root:          ${GOGOPROTO_ROOT}"
echo "  - grpc-gateway-root:       ${GRPC_GATEWAY_ROOT}"
echo "  - raft-root:               ${RAFT_ROOT}"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"

# directories containing protos to be built
DIRS="./server/storage/wal/walpb ./api/etcdserverpb ./server/etcdserver/api/snap/snappb ./api/mvccpb ./server/lease/leasepb ./api/authpb ./server/etcdserver/api/v3lock/v3lockpb ./server/etcdserver/api/v3election/v3electionpb ./api/membershippb ./api/versionpb"

log_callout -e "\\nRunning gofast (gogo) proto generation..."

for dir in ${DIRS}; do
  run pushd "${dir}"
    run protoc --gofast_out=plugins=grpc:. -I=".:${GOGOPROTO_PATH}:${ETCD_ROOT_DIR}/..:${RAFT_ROOT}:${ETCD_ROOT_DIR}:${GOOGLEAPI_ROOT}" \
      --gofast_opt=paths=source_relative,Mraftpb/raft.proto=go.etcd.io/raft/v3/raftpb,Mgoogle/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor \
      -I"${GRPC_GATEWAY_ROOT}" \
      --plugin="${GOFAST_BIN}" ./**/*.proto

    run gofmt -s -w ./**/*.pb.go
    run_go_tool "golang.org/x/tools/cmd/goimports" -w ./**/*.pb.go
  run popd
done

log_callout -e "\\nRunning swagger & grpc_gateway proto generation..."

# remove old swagger files so it's obvious whether the files fail to generate
rm -rf Documentation/dev-guide/apispec/swagger/*json
for pb in api/etcdserverpb/rpc server/etcdserver/api/v3lock/v3lockpb/v3lock server/etcdserver/api/v3election/v3electionpb/v3election; do
  log_callout "grpc & swagger for: ${pb}.proto"
  run protoc -I. \
      -I"${GOOGLEAPI_ROOT}" \
      -I"${GRPC_GATEWAY_ROOT}" \
      -I"${GOGOPROTO_PATH}" \
      -I"${ETCD_ROOT_DIR}/.." \
      -I"${RAFT_ROOT}" \
      --grpc-gateway_out=logtostderr=true,paths=source_relative:. \
      --grpc-gateway_opt=Mgoogle/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor,Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types \
      --openapiv2_out=json_names_for_fields=false,logtostderr=true:./Documentation/dev-guide/apispec/swagger/. \
      --openapiv2_opt=Mgoogle/protobuf/descriptor.proto=github.com/gogo/protobuf/protoc-gen-gogo/descriptor,Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types:. \
      --plugin="${OPENAPIV2_BIN}" \
      --plugin="${GRPC_GATEWAY_BIN}" \
      ${pb}.proto
  # hack to move gw files around so client won't include them
  pkgpath=$(dirname "${pb}")
  pkg=$(basename "${pkgpath}")
  gwfile="${pb}.pb.gw.go"

  run ${SED?} -i -E "s#package $pkg#package gw#g" "${gwfile}"
  run ${SED?} -i -E "s#import \\(#import \\(\"go.etcd.io/etcd/${pkgpath}\"#g" "${gwfile}"
  run ${SED?} -i -E "s#([ (])([a-zA-Z0-9_]*(Client|Server|Request)([^(]|$))#\\1${pkg}.\\2#g" "${gwfile}"
  run ${SED?} -i -E "s# (New[a-zA-Z0-9_]*Client\\()# ${pkg}.\\1#g" "${gwfile}"
  run ${SED?} -i -E "s|go.etcd.io/etcd|go.etcd.io/etcd/v3|g" "${gwfile}"
  run ${SED?} -i -E "s|go.etcd.io/etcd/v3/api|go.etcd.io/etcd/api/v3|g" "${gwfile}"
  run ${SED?} -i -E "s|go.etcd.io/etcd/v3/server|go.etcd.io/etcd/server/v3|g" "${gwfile}"

  run go fmt "${gwfile}"

  gwdir="${pkgpath}/gw/"
  run mkdir -p "${gwdir}"
  run mv "${gwfile}" "${gwdir}"

  swaggerName=$(basename ${pb})
  run mv  Documentation/dev-guide/apispec/swagger/${pb}.swagger.json \
    Documentation/dev-guide/apispec/swagger/"${swaggerName}".swagger.json
done

# We only upgraded grpc-gateway from v1 to v2, but keep gogo/protobuf as it's for now.
# So we have to convert v1 message to v2 message. Once we get rid of gogo/protobuf, and
# start to depend on protobuf v2, then we can remove this patch.
#
# TODO(https://github.com/etcd-io/etcd/issues/14533): Remove the patch below after removal of gogo/protobuf
for pb in api/etcdserverpb/rpc server/etcdserver/api/v3lock/v3lockpb/v3lock server/etcdserver/api/v3election/v3electionpb/v3election; do
  gwfile="$(dirname ${pb})/gw/$(basename ${pb}).pb.gw.go"

  # Changes something like below,
  #  import (
  # +       protov1 "github.com/golang/protobuf/proto"
  # +
  run ${SED?} -i -E "s|import \(|import \(\n\tprotov1 \"github.com/golang/protobuf/proto\"\n|g" "${gwfile}"

  # Changes something like below,
  # - return msg, metadata, err
  # + return protov1.MessageV2(msg), metadata, err
  run ${SED?} -i -E "s|return msg, metadata, err|return protov1.MessageV2\(msg\), metadata, err|g" "${gwfile}"

  # Changes something like below,
  # - if err := marshaler.NewDecoder(newReader()).Decode(&protoReq); err != nil && err != io.EOF {
  # + if err := marshaler.NewDecoder(newReader()).Decode(protov1.MessageV2(&protoReq)); err != nil && err != io.EOF {
  run ${SED?} -i -E "s|Decode\(\&protoReq\)|Decode\(protov1\.MessageV2\(\&protoReq\)\)|g" "${gwfile}"

  # Changes something like below,
  # - forward_Lease_LeaseKeepAlive_0(annotatedContext, mux, outboundMarshaler, w, req, func() (proto.Message, error) { return resp.Recv() }, mux.GetForwardResponseOptions()...)
  # + forward_Lease_LeaseKeepAlive_0(annotatedContext, mux, outboundMarshaler, w, req, func() (proto.Message, error) {
  # +   m1, err := resp.Recv()
  # +   return protov1.MessageV2(m1), err
  # + }, mux.GetForwardResponseOptions()...)
  run ${SED?} -i -E "s|return resp.Recv\(\)|\n\t\t\tm1, err := resp.Recv\(\)\n\t\t\treturn protov1.MessageV2\(m1\), err\n\t\t|g" "${gwfile}"

  run go fmt "${gwfile}"
done

if [ "${1:-}" != "--skip-protodoc" ]; then
  log_callout "protodoc is auto-generating grpc API reference documentation..."

  # API reference
  API_REFERENCE_FILE="Documentation/dev-guide/api_reference_v3.md"
  run rm -rf ${API_REFERENCE_FILE}
  run_go_tool go.etcd.io/protodoc --directories="api/etcdserverpb=service_message,api/mvccpb=service_message,server/lease/leasepb=service_message,api/authpb=service_message" \
    --output="${API_REFERENCE_FILE}" \
    --message-only-from-this-file="api/etcdserverpb/rpc.proto" \
    --disclaimer="---
title: API reference
---

This API reference is autogenerated from the named \`.proto\` files." || exit 2

  # remove the first 3 lines of the doc as an empty --title adds '### ' to the top of the file.
  run ${SED?} -i -e 1,3d ${API_REFERENCE_FILE}

  # API reference: concurrency
  API_REFERENCE_CONCURRENCY_FILE="Documentation/dev-guide/api_concurrency_reference_v3.md"
  run rm -rf ${API_REFERENCE_CONCURRENCY_FILE}
  run_go_tool go.etcd.io/protodoc --directories="server/etcdserver/api/v3lock/v3lockpb=service_message,server/etcdserver/api/v3election/v3electionpb=service_message,api/mvccpb=service_message" \
    --output="${API_REFERENCE_CONCURRENCY_FILE}" \
    --disclaimer="---
title: \"API reference: concurrency\"
---

This API reference is autogenerated from the named \`.proto\` files." || exit 2

  # remove the first 3 lines of the doc as an empty --title adds '### ' to the top of the file.
  run ${SED?} -i -e 1,3d ${API_REFERENCE_CONCURRENCY_FILE}

  log_success "protodoc is finished."
  log_warning -e "\\nThe API references have NOT been automatically published on the website."
  log_success -e "\\nTo publish the API references, copy the following files"
  log_success "  - ${API_REFERENCE_FILE}"
  log_success "  - ${API_REFERENCE_CONCURRENCY_FILE}"
  log_success "to the etcd-io/website repo under the /content/en/docs/next/dev-guide/ folder."
  log_success "(https://github.com/etcd-io/website/tree/main/content/en/docs/next/dev-guide)"
else
  log_warning "skipping grpc API reference document auto-generation..."
fi

log_success -e "\\n./genproto SUCCESS"
