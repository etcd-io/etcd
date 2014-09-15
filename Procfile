# Use goreman to run `go get github.com/mattn/goreman`
etcd1: ./etcd -id 0x1 -l :8080 -peers '0x1=localhost:8080&0x2=localhost:8081&0x3=localhost:8082'
etcd2: ./etcd -id 0x2 -l :8081 -peers '0x1=localhost:8080&0x2=localhost:8081&0x3=localhost:8082'
etcd3: ./etcd -id 0x3 -l :8082 -peers '0x1=localhost:8080&0x2=localhost:8081&0x3=localhost:8082'
proxy: ./etcd -proxy-mode -l :4001 -peers '0x1=localhost:8080&0x2=localhost:8081&0x3=localhost:8082'
