module go.etcd.io/etcd/client/v3

go 1.26

toolchain go1.26.7

require (
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/dustin/go-humanize v1.0.1
	github.com/golang/protobuf v1.5.4
	github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus v1.1.0
	github.com/prometheus/client_golang v1.24.1
	github.com/stretchr/testify v1.12.1
	go.etcd.io/etcd/api/v3 v3.8.0-alpha.0
	go.etcd.io/etcd/client/pkg/v3 v3.8.0-alpha.0
	go.uber.org/zap v1.28.0
	google.golang.org/grpc v1.83.1
	google.golang.org/protobuf v1.36.12
	sigs.k8s.io/yaml v1.6.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.30.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
)

replace (
	go.etcd.io/etcd/api/v3 => ../../api
	go.etcd.io/etcd/client/pkg/v3 => ../pkg
)
