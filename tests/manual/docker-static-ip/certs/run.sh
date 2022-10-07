#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data

goreman -f /certs/Procfile start &

# TODO: remove random sleeps
sleep 7s

./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server.crt \
  --key=/certs/server.key.insecure \
  --endpoints=https://localhost:2379 \
  endpoint health --cluster

./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server.crt \
  --key=/certs/server.key.insecure \
  --endpoints=https://localhost:2379,https://localhost:22379,https://localhost:32379 \
  put abc def

./etcdctl \
  --cacert=/certs/ca.crt \
  --cert=/certs/server.crt \
  --key=/certs/server.key.insecure \
  --endpoints=https://localhost:2379,https://localhost:22379,https://localhost:32379 \
  get abc
