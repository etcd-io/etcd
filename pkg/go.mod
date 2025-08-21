module go.etcd.io/etcd/pkg/v3

go 1.24

toolchain go1.24.6

require (
	github.com/creack/pty v1.1.18
	github.com/dustin/go-humanize v1.0.1
	github.com/spf13/cobra v1.9.1
	github.com/spf13/pflag v1.0.7
	github.com/stretchr/testify v1.10.0
	go.etcd.io/etcd/client/pkg/v3 v3.6.0-alpha.0
	go.opentelemetry.io/otel/trace v1.37.0
	go.uber.org/zap v1.27.0
	golang.org/x/sys v0.35.0
	google.golang.org/grpc v1.74.2
)

require (
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.etcd.io/etcd/client/pkg/v3 => ../client/pkg

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
// Etcd contains lots of packages and dependency relationship.
// Shouldn't import unnecessary dependencies
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/api/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/tests/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
)
