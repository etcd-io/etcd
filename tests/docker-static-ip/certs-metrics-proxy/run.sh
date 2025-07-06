#!/bin/sh
rm -rf /tmp/m1.data /tmp/m2.data /tmp/m3.data /tmp/proxy.data

goreman -f /certs-metrics-proxy/Procfile start &

# TODO: remove random sleeps
sleep 7s

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-metrics-proxy/ca.crt \
  --cert=/certs-metrics-proxy/server.crt \
  --key=/certs-metrics-proxy/server.key.insecure \
  --endpoints=https://localhost:2379 \
  endpoint health --cluster

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-metrics-proxy/ca.crt \
  --cert=/certs-metrics-proxy/server.crt \
  --key=/certs-metrics-proxy/server.key.insecure \
  --endpoints=https://localhost:2379,https://localhost:22379,https://localhost:32379 \
  put abc def

ETCDCTL_API=3 ./etcdctl \
  --cacert=/certs-metrics-proxy/ca.crt \
  --cert=/certs-metrics-proxy/server.crt \
  --key=/certs-metrics-proxy/server.key.insecure \
  --endpoints=https://localhost:2379,https://localhost:22379,https://localhost:32379 \
  get abc

#################
sleep 3s && printf "\n\n" && echo "curl https://localhost:2378/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:2378/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "curl https://localhost:2379/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:2379/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "curl http://localhost:9379/metrics"
curl -L http://localhost:9379/metrics | grep Put | tail -3
#################

#################
sleep 3s && printf "\n\n" && echo "curl https://localhost:22378/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:22378/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "curl https://localhost:22379/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:22379/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "curl http://localhost:29379/metrics"
curl -L http://localhost:29379/metrics | grep Put | tail -3
#################

#################
sleep 3s && printf "\n\n" && echo "curl https://localhost:32378/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:32378/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "curl https://localhost:32379/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:32379/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "curl http://localhost:39379/metrics"
curl -L http://localhost:39379/metrics | grep Put | tail -3
#################

#################
sleep 3s && printf "\n\n" && echo "Requests to gRPC proxy localhost:23790"
ETCDCTL_API=3 ./etcdctl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  --endpoints=localhost:23790 \
  put ghi jkl

ETCDCTL_API=3 ./etcdctl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  --endpoints=localhost:23790 \
  get ghi

sleep 3s && printf "\n" && echo "Requests to gRPC proxy https://localhost:23790/metrics"
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:23790/metrics | grep Put | tail -3

sleep 3s && printf "\n" && echo "Requests to gRPC proxy http://localhost:9378/metrics"
curl -L http://localhost:9378/metrics | grep Put | tail -3
<<COMMENT
curl \
  --cacert /certs-metrics-proxy/ca.crt \
  --cert /certs-metrics-proxy/server.crt \
  --key /certs-metrics-proxy/server.key.insecure \
  -L https://localhost:9378/metrics | grep Put | tail -3
COMMENT
#################
