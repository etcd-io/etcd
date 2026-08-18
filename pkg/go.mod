module go.etcd.io/etcd/pkg/v3

go 1.26

toolchain go1.26.6

require (
	github.com/creack/pty v1.1.24
	github.com/dustin/go-humanize v1.0.1
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	go.etcd.io/etcd/client/pkg/v3 v3.8.0-alpha.0
	go.opentelemetry.io/otel/trace v1.45.0
	go.uber.org/zap v1.28.0
	golang.org/x/sys v0.47.0
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/otel v1.45.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
