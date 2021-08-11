#!/bin/bash

if ! [[ "$0" =~ "./gencerts.sh" ]]; then
  echo "must be run from 'fixtures'"
  exit 255
fi

if ! which cfssl; then
  echo "cfssl is not installed"
  exit 255
fi

cfssl gencert --initca=true ./ca-csr.json | cfssljson --bare ./ca
mv ca.pem ca.crt
openssl x509 -in ca.crt -noout -text

# generate wildcard certificates DNS: m1/m2/m3.etcd.local
cfssl gencert \
    --ca ./ca.crt \
    --ca-key ./ca-key.pem \
    --config ./gencert.json \
    ./server-ca-csr-1.json | cfssljson --bare ./server-1
mv server-1.pem server-1.crt
mv server-1-key.pem server-1.key.insecure

cfssl gencert \
    --ca ./ca.crt \
    --ca-key ./ca-key.pem \
    --config ./gencert.json \
    ./server-ca-csr-2.json | cfssljson --bare ./server-2
mv server-2.pem server-2.crt
mv server-2-key.pem server-2.key.insecure

cfssl gencert \
    --ca ./ca.crt \
    --ca-key ./ca-key.pem \
    --config ./gencert.json \
    ./server-ca-csr-3.json | cfssljson --bare ./server-3
mv server-3.pem server-3.crt
mv server-3-key.pem server-3.key.insecure

rm -f *.csr *.pem *.stderr *.txt
