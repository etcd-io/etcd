module go.etcd.io/etcd/pkg/v3

go 1.24.0

toolchain go1.24.13

require (
	github.com/creack/pty v1.1.11
	github.com/dustin/go-humanize v1.0.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.10.0
	go.etcd.io/etcd/client/pkg/v3 v3.5.26
	go.uber.org/zap v1.17.0
	google.golang.org/grpc v1.71.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/protobuf v1.36.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/api/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
