#!/bin/sh

IP_ADDR=$(ip addr show eth0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1)

exec /opt/etcd/bin/etcd -addr $IP_ADDR:4001 -peer-addr $IP_ADDR:7001 "$@"
