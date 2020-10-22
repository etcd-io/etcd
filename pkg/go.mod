module go.etcd.io/etcd/pkg/v3

go 1.15

require (
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/creack/pty v1.1.11
	github.com/dustin/go-humanize v1.0.0
	github.com/google/pprof v0.0.0-20201016162654-8ef5528bdba2
	github.com/maruel/panicparse v1.5.1
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	golang.org/x/sys v0.0.0-20201009025420-dfb3f7c4e634
	google.golang.org/grpc v1.29.1
)

replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/api/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
