#!/usr/bin/env bash

# A script for updating godep dependencies for the vendored directory /cmd/
# without pulling in etcd itself as a dependency.

rm -rf Godeps vendor
ln -s cmd/vendor vendor
godep save ./...
rm -rf cmd/Godeps
rm vendor
mv Godeps cmd/

# to use 'gogo/protobuf/jsonpb'
# cmd/vendor/github.com/grpc-ecosystem/grpc-gateway/runtime/marshal_jsonpb.go
find cmd/vendor/github.com/grpc-ecosystem/grpc-gateway/ -type f -exec sed -i 's/github.com\/golang\/protobuf\/jsonpb/github.com\/gogo\/protobuf\/jsonpb/g' {} \;

