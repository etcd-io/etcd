#!/bin/sh

/etc/init.d/bind9 start
# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts
goreman -f /Procfile.tls start &
sleep 5s
ETCDCTL_API=3 ./etcdctl --cacert=/certs/ca.crt --endpoints=https://m1.etcd.local:2379 put abc def
