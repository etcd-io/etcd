#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

/etc/init.d/bind9 start

# get rid of hosts so go lookup won't resolve 127.0.0.1 to localhost
cat /dev/null >/etc/hosts

goreman -f /certs/Procfile start &

# TODO: remove random sleeps
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server.crt \
  --key=/certs/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379 \
  endpoint health --cluster

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server.crt \
  --key=/certs/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  put abc def

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server.crt \
  --key=/certs/server.key.insecure \
  --endpoints=https://m1.etcd.local:2379,https://m2.etcd.local:22379,https://m3.etcd.local:32379 \
  get abc

printf "\nWriting v2 key...\n"
curl -L https://127.0.0.1:2379/v2/keys/queue \
  --cacert /certs/ca.crt \
  --cert /certs/server.crt \
  --key /certs/server.key.insecure \
  -X POST \
  -d value=data

printf "\nWriting v2 key...\n"
curl -L https://m1.etcd.local:2379/v2/keys/queue \
  --cacert /certs/ca.crt \
  --cert /certs/server.crt \
  --key /certs/server.key.insecure \
  -X POST \
  -d value=data

printf "\nWriting v3 key...\n"
curl -L https://127.0.0.1:2379/v3/kv/put \
  --cacert /certs/ca.crt \
  --cert /certs/server.crt \
  --key /certs/server.key.insecure \
  -X POST \
  -d '{"key": "Zm9v", "value": "YmFy"}'

printf "\n\nWriting v3 key...\n"
curl -L https://m1.etcd.local:2379/v3/kv/put \
  --cacert /certs/ca.crt \
  --cert /certs/server.crt \
  --key /certs/server.key.insecure \
  -X POST \
  -d '{"key": "Zm9v", "value": "YmFy"}'

printf "\n\nReading v3 key...\n"
curl -L https://m1.etcd.local:2379/v3/kv/range \
  --cacert /certs/ca.crt \
  --cert /certs/server.crt \
  --key /certs/server.key.insecure \
  -X POST \
  -d '{"key": "Zm9v"}'

printf "\n\nFetching 'curl https://m1.etcd.local:2379/metrics'...\n"
curl \
  --cacert /certs/ca.crt \
  --cert /certs/server.crt \
  --key /certs/server.key.insecure \
  -L https://m1.etcd.local:2379/metrics | grep Put | tail -3

printf "\n\nDone!!!\n\n"
