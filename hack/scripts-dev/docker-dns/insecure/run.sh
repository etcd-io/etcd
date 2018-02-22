#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /insecure/Procfile start &

# TODO: remove random sleeps
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --endpoints=http://m1.etcd.local:2379 \
  endpoint health --cluster

ETCDCTL_API=3 ./etcdctl \
  --endpoints=http://m1.etcd.local:2379,http://m2.etcd.local:22379,http://m3.etcd.local:32379 \
  put abc def

ETCDCTL_API=3 ./etcdctl \
  --endpoints=http://m1.etcd.local:2379,http://m2.etcd.local:22379,http://m3.etcd.local:32379 \
  get abc

# TODO: add host header check to enforce same-origin-policy
curl -L http://127.0.0.1:2379/v2/keys/queue \
  -XPOST \
  -d value=Job1

curl -L http://m1.etcd.local:2379/v2/keys/queue \
  -XPOST \
  -d value=Job1
