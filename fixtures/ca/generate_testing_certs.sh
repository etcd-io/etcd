#!/bin/bash
#
# This script is used to generate all cert related files for etcd testing.

# location for temporary depot
depot=".depot"
# The passphrases for the keys are `asdf`.
passphrase="--passphrase asdf"

# etcd-ca could be found at github.com/coreos/etcd-ca
if [ $# -eq 0 ]; then
	# try to find it through $GOPATH
	IFS=':' read -a paths <<< "${GOPATH}"
	for path in ${paths[@]}; do
		if [ -f "${path}/bin/etcd-ca" ]; then
			ca="${path}/bin/etcd-ca --depot-path $depot"
			break
		fi
	done
	if [ "$ca" == "" ]; then echo "Failed finding etcd-ca binary"; exit 1; fi
else
	# treat the first argument as the path to etcd-ca binary
	ca="$1 --depot-path $depot"
fi

rm -rf $depot 2>/dev/null
# create ca, which is assumed to be the broken one
$ca init $passphrase
# export out and rename files
$ca export | tar xvf -
mv ca.crt broken_ca.crt
mv ca.key broken_ca.key

# create certificate
$ca new-cert $passphrase --ip 127.0.0.1 server
$ca sign $passphrase server
# export out and rename files
$ca export --insecure $passphrase server | tar xvf -
mv server.crt broken_server.crt
mv server.key.insecure broken_server.key.insecure

rm -rf $depot 2>/dev/null
# create ca
$ca init $passphrase
$ca export | tar xvf -

# create certificate for server
$ca new-cert $passphrase --ip 127.0.0.1 server
$ca sign $passphrase server
$ca export --insecure $passphrase server | tar xvf -
$ca chain server > server-chain.pem

# create certificate for server2
$ca new-cert $passphrase --ip 127.0.0.1 server2
$ca sign $passphrase server2
$ca export --insecure $passphrase server2 | tar xvf -

rm -rf $depot 2>/dev/null
