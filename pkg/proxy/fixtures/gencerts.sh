#!/bin/bash

set -euo pipefail

if ! [[ "$0" =~ "./gencerts.sh" ]]; then
  echo "must be run from 'fixtures'"
  exit 255
fi

if ! command -v cfssl; then
  echo "cfssl is not installed"
  echo 'use: bash -c "cd ../../../tools/mod; go install github.com/cloudflare/cfssl/cmd/cfssl"'
  exit 255
fi

if ! command -v cfssljson; then
  echo "cfssljson is not installed"
  echo 'use: bash -c "cd ../../../tools/mod; go install github.com/cloudflare/cfssl/cmd/cfssljson"'
  exit 255
fi

cfssl gencert --initca=true ./ca-csr.json | cfssljson --bare ./ca
mv ca.pem ca.crt
openssl x509 -in ca.crt -noout -text

# generate DNS: localhost, IP: 127.0.0.1, CN: example.com certificates
cfssl gencert \
    --ca ./ca.crt \
    --ca-key ./ca-key.pem \
    --config ./gencert.json \
    ./server-ca-csr.json | cfssljson --bare ./server
mv server.pem server.crt
mv server-key.pem server.key.insecure

rm -f *.csr *.pem *.stderr *.txt
