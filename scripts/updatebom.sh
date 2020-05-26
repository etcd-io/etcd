#!/usr/bin/env bash

set -e

if ! [[ "$0" =~ scripts/updatebom.sh ]]; then
	echo "must be run from repository root"
	exit 255
fi

export GO111MODULE=off # Don't add BOM tool to etcd go.mod
echo "installing 'bill-of-materials.json'"
go get -v -u github.com/coreos/license-bill-of-materials
export GO111MODULE=on

echo "generating bill-of-materials.json"
license-bill-of-materials \
    --override-file ./bill-of-materials.override.json \
    go.etcd.io/etcd/v3 go.etcd.io/etcd/v3/etcdctl > bill-of-materials.json

echo "generated bill-of-materials.json"
