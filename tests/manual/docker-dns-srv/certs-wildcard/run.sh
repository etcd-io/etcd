#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /certs-wildcard/Procfile start &

# TODO: remove random sleeps
sleep 7s

./etcdctl \
  --cacert=/certs-wildcard/ca.crt \
  --cert=/certs-wildcard/server.crt \
  --key=/certs-wildcard/server.key.insecure \
  --discovery-srv etcd.local \
  endpoint health --cluster

./etcdctl \
  --cacert=/certs-wildcard/ca.crt \
  --cert=/certs-wildcard/server.crt \
  --key=/certs-wildcard/server.key.insecure \
  --discovery-srv etcd.local \
  put abc def

./etcdctl \
  --cacert=/certs-wildcard/ca.crt \
  --cert=/certs-wildcard/server.crt \
  --key=/certs-wildcard/server.key.insecure \
  --discovery-srv etcd.local \
  get abc
