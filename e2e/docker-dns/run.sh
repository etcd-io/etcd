#!/bin/sh

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /Procfile.tls start &
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server-wildcard.crt \
  --key=/certs//server-wildcard.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  put abc def
