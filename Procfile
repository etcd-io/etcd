# Use goreman to run `go get github.com/mattn/goreman`
etcd1: bin/etcd -name node1 -bind-addr 127.0.0.1:4001 -peer-bind-addr :7001 -bootstrap-config 'node1=localhost:7001,node2=localhost:7002,node3=localhost:7003'
etcd2: bin/etcd -name node2 -bind-addr 127.0.0.1:4002 -peer-bind-addr :7002 -bootstrap-config 'node1=localhost:7001,node2=localhost:7002,node3=localhost:7003'
etcd3: bin/etcd -name node3 -bind-addr 127.0.0.1:4003 -peer-bind-addr :7003 -bootstrap-config 'node1=localhost:7001,node2=localhost:7002,node3=localhost:7003'
#proxy: bin/etcd -proxy=on -bind-addr 127.0.0.1:8080 -peers 'localhost:7001,localhost:7002,localhost:7003'
