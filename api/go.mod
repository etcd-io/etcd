module go.etcd.io/etcd/api/v3

go 1.24.0

toolchain go1.24.13

require (
	github.com/coreos/go-semver v0.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.4
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422
	google.golang.org/grpc v1.71.1
)

require (
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/protobuf v1.36.4 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
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
