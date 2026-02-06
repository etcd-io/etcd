module go.etcd.io/etcd/client/v2

go 1.24.0

toolchain go1.24.13

require (
	github.com/json-iterator/go v1.1.11
	github.com/modern-go/reflect2 v1.0.1
	go.etcd.io/etcd/api/v3 v3.5.26
	go.etcd.io/etcd/client/pkg/v3 v3.5.26
)

require (
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
)

replace (
	go.etcd.io/etcd/api/v3 => ../../api
	go.etcd.io/etcd/client/pkg/v3 => ../pkg
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/pkg/v3 => ./FORBIDDED_DEPENDENCY
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
