# Use goreman to run `go get github.com/mattn/goreman`
etcd1: ./etcd -id 0x1 -l 127.0.0.1:4001 -r :7001 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
etcd2: ./etcd -id 0x2 -l 127.0.0.1:4002 -r :7002 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
etcd3: ./etcd -id 0x3 -l 127.0.0.1:4003 -r :7003 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
proxy: ./etcd -proxy-mode -l 127.0.0.1:8080 -peers '0x1=localhost:7001&0x2=localhost:7002&0x3=localhost:7003'
