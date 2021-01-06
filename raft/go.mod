module go.etcd.io/etcd/raft/v3

go 1.15

require (
	github.com/cockroachdb/datadriven v0.0.0-20200714090401-bf6692d28da5
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/pkg/errors v0.9.1 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.0-pre
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY

replace go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY

replace go.etcd.io/etcd/pkg/v3 => ../pkg
