module go.etcd.io/etcd/client/v3

go 1.25.0

toolchain go1.25.9

require (
	github.com/dustin/go-humanize v1.0.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/prometheus/client_golang v1.11.1
	go.etcd.io/etcd/api/v3 v3.5.28
	go.etcd.io/etcd/client/pkg/v3 v3.5.28
	go.uber.org/zap v1.17.0
	google.golang.org/grpc v1.79.3
	sigs.k8s.io/yaml v1.2.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace (
	go.etcd.io/etcd/api/v3 => ../../api
	go.etcd.io/etcd/client/pkg/v3 => ../pkg
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/pkg/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/etcd/v3 => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/tests/v3 => ./FORBIDDEN_DEPENDENCY
)
