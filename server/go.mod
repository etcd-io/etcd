module go.etcd.io/etcd/server/v3

go 1.17

require (
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/dustin/go-humanize v1.0.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/golang/protobuf v1.5.2
	github.com/google/btree v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/jonboulle/clockwork v0.3.0
	github.com/prometheus/client_golang v1.12.2
	github.com/prometheus/client_model v0.2.0
	github.com/soheilhy/cmux v0.1.5
	github.com/spf13/cobra v1.4.0
	github.com/stretchr/testify v1.7.2
	github.com/tmc/grpc-websocket-proxy v0.0.0-20201229170055-e5319fda7802
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2
	go.etcd.io/bbolt v1.3.6
	go.etcd.io/etcd/api/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/pkg/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/v2 v2.306.0-alpha.0
	go.etcd.io/etcd/client/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/pkg/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/raft/v3 v3.6.0-alpha.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.32.0
	go.opentelemetry.io/otel v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.7.0
	go.opentelemetry.io/otel/sdk v1.7.0
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e
	golang.org/x/net v0.0.0-20220615171555-694bf12d69de
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1
	google.golang.org/grpc v1.47.0
	google.golang.org/protobuf v1.28.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	sigs.k8s.io/yaml v1.3.0
)

require (
	github.com/benbjohnson/clock v1.1.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.7.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.7.0 // indirect
	go.opentelemetry.io/otel/trace v1.7.0 // indirect
	go.opentelemetry.io/proto/otlp v0.16.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
)

replace (
	go.etcd.io/etcd/api/v3 => ../api
	go.etcd.io/etcd/client/pkg/v3 => ../client/pkg
	go.etcd.io/etcd/client/v2 => ../client/v2
	go.etcd.io/etcd/client/v3 => ../client/v3
	go.etcd.io/etcd/pkg/v3 => ../pkg
	go.etcd.io/etcd/raft/v3 => ../raft
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY

replace go.etcd.io/tests/v3 => ./FORBIDDEN_DEPENDENCY
