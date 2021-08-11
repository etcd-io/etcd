#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /certs-common-name-multi/Procfile start &

# TODO: remove random sleeps
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name-multi/ca.crt \
  --cert=/certs-common-name-multi/server-1.crt \
  --key=/certs-common-name-multi/server-1.key.insecure \
  --endpoints=https://m1.etcd.local:2379 \
  endpoint health --cluster

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name-multi/ca.crt \
  --cert=/certs-common-name-multi/server-2.crt \
  --key=/certs-common-name-multi/server-2.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  put abc def

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-common-name-multi/ca.crt \
  --cert=/certs-common-name-multi/server-3.crt \
  --key=/certs-common-name-multi/server-3.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  get abc
