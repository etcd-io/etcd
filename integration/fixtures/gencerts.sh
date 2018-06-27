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

# generate DNS: localhost, IP: 127.0.0.1, CN: example.com certificates (ECDSA)
cfssl gencert \
  --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr-ecdsa.json | cfssljson --bare ./server-ecdsa
mv server-ecdsa.pem server-ecdsa.crt
mv server-ecdsa-key.pem server-ecdsa.key.insecure

# generate IP: 127.0.0.1, CN: example.com certificates
cfssl gencert \
  --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr-ip.json | cfssljson --bare ./server-ip
mv server-ip.pem server-ip.crt
mv server-ip-key.pem server-ip.key.insecure

# generate DNS: localhost, IP: 127.0.0.1, CN: example2.com certificates
cfssl gencert \
  --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr2.json | cfssljson --bare ./server2
mv server2.pem server2.crt
mv server2-key.pem server2.key.insecure

# generate revoked certificates and crl
cfssl gencert --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr.json 2>revoked.stderr | cfssljson --bare ./server-revoked
mv server-revoked.pem server-revoked.crt
mv server-revoked-key.pem server-revoked.key.insecure
grep serial revoked.stderr | awk ' { print $9 } ' >revoke.txt
cfssl gencrl revoke.txt ca.crt ca-key.pem | base64 --decode >revoke.crl

# generate wildcard certificates DNS: *.etcd.local
cfssl gencert \
  --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr-wildcard.json | cfssljson --bare ./server-wildcard
mv server-wildcard.pem server-wildcard.crt
mv server-wildcard-key.pem server-wildcard.key.insecure


rm -f *.csr *.pem *.stderr *.txt
