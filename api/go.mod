module go.etcd.io/etcd/api/v3

go 1.17

require (
	github.com/coreos/go-semver v0.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/stretchr/testify v1.7.2
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1
	google.golang.org/grpc v1.47.0
)

require (
	github.com/davecgh/go-spew v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 // indirect
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
	golang.org/x/text v0.3.5 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
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
