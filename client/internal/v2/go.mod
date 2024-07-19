module go.etcd.io/etcd/client/v2

go 1.22

toolchain go1.22.6

require (
	go.etcd.io/etcd/api/v3 v3.6.0-alpha.0
	go.etcd.io/etcd/client/pkg/v3 v3.6.0-alpha.0
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6
)

require (
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	go.etcd.io/etcd/api/v3 => ./../../../api
	go.etcd.io/etcd/client/pkg/v3 => ./../../pkg
)
