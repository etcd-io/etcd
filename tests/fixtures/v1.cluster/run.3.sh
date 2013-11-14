#!/bin/sh

./etcd -s 127.0.0.1:7003 -c 127.0.0.1:4003 -C 127.0.0.1:7001 -d tmp/node3 -n node3
