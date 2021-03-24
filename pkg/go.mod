module go.etcd.io/etcd/pkg/v3

go 1.15

require (
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/creack/pty v1.1.11
	github.com/dustin/go-humanize v1.0.0
	github.com/golang/protobuf v1.5.1 // indirect
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.16.0
	golang.org/x/sys v0.0.0-20210305215415-5cdee2b1b5a0
	google.golang.org/grpc v1.36.0
)

replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/api/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
