module go.etcd.io/etcd/c/v3

go 1.14

require (
	github.com/coreos/go-semver v0.2.0
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.0.0
	github.com/grpc-ecosystem/grpc-gateway v1.9.5
	github.com/json-iterator/go v1.1.10
	github.com/modern-go/reflect2 v1.0.1
	go.uber.org/zap v1.14.1
	golang.org/x/net v0.0.0-20190813141303-74dc4d7220e7
	google.golang.org/grpc v1.26.0
	sigs.k8s.io/yaml v1.1.0
)

replace go.etcd.io/etcd/v3 => ../NO_SUCH_DEPENDENCY
