module go.etcd.io/etcd/v3

go 1.15

require (
	github.com/bgentry/speakeasy v0.1.0
	github.com/cockroachdb/datadriven v0.0.0-20190809214429-80d97fb3cbaa
	github.com/coreos/go-semver v0.2.0
	github.com/coreos/go-systemd/v22 v22.0.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dustin/go-humanize v0.0.0-20171111073723-bb3d318650d4
	github.com/fatih/color v1.7.0 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/groupcache v0.0.0-20160516000752-02826c3e7903
	github.com/golang/protobuf v1.3.5
	github.com/google/btree v1.0.0
	github.com/google/uuid v1.0.0
	github.com/gorilla/websocket v0.0.0-20170926233335-4201258b820c // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.1-0.20190118093823-f849b5445de4
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway v1.14.6
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jonboulle/clockwork v0.1.0
	github.com/json-iterator/go v1.1.7
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/mattn/go-runewidth v0.0.2 // indirect
	github.com/modern-go/reflect2 v1.0.1
	github.com/olekukonko/tablewriter v0.0.0-20170122224234-a0225b3f23b5
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/soheilhy/cmux v0.1.4
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.1
	github.com/tmc/grpc-websocket-proxy v0.0.0-20170815181823-89b8d40f7ca8
	github.com/urfave/cli v1.20.0
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2
	go.etcd.io/bbolt v1.3.5
	go.etcd.io/etcd/api/v3 v3.0.0-00010101000000-000000000000
	go.etcd.io/etcd/pkg/v3 v3.0.0-00010101000000-000000000000
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20191002192127-34f69633bfdc
	golang.org/x/net v0.0.0-20191002035440-2ec189313ef0
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	google.golang.org/genproto v0.0.0-20200513103714-09dca8ec2884
	google.golang.org/grpc v1.29.1
	gopkg.in/cheggaaa/pb.v1 v1.0.25
	sigs.k8s.io/yaml v1.1.0
)

replace (
	go.etcd.io/etcd/api/v3 => ./api
	go.etcd.io/etcd/pkg/v3 => ./pkg
)

// Bad imports are sometimes causing attempts to pull that code.
// This makes the error more explicit.
replace (
	go.etcd.io/etcd => ./FORBIDDEN_DEPENDENCY
	go.etcd.io/tests/v3 => ./FORBIDDEN_DEPENDENCY
)
