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
if which openssl >/dev/null; then
  openssl x509 -in ca.crt -noout -text
fi

# generate DNS: localhost, IP: 127.0.0.1, CN: example.com certificates
cfssl gencert \
  --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr.json | cfssljson --bare ./server
mv server.pem server.crt
mv server-key.pem server.key.insecure
if which openssl >/dev/null; then
  openssl x509 -in ./server.crt -text -noout
fi

rm -f *.csr *.pem *.stderr *.txt
