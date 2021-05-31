#!/usr/bin/env bash

if ! [[ "$0" =~ "tests/functional/build" ]]; then
  echo "must be run from repository root"
  exit 255
fi

outdir="${BINDIR:-../bin}"

(
  cd ./tests 
  CGO_ENABLED=0 go build -v -installsuffix cgo -ldflags "-s" -o "${outdir}/etcd-agent" ./functional/cmd/etcd-agent
  CGO_ENABLED=0 go build -v -installsuffix cgo -ldflags "-s" -o "${outdir}/etcd-proxy" ./functional/cmd/etcd-proxy
  CGO_ENABLED=0 go build -v -installsuffix cgo -ldflags "-s" -o "${outdir}/etcd-runner" ./functional/cmd/etcd-runner
  CGO_ENABLED=0 go test -v -installsuffix cgo -ldflags "-s" -c -o "${outdir}/etcd-tester" ./functional/cmd/etcd-tester
)
