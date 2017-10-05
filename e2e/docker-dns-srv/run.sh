#!/bin/sh

/etc/init.d/bind9 start
# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts
goreman -f /Procfile start &
sleep 5s
./etcdctl --discovery-srv etcd-srv.local set foo bar
ETCDCTL_API=3 ./etcdctl --discovery-srv etcd-srv.local put foo bar
