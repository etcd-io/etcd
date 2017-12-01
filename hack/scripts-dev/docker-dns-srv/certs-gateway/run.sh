#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /certs-gateway/Procfile start &
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-gateway/ca.crt \
  --cert=/certs-gateway/server.crt \
  --key=/certs-gateway/server.key.insecure \
  --discovery-srv etcd.local \
  endpoint health --cluster

sleep 2s
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-gateway/ca.crt \
  --cert=/certs-gateway/server.crt \
  --key=/certs-gateway/server.key.insecure \
  --discovery-srv etcd.local \
  put abc def

sleep 2s
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-gateway/ca.crt \
  --cert=/certs-gateway/server.crt \
  --key=/certs-gateway/server.key.insecure \
  --discovery-srv etcd.local \
  get abc

sleep 2s
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-gateway/ca.crt \
  --cert=/certs-gateway/server.crt \
  --key=/certs-gateway/server.key.insecure \
  --endpoints=127.0.0.1:23790 \
  put ghi jkl

sleep 2s
ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-gateway/ca.crt \
  --cert=/certs-gateway/server.crt \
  --key=/certs-gateway/server.key.insecure \
  --endpoints=127.0.0.1:23790 \
  get ghi
