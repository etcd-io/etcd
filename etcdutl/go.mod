module go.etcd.io/etcd/etcdutl/v3

go 1.24.0

toolchain go1.24.13

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
	go.etcd.io/bbolt v1.3.12
	go.etcd.io/etcd/api/v3 v3.5.26
	go.etcd.io/etcd/client/pkg/v3 v3.5.26
	go.etcd.io/etcd/client/v3 v3.5.26
	go.etcd.io/etcd/pkg/v3 v3.5.26
	go.etcd.io/etcd/raft/v3 v3.5.26
	go.etcd.io/etcd/server/v3 v3.5.26
	go.uber.org/zap v1.17.0
)

require (
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jonboulle/clockwork v0.2.2 // indirect
	github.com/json-iterator/go v1.1.11 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/prometheus/client_golang v1.11.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/etcd/client/v2 v2.305.26 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.71.1 // indirect
	google.golang.org/protobuf v1.36.4 // indirect
)
