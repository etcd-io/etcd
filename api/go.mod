module go.etcd.io/etcd/api/v3

go 1.15

replace github.com/golang/protobuf => github.com/golang/protobuf v1.3.3

require (
	github.com/coreos/go-semver v0.3.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.1
	github.com/grpc-ecosystem/grpc-gateway v1.14.6
	google.golang.org/genproto v0.0.0-20200806141610-86f49bd18e98
	google.golang.org/grpc v1.30.0
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/api/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/pkg/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
