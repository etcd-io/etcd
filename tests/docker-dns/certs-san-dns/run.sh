#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts
echo "127.0.0.1 m1.etcd.local" >> /etc/hosts
echo "127.0.0.1 m2.etcd.local" >> /etc/hosts
echo "127.0.0.1 m3.etcd.local" >> /etc/hosts

goreman -f /certs-san-dns/Procfile start &
# TODO: remove random sleeps
sleep 7s

ETCDCTL_API=3 ./etcdctl \
 --cacert=/certs-san-dns/ca.crt \
 --cert=/certs-san-dns/server-1.crt \
 --key=/certs-san-dns/server-1.key.insecure \
 --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
 endpoint health --cluster

printf "\nPut abc \n"
ETCDCTL_API=3 ./etcdctl \
 --cacert=/certs-san-dns/ca.crt \
 --cert=/certs-san-dns/server-2.crt \
 --key=/certs-san-dns/server-2.key.insecure \
 --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
 put abc def

printf "\nGet abc \n"
ETCDCTL_API=3 ./etcdctl \
 --cacert=/certs-san-dns/ca.crt \
 --cert=/certs-san-dns/server-3.crt \
 --key=/certs-san-dns/server-3.key.insecure \
 --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
 get abc

printf "\nKill etcd server 1...\n"
kill $(lsof -t -i:2379)
sleep 7s

printf "\nGet abc after killing server 1\n"
ETCDCTL_API=3 ./etcdctl \
 --cacert=/certs-san-dns/ca.crt \
 --cert=/certs-san-dns/server-2.crt \
 --key=/certs-san-dns/server-2.key.insecure \
 --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
 get abc
printf "\n\nDone!!!\n\n"

