#!/bin/sh

./etcd -s 127.0.0.1:7002 -c 127.0.0.1:4002 -C 127.0.0.1:7001 -d tmp/node2 -n node2
