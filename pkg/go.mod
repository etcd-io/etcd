module go.etcd.io/etcd/pkg/v3

go 1.15

require (
	github.com/coreos/go-systemd/v22 v22.0.0
	github.com/creack/pty v1.1.11
	github.com/dustin/go-humanize v0.0.0-20171111073723-bb3d318650d4
	github.com/spf13/pflag v1.0.1
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20201009025420-dfb3f7c4e634
	google.golang.org/grpc v1.29.1
)

replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/api/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
