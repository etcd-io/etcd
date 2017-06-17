#!/bin/bash

TMP=`mktemp -d -t gen-secrets`
trap "rm -rf $TMP" EXIT

OUT="all.yml"

pushd $TMP >/dev/null

for type in "peer" "client"; do
  ca="etcd-ca --depot-path=${type}"
  $ca init --passphrase=nil > /dev/null
  $ca export --passphrase=nil | tar xf -
  CRT=`base64 ca.crt`
  cat <<EOF >>$OUT
---
apiVersion: v1
kind: Secret
metadata:
  name: etcd-${type}-ca
type: Opaque
data:
  ca.crt: $CRT
EOF

  if [ "x${type}" == "xclient" ]; then
    # generate one client cred for clients to use
    $ca new-cert --passphrase=nil client >/dev/null
    $ca sign --passphrase=nil client >/dev/null
    $ca export --passphrase=nil --insecure client | tar xf -
    CRT=`base64 client.crt`
    KEY=`base64 client.key.insecure`
    cat << EOF1 >>$OUT
---
apiVersion: v1
kind: Secret
metadata:
  name: etcd-client-key
type: Opaque
data:
  my.crt: ${CRT}
  my.key: ${KEY}
EOF1
  fi

  for i in {0..2}; do
    $ca new-cert --passphrase=nil --domain etcd${i} etcd${i} >/dev/null
    $ca sign --passphrase=nil etcd${i} >/dev/null
    $ca export --passphrase=nil --insecure etcd${i} | tar xf -
    CRT=`base64 etcd${i}.crt`
    KEY=`base64 etcd${i}.key.insecure`
    cat << EOF1 >>$OUT
---
apiVersion: v1
kind: Secret
metadata:
  name: etcd${i}-${type}-key
type: Opaque
data:
  my.crt: ${CRT}
  my.key: ${KEY}
EOF1
  done
done

cat $OUT
