module go.etcd.io/etcd/api/v3

go 1.23

toolchain go1.23.4

require (
	github.com/coreos/go-semver v0.3.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.4
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0
	github.com/stretchr/testify v1.10.0
	google.golang.org/genproto/googleapis/api v0.0.0-20241118233622-e639e219e697
	google.golang.org/grpc v1.69.2
	google.golang.org/protobuf v1.36.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	go.opentelemetry.io/otel v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk v1.33.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241118233622-e639e219e697 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
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
