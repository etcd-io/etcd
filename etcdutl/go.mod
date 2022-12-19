module go.etcd.io/etcd/etcdutl/v3

go 1.19

replace (
	go.etcd.io/etcd/api/v3 => ../api
	go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
	go.etcd.io/etcd/client/v2 => ../client/v2
	go.etcd.io/etcd/client/v3 => ../client/v3
	go.etcd.io/etcd/pkg/v3 => ../pkg
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
	github.com/coreos/go-semver v0.3.0
	github.com/dustin/go-humanize v1.0.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/spf13/cobra v1.6.1
	go.etcd.io/bbolt v1.3.6
	go.etcd.io/etcd/api/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/pkg/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/pkg/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/server/v3 v3.6.0-alpha.0
	go.etcd.io/raft/v3 v3.0.0-20221201111702-eaa6808e1f7a
	go.uber.org/zap v1.24.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.3 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/etcd/client/v2 v2.306.0-alpha.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.37.0 // indirect
	go.opentelemetry.io/otel v1.11.2 // indirect
	go.opentelemetry.io/otel/metric v0.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.11.2 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e // indirect
	golang.org/x/net v0.4.0 // indirect
	golang.org/x/oauth2 v0.0.0-20220608161450-d0670ef3b1eb // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858 // indirect
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1 // indirect
	google.golang.org/grpc v1.51.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
)
