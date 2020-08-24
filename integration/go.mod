module go.etcd.io/etcd/integration/v3

go 1.14

require (
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/prometheus/client_golang v1.0.0
	github.com/soheilhy/cmux v0.1.4
	go.etcd.io/etcd/c/v3 v3.0.0-00010101000000-000000000000
	go.etcd.io/etcd/v3 v3.0.0-00010101000000-000000000000
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20191002192127-34f69633bfdc
	google.golang.org/grpc v1.26.0
)

replace go.etcd.io/etcd/c/v3 => ../c/

replace go.etcd.io/etcd/v3 => ../
