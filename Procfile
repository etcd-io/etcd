# Use goreman to run `go get github.com/mattn/goreman`
etcd1: ./etcd -id 0x1 -bind-addr 127.0.0.1:4001 -peer-bind-addr :7001 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
etcd2: ./etcd -id 0x2 -bind-addr 127.0.0.1:4002 -peer-bind-addr :7002 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
etcd3: ./etcd -id 0x3 -bind-addr 127.0.0.1:4003 -peer-bind-addr :7003 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
proxy: ./etcd -proxy=on -bind-addr 127.0.0.1:8080 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
