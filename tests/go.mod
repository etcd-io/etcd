module go.etcd.io/etcd/tests/v3

go 1.15

replace go.etcd.io/etcd/v3 => ../

require (
	github.com/grpc-ecosystem/grpc-gateway v1.9.5
	go.etcd.io/bbolt v1.3.5
	go.etcd.io/etcd/v3 v3.0.0-00010101000000-000000000000
)
