#!/bin/bash

set -e

if ! [[ "$0" =~ "./gencerts.sh" ]]; then
  echo "must be run from 'fixtures'"
  exit 255
fi

if ! which cfssl; then
  echo "cfssl is not installed"
  echo "use: go install -mod mod github.com/cloudflare/cfssl/cmd/cfssl github.com/cloudflare/cfssl/cmd/cfssljson"
  exit 255
fi

cfssl gencert --initca=true ./ca-csr.json | cfssljson --bare ./ca
mv ca.pem ca.crt

if which openssl >/dev/null; then
  openssl x509 -in ca.crt -noout -text
fi

# gencert [config_file.json] [cert-name]
function gencert {
  cfssl gencert \
    --ca ./ca.crt \
    --ca-key ./ca-key.pem \
    --config ./gencert.json \
    $1 | cfssljson --bare ./$2
  mv $2.pem $2.crt
  mv $2-key.pem $2.key.insecure
}

# generate DNS: localhost, IP: 127.0.0.1, CN: example.com certificates
gencert ./server-ca-csr.json server

#generates certificate that does not contain CN, to be used for proxy -> server connections.
gencert ./client-ca-csr-nocn.json client-nocn

# generate DNS: localhost, IP: 127.0.0.1, CN: example.com certificates (ECDSA)
gencert ./server-ca-csr-ecdsa.json server-ecdsa

# generate IP: 127.0.0.1, CN: example.com certificates
gencert ./server-ca-csr-ip.json server-ip

# generate IPv6: [::1], CN: example.com certificates
gencert ./server-ca-csr-ipv6.json server-ipv6

# generate DNS: localhost, IP: 127.0.0.1, CN: example2.com certificates
gencert ./server-ca-csr2.json server2

# generate DNS: localhost, IP: 127.0.0.1, CN: "" certificates
gencert ./server-ca-csr3.json server3

# generate wildcard certificates DNS: *.etcd.local
gencert ./server-ca-csr-wildcard.json server-wildcard

# generate revoked certificates and crl
cfssl gencert --ca ./ca.crt \
  --ca-key ./ca-key.pem \
  --config ./gencert.json \
  ./server-ca-csr.json 2>revoked.stderr | cfssljson --bare ./server-revoked
mv server-revoked.pem server-revoked.crt
mv server-revoked-key.pem server-revoked.key.insecure
grep serial revoked.stderr | awk ' { print $9 } ' >revoke.txt
cfssl gencrl revoke.txt ca.crt ca-key.pem | base64 --decode >revoke.crl

rm -f *.csr *.pem *.stderr *.txt
