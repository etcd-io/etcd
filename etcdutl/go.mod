module go.etcd.io/etcd/etcdutl/v3

go 1.16

replace (
	go.etcd.io/etcd/api/v3 => ../api
	go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
	go.etcd.io/etcd/client/v2 => ../client/v2
	go.etcd.io/etcd/client/v3 => ../client/v3
	go.etcd.io/etcd/pkg/v3 => ../pkg
	go.etcd.io/etcd/raft/v3 => ../raft
	go.etcd.io/etcd/server/v3 => ../server
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/tests/v3 => ./FORBIDDEN_DEPENDENCY
)

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/spf13/cobra v1.1.3
	go.etcd.io/bbolt v1.3.6-0.20210426205525-9c92be978ae0
	go.etcd.io/etcd/api/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/client/pkg/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/pkg/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/raft/v3 v3.5.0-alpha.0
	go.etcd.io/etcd/server/v3 v3.5.0-alpha.0
	go.uber.org/zap v1.16.1-0.20210329175301-c23abee72d19
)
