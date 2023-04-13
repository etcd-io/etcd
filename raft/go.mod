module go.etcd.io/etcd/raft/v3

go 1.19

require (
	github.com/cockroachdb/datadriven v1.0.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	go.etcd.io/etcd/client/pkg/v3 v3.5.8
)

require (
	github.com/pmezard/go-difflib v1.0.0 // indirect
	google.golang.org/protobuf v1.26.0 // indirect
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY

replace go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY

replace go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
