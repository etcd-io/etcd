# Use goreman to run `go get github.com/mattn/goreman`

etcd1: etcd -name=etcd1 -bind-addr=127.0.0.1:8899 -peer-bind-addr=127.0.0.1:7005 -peer-addr=127.0.0.1:7005 -peers='127.0.0.1:7001,127.0.0.1:7002,127.0.0.1:7003' -data-dir="."
etcd2: etcd -name=etcd2 -bind-addr=127.0.0.1:8900 -peer-bind-addr=127.0.0.1:7002 -peer-addr=127.0.0.1:7002 -peers='127.0.0.1:7001,127.0.0.1:7002,127.0.0.1:7003' -data-dir="."
etcd3: etcd -name=etcd3 -bind-addr=127.0.0.1:8901 -peer-bind-addr=127.0.0.1:7003 -peer-addr=127.0.0.1:7003 -peers='127.0.0.1:7001,127.0.0.1:7002,127.0.0.1:7003' -data-dir="."

proxy: etcd -name=proxy -bind-addr=127.0.0.1:8080 -peer-bind-addr=127.0.0.1:7004 -peer-addr=127.0.0.1:7004 -peers='127.0.0.1:7001,127.0.0.1:7002,127.0.0.1:7003,127.0.0.1:7004' -data-dir="."

