module go.etcd.io/etcd/etcdctl/v3

go 1.19

require (
	github.com/bgentry/speakeasy v0.1.0
	github.com/cheggaaa/pb/v3 v3.1.0
	github.com/dustin/go-humanize v1.0.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	go.etcd.io/etcd/api/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/pkg/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/pkg/v3 v3.6.0-alpha.0
	go.uber.org/zap v1.24.0
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858
	google.golang.org/grpc v1.53.0
)

require (
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.12 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	golang.org/x/text v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)

replace (
	go.etcd.io/etcd/api/v3 => ../api
	go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
	go.etcd.io/etcd/client/v3 => ../client/v3
	go.etcd.io/etcd/pkg/v3 => ../pkg
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/tests/v3 => ./FORBIDDEN_DEPENDENCY
)
