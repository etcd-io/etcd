module go.etcd.io/etcd/client/v2

go 1.15

require (
	github.com/json-iterator/go v1.1.10
	github.com/modern-go/reflect2 v1.0.1
	go.etcd.io/etcd/api/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/pkg/v3 v3.5.0-alpha.0
)

replace (
	go.etcd.io/etcd/api/v3 => ../../api
	go.etcd.io/etcd/pkg/v3 => ../../pkg
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/tests/v3 => ./FORBIDDEN_DEPENDENCY
)
